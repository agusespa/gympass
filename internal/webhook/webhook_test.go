package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agusespa/gym-tracker/internal/store"
)

type receiver struct {
	mu       sync.Mutex
	status   int
	payloads []Payload
	auth     []string
}

func (r *receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var p Payload
	if err := json.NewDecoder(req.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.payloads = append(r.payloads, p)
	r.auth = append(r.auth, req.Header.Get("Authorization"))
	if r.status != 0 {
		w.WriteHeader(r.status)
	}
}

func setup(t *testing.T) (*store.Store, *Sender, *receiver) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.EnableWebhookOutbox(0) // due immediately
	rec := &receiver{}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	return s, &Sender{Store: s, URL: srv.URL, Secret: "shh"}, rec
}

func syncRows(t *testing.T, s *store.Store, changes map[string][]store.Row) {
	t.Helper()
	if _, _, _, err := s.Sync(context.Background(), 0, changes); err != nil {
		t.Fatal(err)
	}
}

func workout(updatedAt int64, finished any, deleted bool) store.Row {
	return store.Row{"id": "w1", "updated_at": updatedAt, "deleted": deleted, "name": "Push", "date": "2026-11-14", "started_at": int64(1), "finished_at": finished}
}

func flush(t *testing.T, sender *Sender) {
	t.Helper()
	if err := sender.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSendsFinishedWorkout(t *testing.T) {
	s, sender, rec := setup(t)
	finishedAt := time.Date(2026, 11, 14, 18, 30, 0, 0, time.UTC).UnixMilli()
	syncRows(t, s, map[string][]store.Row{
		"workouts": {func() store.Row {
			r := workout(1, finishedAt, false)
			r["started_at"] = finishedAt - (72*time.Minute + 30*time.Second).Milliseconds()
			return r
		}()},
		"workout_exercises": {{"id": "e1", "updated_at": int64(1), "deleted": false, "workout_id": "w1", "exercise_id": "bench-press", "position": 1.0, "superset_id": nil},
			// Planned but never checked off: left out of the payload and the totals.
			{"id": "e2", "updated_at": int64(1), "deleted": false, "workout_id": "w1", "exercise_id": "dip", "position": 2.0, "superset_id": nil}},
		"sets": {
			{"id": "s4", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e1", "position": 4.0, "reps": int64(6), "warmup": false, "weight_kg": 85.0, "done": false, "rest_seconds": int64(120), "duration_seconds": int64(0), "rep_min": int64(0), "rep_max": int64(0)},
			{"id": "s5", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e2", "position": 1.0, "reps": int64(10), "warmup": false, "weight_kg": 0.0, "done": false, "rest_seconds": int64(120), "duration_seconds": int64(0), "rep_min": int64(0), "rep_max": int64(0)},
			{"id": "s1", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e1", "position": 1.0, "reps": int64(10), "weight_kg": 40.0, "warmup": true, "done": true, "rest_seconds": int64(120), "duration_seconds": int64(0), "rep_min": int64(0), "rep_max": int64(0)},
			{"id": "s2", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e1", "position": 2.0, "reps": int64(8), "warmup": false, "weight_kg": 82.5, "done": true, "rest_seconds": int64(120), "duration_seconds": int64(0), "rep_min": int64(0), "rep_max": int64(0)},
			{"id": "s3", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e1", "position": 3.0, "reps": int64(6), "warmup": false, "weight_kg": 85.0, "done": true, "rest_seconds": int64(120), "duration_seconds": int64(0), "rep_min": int64(0), "rep_max": int64(0)},
		},
	})
	flush(t, sender)

	if len(rec.payloads) != 1 {
		t.Fatalf("got %d requests, want 1", len(rec.payloads))
	}
	p := rec.payloads[0]
	if p.Event != EventSaved || rec.auth[0] != "Bearer shh" {
		t.Fatalf("event=%q auth=%q", p.Event, rec.auth[0])
	}
	w := p.Workout
	if w.ID != "w1" || w.Name != "Push" || w.Date != "2026-11-14" || w.FinishedAt != "2026-11-14T18:30:00Z" ||
		w.StartedAt != "2026-11-14T17:17:30Z" || w.DurationSeconds != 72*60+30 {
		t.Fatalf("workout = %+v", w)
	}
	if len(w.Exercises) != 1 || w.Exercises[0].Name != "Bench Press" || len(w.Exercises[0].Sets) != 3 || !w.Exercises[0].Sets[0].Warmup {
		t.Fatalf("exercises = %+v", w.Exercises)
	}
	want := Totals{Exercises: 1, WorkingSets: 2, Reps: 14}
	if *w.Totals != want {
		t.Fatalf("totals = %+v, want %+v", *w.Totals, want)
	}

	// Nothing more to send until something changes.
	flush(t, sender)
	if len(rec.payloads) != 1 {
		t.Fatalf("resent without changes: %d", len(rec.payloads))
	}

	// Deleting a delivered workout sends a deletion.
	syncRows(t, s, map[string][]store.Row{"workouts": {workout(2, finishedAt, true)}})
	flush(t, sender)
	if len(rec.payloads) != 2 || rec.payloads[1].Event != EventDeleted || rec.payloads[1].Workout.ID != "w1" {
		t.Fatalf("payloads = %+v", rec.payloads)
	}
}

func TestNothingSentForRunningOrUndeliveredDeletes(t *testing.T) {
	s, sender, rec := setup(t)
	syncRows(t, s, map[string][]store.Row{"workouts": {workout(1, nil, false)}})
	flush(t, sender)
	// Finished, then deleted before the first delivery: nothing to tell the receiver.
	syncRows(t, s, map[string][]store.Row{"workouts": {workout(2, int64(2), false)}})
	syncRows(t, s, map[string][]store.Row{"workouts": {workout(3, int64(2), true)}})
	flush(t, sender)
	if len(rec.payloads) != 0 {
		t.Fatalf("unexpected requests: %+v", rec.payloads)
	}
	if due, _ := s.DueWebhooks(context.Background(), 10); len(due) != 0 {
		t.Fatalf("still pending: %v", due)
	}
}

func TestFailedSendIsRetried(t *testing.T) {
	s, sender, rec := setup(t)
	rec.status = http.StatusInternalServerError
	syncRows(t, s, map[string][]store.Row{"workouts": {workout(1, int64(1), false)}})
	flush(t, sender)
	if len(rec.payloads) != 1 {
		t.Fatalf("got %d requests, want 1", len(rec.payloads))
	}
	// Rescheduled with backoff, so not due again right away.
	flush(t, sender)
	if len(rec.payloads) != 1 {
		t.Fatalf("retried too early: %d requests", len(rec.payloads))
	}
	due, err := s.DueWebhooks(context.Background(), 10)
	if err != nil || len(due) != 0 {
		t.Fatalf("due=%v err=%v", due, err)
	}
}

func TestBackoff(t *testing.T) {
	for attempts, want := range map[int]time.Duration{0: 30 * time.Second, 1: time.Minute, 3: 4 * time.Minute, 20: time.Hour} {
		if got := backoff(attempts); got != want {
			t.Errorf("backoff(%d) = %s, want %s", attempts, got, want)
		}
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	_, sender, _ := setup(t)
	sender.Poll = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { sender.Run(ctx); close(done) }()
	time.Sleep(5 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}
