package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func weighIn(id, date string, kg float64, updatedAt int64, deleted bool) Row {
	return Row{"id": id, "updated_at": updatedAt, "deleted": deleted, "date": date, "weight_kg": kg}
}

func setting(name string, pct, updatedAt int64, deleted bool, id string) Row {
	return Row{"id": id, "updated_at": updatedAt, "deleted": deleted, "name": name, "bodyweight_pct": pct}
}

func TestBodyWeightOn(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	got, err := s.BodyWeightOn(ctx, "2026-11-15")
	if err != nil || got != nil {
		t.Fatalf("no weigh-ins: got %v, %v; want nil", got, err)
	}

	mustSync(t, s, map[string][]Row{"body_weights": {
		weighIn("2026-11-01", "2026-11-01", 80, 1, false),
		weighIn("2026-11-10", "2026-11-10", 82, 1, false),
		weighIn("2026-11-12", "2026-11-12", 83, 1, false),
		weighIn("2026-11-11", "2026-11-11", 99, 1, true),
	}})
	// A second phone logging the same date edits the same row.
	mustSync(t, s, map[string][]Row{"body_weights": {weighIn("2026-11-12", "2026-11-12", 84, 2, false)}})
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
	mustSync(t, s, map[string][]Row{"exercise_settings": {
		setting("Pull-up", 50, 1, false, "pull-up"),
		setting("Push-up", 65, 1, false, "push-up"),
		setting("Dip", 100, 1, true, "dip"),
	}})
	// Same exercise typed differently on another phone: same row, newer wins.
	mustSync(t, s, map[string][]Row{"exercise_settings": {setting(" pull-UP ", 100, 2, false, "pull-up")}})
	got, err := s.BodyweightShares(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["pull-up"] != 100 || got["push-up"] != 65 {
		t.Fatalf("shares = %v", got)
	}

	_, _, rejected, err := s.Sync(context.Background(), 0, map[string][]Row{"exercise_settings": {setting("Dip", 101, 1, false, "dip")}})
	if err != nil || len(rejected) != 1 {
		t.Fatalf("bodyweight_pct 101: err %v, rejected %v", err, rejected)
	}
}

func TestWorkoutSnapshotBodyWeight(t *testing.T) {
	s := openTest(t)
	w := finishedWorkout("w1", 5)
	mustSync(t, s, map[string][]Row{
		"workouts":          {w},
		"workout_exercises": {exerciseRow("e1", "w1", "Pull-up", 1), exerciseRow("e2", "w1", "Row", 2)},
		"body_weights":      {weighIn("2026-11-01", "2026-11-01", 80, 1, false)},
		"exercise_settings": {setting("pull-up", 100, 1, false, "pull-up")},
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

// The client computes the same keys (e2e/exercise-key.spec.ts reads the same
// file); a mismatch would make the server refuse exercise_settings rows.
func TestExerciseKeyVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/exercise_keys.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ Name, Key string }
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		if got := ExerciseKey(v.Name); got != v.Key {
			t.Errorf("ExerciseKey(%q) = %q, want %q", v.Name, got, v.Key)
		}
	}
}
