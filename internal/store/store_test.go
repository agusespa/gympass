package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
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
	return Row{"id": id, "updated_at": updatedAt, "deleted": false, "name": name, "date": "2026-11-14"}
}

func TestSyncRoundTrip(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	cur, out, err := s.Sync(ctx, 0, map[string][]Row{
		"workouts": {workout("w1", 100, "Push")},
		"workout_exercises": {{
			"id": "e1", "updated_at": int64(100), "deleted": false,
			"workout_id": "w1", "name": "Bench", "position": 1.5, "superset_id": "s1",
		}},
		"sets": {{
			"id": "s1", "updated_at": int64(100), "deleted": false,
			"workout_exercise_id": "e1", "position": 1.0, "reps": int64(8), "weight_kg": 82.5, "warmup": true, "rest_seconds": int64(90),
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
	cur2, out2, err := s.Sync(ctx, cur, nil)
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

	if _, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 200, "New")}}); err != nil {
		t.Fatal(err)
	}
	// Older write is ignored and does not advance the cursor.
	cur, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 100, "Old")}})
	if err != nil {
		t.Fatal(err)
	}
	if cur != 1 {
		t.Fatalf("cursor advanced on ignored write: %d", cur)
	}
	// Equal timestamp keeps the existing row.
	_, out, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 200, "Same")}})
	if err != nil {
		t.Fatal(err)
	}
	if got := out["workouts"][0]["name"]; got != "New" {
		t.Fatalf("name = %v, want New", got)
	}
	// Newer write replaces it.
	cur, out, err = s.Sync(ctx, 1, map[string][]Row{"workouts": {workout("w1", 300, "Newer")}})
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
	cur, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 100, "Push")}})
	if err != nil {
		t.Fatal(err)
	}
	del := workout("w1", 200, "Push")
	del["deleted"] = true
	if _, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {del}}); err != nil {
		t.Fatal(err)
	}
	// Another device that synced before the delete gets the tombstone.
	_, out, err := s.Sync(ctx, cur, nil)
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
	if _, _, err := s.Sync(ctx, 0, map[string][]Row{"workouts": {workout("w1", 100, "Push")}}); err != nil {
		t.Fatal(err)
	}
	_, out, err := s.Sync(ctx, 999, nil)
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
	bad := []struct {
		name    string
		changes map[string][]Row
	}{
		{"unknown table", map[string][]Row{"nope": {workout("w1", 1, "x")}}},
		{"missing id", map[string][]Row{"workouts": {{"updated_at": int64(1), "deleted": false, "name": "x", "date": "2026-01-01"}}}},
		{"bad updated_at", map[string][]Row{"workouts": {{"id": "w", "updated_at": 1.5, "deleted": false, "name": "x", "date": "2026-01-01"}}}},
		{"bad deleted", map[string][]Row{"workouts": {{"id": "w", "updated_at": int64(1), "deleted": 0, "name": "x", "date": "2026-01-01"}}}},
		{"bad date", map[string][]Row{"workouts": {{"id": "w", "updated_at": int64(1), "deleted": false, "name": "x", "date": "14/11/2026"}}}},
		{"negative reps", map[string][]Row{"sets": {{"id": "s", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e", "position": 1.0, "reps": int64(-1), "weight_kg": 1.0}}}},
		{"string warmup", map[string][]Row{"sets": {{"id": "s", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e", "position": 1.0, "reps": int64(1), "weight_kg": 1.0, "warmup": "yes"}}}},
		{"rest too long", map[string][]Row{"sets": {{"id": "s", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e", "position": 1.0, "reps": int64(1), "weight_kg": 1.0, "rest_seconds": int64(3601)}}}},
		{"negative rest", map[string][]Row{"sets": {{"id": "s", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e", "position": 1.0, "reps": int64(1), "weight_kg": 1.0, "rest_seconds": int64(-1)}}}},
		{"string weight", map[string][]Row{"sets": {{"id": "s", "updated_at": int64(1), "deleted": false, "workout_exercise_id": "e", "position": 1.0, "reps": int64(1), "weight_kg": "1"}}}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := s.Sync(ctx, 0, tc.changes)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want ValidationError, got %v", err)
			}
		})
	}
	// Nothing was written by the rejected batches.
	cur, out, err := s.Sync(ctx, 0, nil)
	if err != nil || cur != 0 || len(out) != 0 {
		t.Fatalf("cursor %d out %v err %v", cur, out, err)
	}
}

func TestReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Sync(context.Background(), 0, map[string][]Row{"routines": {{"id": "r1", "updated_at": int64(5), "deleted": false, "name": "Legs"}}}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	cur, out, err := s.Sync(context.Background(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cur != 1 || len(out["routines"]) != 1 {
		t.Fatalf("cursor %d out %v", cur, out)
	}
}

// Older clients don't send the newer set fields.
func TestMissingSetFieldsGetDefaults(t *testing.T) {
	s := openTest(t)
	_, out, err := s.Sync(context.Background(), 0, map[string][]Row{"routine_sets": {{
		"id": "rs1", "updated_at": int64(1), "deleted": false,
		"routine_exercise_id": "re1", "position": 1.0, "reps": int64(5), "weight_kg": 60.0,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	rs := out["routine_sets"][0]
	if rs["warmup"] != false || rs["rest_seconds"] != int64(defaultRestSeconds) {
		t.Fatalf("routine set = %v", rs)
	}
}

// A database created before warm-ups and rest times gains the columns and keeps its rows.
func TestMigrateFromV1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"CREATE TABLE sync_state (id INTEGER PRIMARY KEY CHECK (id = 1), seq INTEGER NOT NULL); INSERT INTO sync_state VALUES (1, 1);",
		"CREATE TABLE workouts (id TEXT PRIMARY KEY, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0, seq INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '', date TEXT NOT NULL DEFAULT '')",
		"CREATE TABLE workout_exercises (id TEXT PRIMARY KEY, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0, seq INTEGER NOT NULL, workout_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '', position REAL NOT NULL DEFAULT 0, superset_id TEXT)",
		"CREATE TABLE sets (id TEXT PRIMARY KEY, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0, seq INTEGER NOT NULL, workout_exercise_id TEXT NOT NULL DEFAULT '', position REAL NOT NULL DEFAULT 0, reps INTEGER NOT NULL DEFAULT 0, weight_kg REAL NOT NULL DEFAULT 0)",
		"CREATE TABLE routines (id TEXT PRIMARY KEY, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0, seq INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '')",
		"CREATE TABLE routine_exercises (id TEXT PRIMARY KEY, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0, seq INTEGER NOT NULL, routine_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '', position REAL NOT NULL DEFAULT 0, superset_id TEXT)",
		"CREATE TABLE routine_sets (id TEXT PRIMARY KEY, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0, seq INTEGER NOT NULL, routine_exercise_id TEXT NOT NULL DEFAULT '', position REAL NOT NULL DEFAULT 0, reps INTEGER NOT NULL DEFAULT 0, weight_kg REAL NOT NULL DEFAULT 0)",
		"INSERT INTO sets VALUES ('s1', 1, 0, 1, 'e1', 1, 8, 80)",
		"INSERT INTO workouts VALUES ('w1', 42, 0, 1, 'Old', '2026-01-01')",
		"PRAGMA user_version = 1",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	_ = db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	_, out, err := s.Sync(context.Background(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out["sets"]) != 1 || out["sets"][0]["warmup"] != false || out["sets"][0]["reps"] != int64(8) ||
		out["sets"][0]["rest_seconds"] != int64(defaultRestSeconds) {
		t.Fatalf("sets = %v", out["sets"])
	}
	// Existing workouts are history: finished at their last edit.
	if len(out["workouts"]) != 1 || out["workouts"][0]["finished_at"] != int64(42) {
		t.Fatalf("workouts = %v", out["workouts"])
	}
}
