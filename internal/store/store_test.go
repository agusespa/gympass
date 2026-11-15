package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func workout(id string, updatedAt int64, name string) Row {
	return Row{"id": id, "updated_at": updatedAt, "deleted": false, "name": name, "date": "2026-11-14", "started_at": nil, "finished_at": nil}
}

// exerciseRow is a workout exercise outside any superset.
func exerciseRow(id, workoutID, name string, pos float64) Row {
	return Row{"id": id, "updated_at": int64(1), "deleted": false, "workout_id": workoutID, "name": name, "position": pos, "superset_id": nil}
}

// setRow is a checked working set with the default rest; tests override fields.
func setRow(id, exercise string, pos float64, reps int64, kg float64) Row {
	return Row{"id": id, "updated_at": int64(1), "deleted": false, "workout_exercise_id": exercise, "position": pos,
		"reps": reps, "weight_kg": kg, "warmup": false, "rest_seconds": int64(120), "done": true,
		"duration_seconds": int64(0), "rep_min": int64(0), "rep_max": int64(0)}
}

func TestSyncRoundTrip(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	cur, out, _, err := s.Sync(ctx, 0, map[string][]Row{
		"workouts": {workout("w1", 100, "Push")},
		"workout_exercises": {{
			"id": "e1", "updated_at": int64(100), "deleted": false,
			"workout_id": "w1", "name": "Bench", "position": 1.5, "superset_id": "s1",
		}},
		"sets": {{
			"id": "s1", "updated_at": int64(100), "deleted": false,
			"workout_exercise_id": "e1", "position": 1.0, "reps": int64(8), "weight_kg": 82.5, "warmup": true, "rest_seconds": int64(90), "duration_seconds": int64(0), "rep_min": int64(0), "rep_max": int64(0), "done": true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cur != 1 {
		t.Fatalf("cursor = %d, want 1", cur)
	}
	if len(out["workouts"]) != 1 || len(out["workout_exercises"]) != 1 || len(out["sets"]) != 1 {
		t.Fatalf("unexpected changes: %v", out)
	}
	set := out["sets"][0]
	if set["reps"] != int64(8) || set["weight_kg"] != 82.5 || set["warmup"] != true || set["rest_seconds"] != int64(90) || set["deleted"] != false {
		t.Fatalf("set = %v", set)
	}
	if out["workout_exercises"][0]["superset_id"] != "s1" {
		t.Fatalf("superset_id lost: %v", out["workout_exercises"][0])
	}

	// Nothing new since cursor.
	cur2, out2, _, err := s.Sync(ctx, cur, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cur2 != cur || len(out2) != 0 {
		t.Fatalf("expected no changes, got cursor %d %v", cur2, out2)
	}
}

func TestNewestWins(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	if _, _, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 200, "New")}}); err != nil {
		t.Fatal(err)
	}
	// Older write is ignored and does not advance the cursor.
	cur, _, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 100, "Old")}})
	if err != nil {
		t.Fatal(err)
	}
	if cur != 1 {
		t.Fatalf("cursor advanced on ignored write: %d", cur)
	}
	// Equal timestamp keeps the existing row.
	_, out, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 200, "Same")}})
	if err != nil {
		t.Fatal(err)
	}
	if got := out["workouts"][0]["name"]; got != "New" {
		t.Fatalf("name = %v, want New", got)
	}
	// Newer write replaces it.
	cur, out, _, err = s.Sync(ctx, 1, map[string][]Row{"workouts": {workout("w1", 300, "Newer")}})
	if err != nil {
		t.Fatal(err)
	}
	if cur != 2 || out["workouts"][0]["name"] != "Newer" {
		t.Fatalf("cursor %d, out %v", cur, out)
	}
}

func TestDeleteSyncs(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	cur, _, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 100, "Push")}})
	if err != nil {
		t.Fatal(err)
	}
	del := workout("w1", 200, "Push")
	del["deleted"] = true
	if _, _, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {del}}); err != nil {
		t.Fatal(err)
	}
	// Another device that synced before the delete gets the tombstone.
	_, out, _, err := s.Sync(ctx, cur, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out["workouts"]) != 1 || out["workouts"][0]["deleted"] != true {
		t.Fatalf("tombstone not returned: %v", out)
	}
}

func TestCursorAheadResendsAll(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if _, _, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 100, "Push")}}); err != nil {
		t.Fatal(err)
	}
	_, out, _, err := s.Sync(ctx, 999, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out["workouts"]) != 1 {
		t.Fatalf("expected full resend, got %v", out)
	}
}

func TestValidation(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	w := func(k string, v any) Row {
		r := workout("w", 1, "x")
		r[k] = v
		return r
	}
	set := func(k string, v any) Row {
		r := setRow("s", "e", 1, 1, 1)
		r[k] = v
		return r
	}
	without := func(r Row, k string) Row {
		delete(r, k)
		return r
	}
	bad := []struct {
		name    string
		changes map[string][]Row
	}{
		{"unknown table", map[string][]Row{"nope": {workout("w1", 1, "x")}}},
		{"missing id", map[string][]Row{"workouts": {without(workout("w", 1, "x"), "id")}}},
		{"missing column", map[string][]Row{"workouts": {without(workout("w", 1, "x"), "started_at")}}},
		{"bad updated_at", map[string][]Row{"workouts": {w("updated_at", 1.5)}}},
		{"updated_at in the future", map[string][]Row{"workouts": {w("updated_at", time.Now().Add(25*time.Hour).UnixMilli())}}},
		{"bad deleted", map[string][]Row{"workouts": {w("deleted", 0)}}},
		{"bad date", map[string][]Row{"workouts": {w("date", "14/11/2026")}}},
		{"empty ref", map[string][]Row{"sets": {set("workout_exercise_id", "")}}},
		{"negative reps", map[string][]Row{"sets": {set("reps", int64(-1))}}},
		{"too many reps", map[string][]Row{"sets": {set("reps", int64(10001))}}},
		{"negative weight", map[string][]Row{"sets": {set("weight_kg", -1.0)}}},
		{"string warmup", map[string][]Row{"sets": {set("warmup", "yes")}}},
		{"null done", map[string][]Row{"sets": {set("done", nil)}}},
		{"rest too long", map[string][]Row{"sets": {set("rest_seconds", int64(3601))}}},
		{"negative rest", map[string][]Row{"sets": {set("rest_seconds", int64(-1))}}},
		{"string weight", map[string][]Row{"sets": {set("weight_kg", "1")}}},
		{"weigh-in id not its date", map[string][]Row{"body_weights": {weighIn("x", "2026-11-01", 80, 1, false)}}},
		{"setting id not its key", map[string][]Row{"exercise_settings": {setting("Pull-up", 100, 1, false, "pull-up ")}}},
		{"settings id", map[string][]Row{"settings": {{"id": "other", "updated_at": int64(1), "deleted": false, "weekly_goal": int64(3)}}}},
		{"weekly goal too high", map[string][]Row{"settings": {{"id": SettingsID, "updated_at": int64(1), "deleted": false, "weekly_goal": int64(8)}}}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, _, rejected, err := s.Sync(ctx, 0, tc.changes)
			if err != nil || len(rejected) != 1 || rejected[0].Error == "" {
				t.Fatalf("want one rejection, got %v (err %v)", rejected, err)
			}
		})
	}
	if _, _, _, err := s.Sync(ctx, -1, nil); !errors.As(err, new(*ValidationError)) {
		t.Fatalf("negative cursor: %v", err)
	}
	// Nothing was written by the rejected rows.
	cur, out, _, err := s.Sync(ctx, 0, nil)
	if err != nil || cur != 0 || len(out) != 0 {
		t.Fatalf("cursor %d out %v err %v", cur, out, err)
	}
}

// One bad row is refused on its own: the rest of the batch is stored, and the
// rejection names the row so the client can keep it unsynced.
func TestSyncRejectsRowsIndividually(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	heavy := setRow("s-bad", "e1", 2, 5, 20000)
	cur, out, rejected, err := s.Sync(ctx, 0, map[string][]Row{
		"workouts": {workout("w1", 1, "Push")},
		"sets":     {setRow("s-ok", "e1", 1, 5, 100), heavy},
		"from_v9":  {{"id": "n1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cur != 1 || len(out["workouts"]) != 1 || len(out["sets"]) != 1 || out["sets"][0]["id"] != "s-ok" {
		t.Fatalf("cursor %d, out %v", cur, out)
	}
	want := []Rejection{
		{Table: "from_v9", ID: "n1", Error: `unknown table "from_v9"`},
		{Table: "sets", ID: "s-bad", Error: "weight_kg must be a number from 0 to 10000"},
	}
	if fmt.Sprint(rejected) != fmt.Sprint(want) {
		t.Fatalf("rejected = %v", rejected)
	}
}

func TestReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.Sync(context.Background(), 0, map[string][]Row{"routines": {{"id": "r1", "updated_at": int64(5), "deleted": false, "name": "Legs"}}}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	cur, out, _, err := s.Sync(context.Background(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cur != 1 || len(out["routines"]) != 1 {
		t.Fatalf("cursor %d out %v", cur, out)
	}
}

// A database written by a newer build is refused rather than misread.
func TestNewerSchemaRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations)+1)); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if s, err := Open(path); err == nil {
		_ = s.Close()
		t.Fatal("opened a database from a newer build")
	}
}

func TestTimedSetsSync(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	hang := setRow("s1", "e1", 1, 0, 0)
	hang["duration_seconds"] = int64(45)
	_, out, rejected, err := s.Sync(ctx, 0, map[string][]Row{
		"workouts":          {workout("w1", 1, "Grip")},
		"workout_exercises": {exerciseRow("e1", "w1", "Dead hang", 1)},
		"sets":              {hang, setRow("s2", "e1", 2, 8, 20)},
		"exercise_settings": {{"id": "dead hang", "updated_at": int64(1), "deleted": false,
			"name": "Dead hang", "bodyweight_pct": int64(0), "timed": true}},
	})
	if err != nil || len(rejected) > 0 {
		t.Fatal(err, rejected)
	}
	got := map[any]any{}
	for _, r := range out["sets"] {
		got[r["id"]] = r["duration_seconds"]
	}
	if got["s1"] != int64(45) || got["s2"] != int64(0) {
		t.Fatalf("duration_seconds = %v", got)
	}
	if out["exercise_settings"][0]["timed"] != true {
		t.Fatalf("timed = %v", out["exercise_settings"][0]["timed"])
	}

	bad := setRow("s3", "e1", 3, 0, 0)
	bad["duration_seconds"] = int64(maxDurationSeconds + 1)
	_, _, rejected, err = s.Sync(ctx, 0, map[string][]Row{"sets": {bad}})
	if err != nil || len(rejected) != 1 {
		t.Fatalf("over-long set: err %v, rejected %v", err, rejected)
	}
}

func TestRepRangeSync(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	ranged := setRow("s1", "e1", 1, 10, 60)
	ranged["rep_min"], ranged["rep_max"] = int64(8), int64(12)
	_, out, rejected, err := s.Sync(ctx, 0, map[string][]Row{
		"workouts":          {workout("w1", 1, "Push")},
		"workout_exercises": {exerciseRow("e1", "w1", "Bench press", 1)},
		"sets":              {ranged, setRow("s2", "e1", 2, 8, 60)},
	})
	if err != nil || len(rejected) > 0 {
		t.Fatal(err, rejected)
	}
	got := map[any][2]any{}
	for _, r := range out["sets"] {
		got[r["id"]] = [2]any{r["rep_min"], r["rep_max"]}
	}
	if got["s1"] != [2]any{int64(8), int64(12)} || got["s2"] != [2]any{int64(0), int64(0)} {
		t.Fatalf("rep ranges = %v", got)
	}
}
