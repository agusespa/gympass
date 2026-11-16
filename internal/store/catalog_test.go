package store

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"testing"
)

func TestCatalog(t *testing.T) {
	if len(catalog) == 0 || len(catalogByID) != len(catalog) || len(catalogByKey) != len(catalog) {
		t.Errorf("%d entries with %d distinct ids and %d distinct names", len(catalog), len(catalogByID), len(catalogByKey))
	}
	id := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	for _, e := range catalog {
		if !id.MatchString(e.ID) || e.Name == "" || e.Name != TrimName(e.Name) ||
			!slices.Contains(measures, e.Measure) || !slices.Contains(equipments, e.Equipment) {
			t.Errorf("bad entry %+v", e)
		}
	}

	var names []string
	for _, e := range Exercises() {
		names = append(names, ExerciseKey(e.Name))
	}
	if len(names) != len(catalog) || !slices.IsSorted(names) {
		t.Errorf("listed names = %v", names)
	}
}

func TestRoutineExercisesByName(t *testing.T) {
	s, _ := routineStore(t)
	ctx := context.Background()
	names := func(list ...string) Routine {
		r := Routine{Name: "Mix"}
		for _, n := range list {
			r.Exercises = append(r.Exercises, RoutineExercise{Name: n})
		}
		return r
	}
	got, err := s.CreateRoutine(ctx, names(" bench press", "PULL-UP", "Bench Press"))
	if err != nil {
		t.Fatal(err)
	}
	type ex struct{ id, name string }
	var exercises []ex
	for _, e := range got.Exercises {
		exercises = append(exercises, ex{e.ExerciseID, e.Name})
	}
	want := []ex{{"bench-press", "Bench Press"}, {"pull-up", "Pull-Up"}, {"bench-press", "Bench Press"}}
	if !slices.Equal(exercises, want) {
		t.Fatalf("exercises = %v, want %v", exercises, want)
	}

	var ve *ValidationError
	if _, err := s.CreateRoutine(ctx, names("Zercher Carry")); !errors.As(err, &ve) {
		t.Fatalf("unknown exercise: err = %v, want ValidationError", err)
	}
}
