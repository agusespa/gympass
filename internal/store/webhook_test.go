package store

import (
	"context"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }
func finishedWorkout(id string, at int64) Row {
	r := workout(id, at, "Push")
	r["finished_at"] = at
	return r
}

func outboxStore(t *testing.T) (*Store, *fakeClock) {
	t.Helper()
	s := openTest(t)
	clock := &fakeClock{t: time.UnixMilli(1_000_000)}
	s.now = clock.now
	s.EnableWebhookOutbox(30 * time.Second)
	return s, clock
}

func mustSync(t *testing.T, s *Store, changes map[string][]Row) {
	t.Helper()
	_, _, rejected, err := s.Sync(context.Background(), 0, changes)
	if err != nil || len(rejected) > 0 {
		t.Fatalf("err %v, rejected %v", err, rejected)
	}
}

func due(t *testing.T, s *Store) []PendingWebhook {
	t.Helper()
	p, err := s.DueWebhooks(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFinishedAtRoundTrip(t *testing.T) {
	s := openTest(t)
	_, out, _, err := s.Sync(context.Background(), 0, map[string][]Row{
		"workouts": {workout("running", 1, "A"), finishedWorkout("done", 2)},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	for _, r := range out["workouts"] {
		got[r["id"].(string)] = r["finished_at"]
	}
	if got["running"] != nil || got["done"] != int64(2) {
		t.Fatalf("finished_at = %v", got)
	}
	bad := workout("bad", 3, "B")
	bad["finished_at"] = "yesterday"
	if _, _, rejected, err := s.Sync(context.Background(), 0, map[string][]Row{"workouts": {bad}}); err != nil || len(rejected) != 1 {
		t.Fatalf("non-integer finished_at: err %v, rejected %v", err, rejected)
	}
}

func TestOutboxSchedulesOnlyFinishedWorkouts(t *testing.T) {
	s, clock := outboxStore(t)
	ctx := context.Background()

	mustSync(t, s, map[string][]Row{"workouts": {workout("w1", 100, "Running")}})
	clock.advance(time.Minute)
	if p := due(t, s); len(p) != 0 {
		t.Fatalf("running workout scheduled: %v", p)
	}

	mustSync(t, s, map[string][]Row{"workouts": {finishedWorkout("w1", 200)}})
	clock.advance(29 * time.Second)
	if p := due(t, s); len(p) != 0 {
		t.Fatalf("due before delay: %v", p)
	}
	clock.advance(time.Second)
	p := due(t, s)
	if len(p) != 1 || p[0].WorkoutID != "w1" || p[0].Sent {
		t.Fatalf("due = %v", p)
	}

	// An edit to a set after DueWebhooks returned it keeps the newer send pending.
	mustSync(t, s, map[string][]Row{
		"workout_exercises": {{"id": "e1", "updated_at": int64(300), "deleted": false, "workout_id": "w1", "name": "Bench", "position": 1.0, "superset_id": nil}},
	})
	if err := s.MarkWebhookSent(ctx, p[0]); err != nil {
		t.Fatal(err)
	}
	clock.advance(30 * time.Second)
	p = due(t, s)
	if len(p) != 1 || !p[0].Sent {
		t.Fatalf("edit after send not pending: %v", p)
	}
	if err := s.MarkWebhookSent(ctx, p[0]); err != nil {
		t.Fatal(err)
	}

	// A set change maps back to its workout.
	mustSync(t, s, map[string][]Row{
		"sets": {func() Row { r := setRow("s1", "e1", 1, 5, 100); r["updated_at"] = int64(400); return r }()},
	})
	clock.advance(30 * time.Second)
	if p := due(t, s); len(p) != 1 || p[0].WorkoutID != "w1" {
		t.Fatalf("set change not scheduled: %v", p)
	}
}

func TestOutboxRetryAndSkip(t *testing.T) {
	s, clock := outboxStore(t)
	ctx := context.Background()
	mustSync(t, s, map[string][]Row{"workouts": {finishedWorkout("w1", 100)}})
	clock.advance(30 * time.Second)
	p := due(t, s)[0]

	if err := s.RetryWebhook(ctx, p, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := due(t, s); len(got) != 0 {
		t.Fatalf("retried send due early: %v", got)
	}
	clock.advance(time.Minute)
	p = due(t, s)[0]
	if p.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", p.Attempts)
	}
	if err := s.SkipWebhook(ctx, p); err != nil {
		t.Fatal(err)
	}
	if got := due(t, s); len(got) != 0 {
		t.Fatalf("skipped send still due: %v", got)
	}
}

func TestOutboxDisabledByDefault(t *testing.T) {
	s := openTest(t)
	mustSync(t, s, map[string][]Row{"workouts": {finishedWorkout("w1", 100)}})
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM webhook_outbox").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("outbox rows = %d, want 0", n)
	}
}

func TestWorkoutSnapshot(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	ex := func(id string, pos float64, name string, deleted bool) Row {
		return Row{"id": id, "updated_at": int64(1), "deleted": deleted, "workout_id": "w1", "name": name, "position": pos, "superset_id": nil}
	}
	set := func(id, ex string, pos float64, reps int64, kg float64, warmup, deleted bool) Row {
		return Row{"id": id, "updated_at": int64(1), "deleted": deleted, "workout_exercise_id": ex, "position": pos, "reps": reps, "weight_kg": kg, "warmup": warmup, "rest_seconds": int64(120), "done": true}
	}
	unchecked := set("a4", "e1", 4, 5, 90, false, false)
	unchecked["done"] = false
	mustSync(t, s, map[string][]Row{
		"workouts":          {finishedWorkout("w1", 5000)},
		"workout_exercises": {ex("e2", 2, "Row", false), ex("e1", 1, "Bench", false), ex("e3", 3, "Gone", true), ex("e4", 4, "Empty", false)},
		"sets": {
			set("a2", "e1", 2, 8, 80, false, false),
			set("a1", "e1", 1, 10, 40, true, false),
			set("a3", "e1", 3, 1, 1, false, true),
			set("b1", "e2", 1, 12, 50, false, false),
			unchecked,
		},
	})

	w, ok, err := s.Workout(ctx, "w1")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if w.FinishedAt == nil || *w.FinishedAt != 5000 || w.Deleted {
		t.Fatalf("workout = %+v", w)
	}
	names := []string{}
	for _, e := range w.Exercises {
		names = append(names, e.Name)
	}
	if len(names) != 3 || names[0] != "Bench" || names[1] != "Row" || names[2] != "Empty" {
		t.Fatalf("exercises = %v", names)
	}
	bench := w.Exercises[0].Sets
	if len(bench) != 2 || !bench[0].Warmup || bench[1].Reps != 8 || bench[1].RestSeconds != 120 {
		t.Fatalf("bench sets = %+v", bench)
	}
	if len(w.Exercises[2].Sets) != 0 {
		t.Fatalf("empty exercise has sets: %+v", w.Exercises[2])
	}

	if _, ok, err := s.Workout(ctx, "nope"); ok || err != nil {
		t.Fatalf("unknown workout: ok=%v err=%v", ok, err)
	}
}
