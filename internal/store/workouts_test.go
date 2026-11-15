package store

import (
	"context"
	"testing"
)

func TestFinishedWorkouts(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	dated := func(id, date string, finished int64) Row {
		r := workout(id, 100, id)
		r["date"] = date
		if finished > 0 {
			r["finished_at"] = finished
		}
		return r
	}
	deleted := dated("gone", "2026-11-11", 500)
	deleted["deleted"] = true
	mustSync(t, s, map[string][]Row{
		"workouts": {
			dated("late", "2026-11-12", 900),
			dated("early", "2026-11-12", 300),
			dated("first", "2026-11-10", 700),
			dated("last-day", "2026-11-16", 700),
			dated("running", "2026-11-12", 0),
			dated("before", "2026-11-09", 700),
			dated("after", "2026-11-17", 700),
			deleted,
		},
		"workout_exercises": {{
			"id": "e1", "updated_at": int64(100), "deleted": false,
			"workout_id": "early", "name": "Bench", "position": 1.0, "superset_id": nil,
		}},
		"sets": {setRow("s1", "e1", 1, 8, 80)},
	})

	got, err := s.FinishedWorkouts(ctx, "2026-11-10", "2026-11-16")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, w := range got {
		ids = append(ids, w.ID)
	}
	want := []string{"first", "early", "late", "last-day"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
	if ex := got[1].Exercises; len(ex) != 1 || len(ex[0].Sets) != 1 {
		t.Fatalf("early exercises = %+v, want one exercise with its set", ex)
	}

	none, err := s.FinishedWorkouts(ctx, "2027-01-01", "2027-01-07")
	if err != nil {
		t.Fatal(err)
	}
	if none == nil || len(none) != 0 {
		t.Fatalf("empty range = %#v, want empty non-nil slice", none)
	}
}
