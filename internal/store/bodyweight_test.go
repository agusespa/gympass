package store

import (
	"context"
	"testing"
)

func weighIn(id, date string, kg float64, updatedAt int64, deleted bool) Row {
	return Row{"id": id, "updated_at": updatedAt, "deleted": deleted, "date": date, "weight_kg": kg}
}

func TestBodyWeightOn(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	got, err := s.BodyWeightOn(ctx, "2026-11-15")
	if err != nil || got != nil {
		t.Fatalf("no weigh-ins: got %v, %v; want nil", got, err)
	}

	mustSync(t, s, map[string][]Row{"body_weights": {
		weighIn("a", "2026-11-01", 80, 1, false),
		weighIn("b", "2026-11-10", 82, 1, false),
		// Two entries on one date: the newest edit wins.
		weighIn("c", "2026-11-12", 83, 1, false),
		weighIn("d", "2026-11-12", 84, 2, false),
		weighIn("gone", "2026-11-11", 99, 1, true),
	}})
	for date, want := range map[string]float64{
		"2026-10-01": 80, // before the first weigh-in: the earliest one
		"2026-11-01": 80,
		"2026-11-05": 80,
		"2026-11-10": 82,
		"2026-11-11": 82, // deleted weigh-in is ignored
		"2026-11-12": 84,
		"2027-01-01": 84,
	} {
		got, err := s.BodyWeightOn(ctx, date)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || *got != want {
			t.Errorf("%s: got %v, want %v", date, got, want)
		}
	}
}

func TestBodyweightShares(t *testing.T) {
	s := openTest(t)
	setting := func(id, name string, pct, updatedAt int64, deleted bool) Row {
		return Row{"id": id, "updated_at": updatedAt, "deleted": deleted, "name": name, "bodyweight_pct": pct}
	}
	mustSync(t, s, map[string][]Row{"exercise_settings": {
		setting("a", "Pull-up", 50, 1, false),
		setting("b", " pull-UP ", 100, 2, false), // same exercise, newer
		setting("c", "Push-up", 65, 1, false),
		setting("d", "Dip", 100, 1, true),
	}})
	got, err := s.BodyweightShares(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["pull-up"] != 100 || got["push-up"] != 65 {
		t.Fatalf("shares = %v", got)
	}

	_, _, err = s.Sync(context.Background(), 0, map[string][]Row{"exercise_settings": {setting("x", "Dip", 101, 1, false)}})
	if err == nil {
		t.Fatal("bodyweight_pct 101 accepted")
	}
}

func TestWorkoutSnapshotBodyWeight(t *testing.T) {
	s := openTest(t)
	w := workout("w1", 1, "Pull")
	w["date"] = "2026-11-14"
	w["finished_at"] = int64(5)
	mustSync(t, s, map[string][]Row{
		"workouts": {w},
		"workout_exercises": {
			{"id": "e1", "updated_at": int64(1), "deleted": false, "workout_id": "w1", "name": "Pull-up", "position": 1.0},
			{"id": "e2", "updated_at": int64(1), "deleted": false, "workout_id": "w1", "name": "Row", "position": 2.0},
		},
		"body_weights":      {weighIn("bw", "2026-11-01", 80, 1, false)},
		"exercise_settings": {{"id": "x", "updated_at": int64(1), "deleted": false, "name": "pull-up", "bodyweight_pct": int64(100)}},
	})
	snap, ok, err := s.Workout(context.Background(), "w1")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if snap.BodyWeightKg == nil || *snap.BodyWeightKg != 80 {
		t.Fatalf("body weight = %v, want 80", snap.BodyWeightKg)
	}
	if snap.Exercises[0].BodyweightPct != 100 || snap.Exercises[1].BodyweightPct != 0 {
		t.Fatalf("exercises = %+v", snap.Exercises)
	}
}
