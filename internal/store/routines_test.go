package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// phoneRoutine syncs a routine the way the app creates one: fractional
// positions, a superset and timestamps from the phone's clock.
func phoneRoutine(t *testing.T, s *Store, at int64) {
	t.Helper()
	exercises := map[string]string{"bench": "bench-press", "fly": "dumbbell-fly", "dips": "dip"}
	ex := func(id string, pos float64, superset any) Row {
		return Row{"id": id, "updated_at": at, "deleted": false, "routine_id": "r1",
			"exercise_id": exercises[id], "position": pos, "superset_id": superset}
	}
	set := func(id, ex string, pos float64, reps int64) Row {
		return Row{"id": id, "updated_at": at, "deleted": false, "routine_exercise_id": ex,
			"position": pos, "reps": reps, "warmup": false, "rest_seconds": int64(90), "duration_seconds": int64(0), "rep_min": int64(0), "rep_max": int64(0)}
	}
	mustSync(t, s, map[string][]Row{
		"routines":          {{"id": "r1", "updated_at": at, "deleted": false, "name": "Push"}},
		"routine_exercises": {ex("bench", 1, "g1"), ex("fly", 1.5, "g1"), ex("dips", 2, nil)},
		"routine_sets": {
			set("b1", "bench", 1, 8), set("b2", "bench", 2, 8),
			set("f1", "fly", 1, 12), set("d1", "dips", 1, 10),
		},
	})
}

func routineStore(t *testing.T) (*Store, *fakeClock) {
	t.Helper()
	s := openTest(t)
	clock := &fakeClock{t: time.UnixMilli(2_000_000)}
	s.now = clock.now
	return s, clock
}

// changedSince returns the ids of rows per table written after cursor.
func changedSince(t *testing.T, s *Store, cursor int64) (int64, map[string][]Row) {
	t.Helper()
	cur, out, _, err := s.Sync(context.Background(), cursor, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cur, out
}

func cursorNow(t *testing.T, s *Store) int64 {
	t.Helper()
	cur, _ := changedSince(t, s, 0)
	return cur
}

func TestCreateRoutine(t *testing.T) {
	s, _ := routineStore(t)
	ctx := context.Background()
	in := Routine{ID: "ignored", Name: "  Legs ", Exercises: []RoutineExercise{
		{ID: "ignored-too", Name: "Squat", Sets: []RoutineSet{{Reps: 5, RestSeconds: 180}, {Reps: 5, RestSeconds: 180}}},
		{Name: "Barbell Curl", SupersetID: ptr("x")}, // a lone superset label is cleared
	}}
	got, err := s.CreateRoutine(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "ignored" || len(got.ID) != 36 || got.Name != "Legs" {
		t.Fatalf("routine = %+v", got)
	}
	if len(got.Exercises) != 2 || got.Exercises[0].ID == "ignored-too" || len(got.Exercises[0].Sets) != 2 {
		t.Fatalf("exercises = %+v", got.Exercises)
	}
	if got.Exercises[0].ExerciseID != "squat" || got.Exercises[1].ExerciseID != "barbell-curl" {
		t.Fatalf("exercise ids = %q, %q", got.Exercises[0].ExerciseID, got.Exercises[1].ExerciseID)
	}
	if got.Exercises[1].SupersetID != nil || got.Exercises[1].Sets == nil {
		t.Fatalf("curl = %+v, want no superset and an empty set list", got.Exercises[1])
	}

	// Phones pull it like any other change.
	_, rows := changedSince(t, s, 0)
	if len(rows["routines"]) != 1 || len(rows["routine_exercises"]) != 2 || len(rows["routine_sets"]) != 2 {
		t.Fatalf("synced rows = %v", rows)
	}
	if rows["routines"][0]["updated_at"] != int64(2_000_000) {
		t.Errorf("updated_at = %v, want server time", rows["routines"][0]["updated_at"])
	}
}

func TestReplaceRoutineIncrementWritesOnlyThatSet(t *testing.T) {
	s, _ := routineStore(t)
	ctx := context.Background()
	phoneRoutine(t, s, 1_000_000)
	r, ok, err := s.Routine(ctx, "r1")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if got := r.Exercises[0].SupersetID; got == nil || *got != "g1" || *r.Exercises[1].SupersetID != "g1" {
		t.Fatalf("supersets = %v %v", r.Exercises[0].SupersetID, r.Exercises[1].SupersetID)
	}

	cursor := cursorNow(t, s)
	r.Exercises[0].Sets[1].Reps = 10
	got, err := s.ReplaceRoutine(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Exercises[0].Sets[1].Reps != 10 || *got.Exercises[0].SupersetID != "g1" {
		t.Fatalf("saved = %+v", got.Exercises[0])
	}
	_, rows := changedSince(t, s, cursor)
	if len(rows) != 1 || len(rows["routine_sets"]) != 1 || rows["routine_sets"][0]["id"] != "b2" {
		t.Fatalf("written = %v, want only set b2", rows)
	}

	// Sending the same routine again writes nothing.
	cursor = cursorNow(t, s)
	if _, err := s.ReplaceRoutine(ctx, got); err != nil {
		t.Fatal(err)
	}
	if cur, rows := changedSince(t, s, cursor); len(rows) != 0 || cur != cursor {
		t.Fatalf("no-op replace wrote %v", rows)
	}
}

func TestReplaceRoutineReshapes(t *testing.T) {
	s, _ := routineStore(t)
	ctx := context.Background()
	phoneRoutine(t, s, 1_000_000)
	r, _, _ := s.Routine(ctx, "r1")

	// Drop fly (ending the superset), move dips first, add a set to bench, add rows.
	bench, dips := r.Exercises[0], r.Exercises[2]
	bench.Sets = append(bench.Sets[:1], RoutineSet{Reps: 6, RestSeconds: 120})
	r.Name = "Push A"
	r.Exercises = []RoutineExercise{dips, bench, {Name: "Triceps Pushdown", Sets: []RoutineSet{{Reps: 12}}}}
	got, err := s.ReplaceRoutine(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Push A" || len(got.Exercises) != 3 ||
		got.Exercises[0].ID != "dips" || got.Exercises[1].ID != "bench" || got.Exercises[2].Name != "Triceps Pushdown" {
		t.Fatalf("saved = %+v", got)
	}
	if got.Exercises[1].SupersetID != nil {
		t.Errorf("bench superset = %v, want cleared once fly is gone", *got.Exercises[1].SupersetID)
	}
	if s := got.Exercises[1].Sets; len(s) != 2 || s[0].ID != "b1" || s[1].Reps != 6 {
		t.Errorf("bench sets = %+v", s)
	}

	_, rows := changedSince(t, s, 0)
	deleted := map[string]bool{}
	for _, table := range []string{"routine_exercises", "routine_sets"} {
		for _, row := range rows[table] {
			if row["deleted"] == true {
				deleted[row["id"].(string)] = true
			}
		}
	}
	if len(deleted) != 3 || !deleted["fly"] || !deleted["f1"] || !deleted["b2"] {
		t.Errorf("deleted = %v, want fly, f1 and b2", deleted)
	}
}

func TestReplaceRoutineBeatsPhoneClockAhead(t *testing.T) {
	s, _ := routineStore(t) // server at 2_000_000
	ctx := context.Background()
	phoneRoutine(t, s, 9_000_000)
	r, _, _ := s.Routine(ctx, "r1")
	r.Exercises[0].Sets[0].Reps = 10
	got, err := s.ReplaceRoutine(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Exercises[0].Sets[0].Reps != 10 {
		t.Fatalf("reps = %d, want 10: write lost to a newer phone timestamp", got.Exercises[0].Sets[0].Reps)
	}
}

func TestReplaceRoutineSupersetLabels(t *testing.T) {
	s, _ := routineStore(t)
	ctx := context.Background()
	phoneRoutine(t, s, 1_000_000)
	r, _, _ := s.Routine(ctx, "r1")
	r.Exercises[2].SupersetID = ptr("new")
	r.Exercises = append(r.Exercises, RoutineExercise{Name: "Lateral Raise", SupersetID: ptr("new")})
	got, err := s.ReplaceRoutine(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	g1, g2 := got.Exercises[0].SupersetID, got.Exercises[2].SupersetID
	if g1 == nil || *g1 != "g1" || g2 == nil || *g2 == "new" || len(*g2) != 36 || *got.Exercises[3].SupersetID != *g2 {
		t.Fatalf("supersets = %v %v %v", g1, g2, got.Exercises[3].SupersetID)
	}
}

func TestReplaceRoutineRejects(t *testing.T) {
	s, _ := routineStore(t)
	ctx := context.Background()
	phoneRoutine(t, s, 1_000_000)
	mustSync(t, s, map[string][]Row{
		"routines":          {{"id": "r2", "updated_at": int64(1), "deleted": false, "name": "Other"}},
		"routine_exercises": {{"id": "other-ex", "updated_at": int64(1), "deleted": false, "routine_id": "r2", "exercise_id": "row", "position": 1.0, "superset_id": nil}},
	})
	base, _, _ := s.Routine(ctx, "r1")
	cursor := cursorNow(t, s)

	cases := map[string]func(r *Routine){
		"empty name":           func(r *Routine) { r.Name = " " },
		"negative reps":        func(r *Routine) { r.Exercises[0].Sets[0].Reps = -1 },
		"long rest":            func(r *Routine) { r.Exercises[0].Sets[0].RestSeconds = 3601 },
		"other routine's ex":   func(r *Routine) { r.Exercises[0].ID = "other-ex" },
		"set of other ex":      func(r *Routine) { r.Exercises[0].Sets[0].ID = "f1" },
		"duplicate id":         func(r *Routine) { r.Exercises[1].ID = r.Exercises[0].ID },
		"unknown exercise id":  func(r *Routine) { r.Exercises[0].ID = "made-up" },
		"empty exercise name":  func(r *Routine) { r.Exercises[1].Name = "" },
		"unknown exercise":     func(r *Routine) { r.Exercises[1].Name = "Zercher Carry" },
		"exercise without set": func(r *Routine) { r.Exercises[0].Sets[0].ID = "d1" },
	}
	for name, mutate := range cases {
		r, _, _ := s.Routine(ctx, "r1")
		mutate(&r)
		var ve *ValidationError
		if _, err := s.ReplaceRoutine(ctx, r); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want ValidationError", name, err)
		}
	}
	if _, rows := changedSince(t, s, cursor); len(rows) != 0 {
		t.Fatalf("rejected writes changed %v", rows)
	}

	base.ID = "missing"
	if _, err := s.ReplaceRoutine(ctx, base); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing routine: err = %v, want ErrNotFound", err)
	}
	mustSync(t, s, map[string][]Row{"routines": {{"id": "r2", "updated_at": int64(2), "deleted": true, "name": "Other"}}})
	base.ID = "r2"
	base.Exercises = nil
	if _, err := s.ReplaceRoutine(ctx, base); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted routine: err = %v, want ErrNotFound", err)
	}
}

func TestRoutinesList(t *testing.T) {
	s, _ := routineStore(t)
	ctx := context.Background()
	phoneRoutine(t, s, 1_000_000)
	if _, err := s.CreateRoutine(ctx, Routine{Name: "arms"}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, s, map[string][]Row{"routines": {{"id": "gone", "updated_at": int64(1), "deleted": true, "name": "Abs"}}})
	got, more, err := s.RoutinePage(ctx, "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if more || len(got) != 2 || got[0].Name != "arms" || got[1].Name != "Push" || len(got[1].Exercises) != 3 {
		t.Fatalf("routines = %+v, more %v", got, more)
	}

	// Paging walks the same order, including ties on name (ordered by id).
	if _, err := s.CreateRoutine(ctx, Routine{Name: "ARMS"}); err != nil {
		t.Fatal(err)
	}
	var names []string
	afterName, afterID := "", ""
	for pages := 0; ; pages++ {
		page, more, err := s.RoutinePage(ctx, afterName, afterID, 1)
		if err != nil || len(page) != 1 || pages > 3 {
			t.Fatalf("page %d: %+v, %v", pages, page, err)
		}
		names = append(names, strings.ToLower(page[0].Name))
		afterName, afterID = page[0].Name, page[0].ID
		if !more {
			break
		}
	}
	if strings.Join(names, ",") != "arms,arms,push" {
		t.Fatalf("paged names = %v", names)
	}
}

func ptr[T any](v T) *T { return &v }

func TestPositions(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		in   []*float64
		want []float64
	}{
		{[]*float64{nil, nil}, []float64{1, 2}},
		{[]*float64{f(1), f(1.5), f(2)}, []float64{1, 1.5, 2}},
		{[]*float64{f(1), nil, f(2)}, []float64{1, 1.5, 2}},
		{[]*float64{nil, f(1), f(2)}, []float64{0, 1, 2}},
		{[]*float64{f(1), f(2), nil, nil}, []float64{1, 2, 3, 4}},
		{[]*float64{f(3), f(1), f(2)}, []float64{0, 1, 2}},            // moved row lands before
		{[]*float64{f(1), f(3), f(2), f(4)}, []float64{1, 3, 3.5, 4}}, // one row moves either way
	}
	for _, c := range cases {
		got := positions(c.in)
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("positions(%v) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestReplaceRoutineReorderWritesOnlyMovedExercise(t *testing.T) {
	s, _ := routineStore(t)
	ctx := context.Background()
	phoneRoutine(t, s, 1_000_000)
	r, _, _ := s.Routine(ctx, "r1")
	r.Exercises = []RoutineExercise{r.Exercises[2], r.Exercises[0], r.Exercises[1]} // dips first
	cursor := cursorNow(t, s)
	got, err := s.ReplaceRoutine(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Exercises[0].ID != "dips" || got.Exercises[1].ID != "bench" || got.Exercises[2].ID != "fly" {
		t.Fatalf("order = %s %s %s", got.Exercises[0].ID, got.Exercises[1].ID, got.Exercises[2].ID)
	}
	_, rows := changedSince(t, s, cursor)
	if len(rows) != 1 || len(rows["routine_exercises"]) != 1 || rows["routine_exercises"][0]["id"] != "dips" {
		t.Fatalf("written = %v, want only dips", rows)
	}
}
