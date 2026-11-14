package store

import (
	"fmt"
	"strings"
)

type colType int

const (
	colText colType = iota
	colTextNull
	colDate
	colInt
	colReal
	colBool    // JSON boolean stored as 0/1; missing means false
	colSeconds // rest duration, 0..maxRestSeconds; missing means defaultRestSeconds
)

const (
	defaultRestSeconds = 120
	maxRestSeconds     = 3600
)

type column struct {
	name string
	typ  colType
}

type table struct {
	name string
	cols []column
}

// tables lists every synced table. Each also has id, updated_at, deleted and seq.
// The client (web/src/lib/db.ts) mirrors these definitions.
var tables = []table{
	{"workouts", []column{{"name", colText}, {"date", colDate}}},
	{"workout_exercises", []column{{"workout_id", colText}, {"name", colText}, {"position", colReal}, {"superset_id", colTextNull}}},
	{"sets", []column{{"workout_exercise_id", colText}, {"position", colReal}, {"reps", colInt}, {"weight_kg", colReal}, {"warmup", colBool}, {"rest_seconds", colSeconds}}},
	{"routines", []column{{"name", colText}}},
	{"routine_exercises", []column{{"routine_id", colText}, {"name", colText}, {"position", colReal}, {"superset_id", colTextNull}}},
	{"routine_sets", []column{{"routine_exercise_id", colText}, {"position", colReal}, {"reps", colInt}, {"weight_kg", colReal}, {"warmup", colBool}, {"rest_seconds", colSeconds}}},
}

var tableByName = func() map[string]*table {
	m := make(map[string]*table, len(tables))
	for i := range tables {
		m[tables[i].name] = &tables[i]
	}
	return m
}()

func sqlType(t colType) string {
	switch t {
	case colInt, colBool:
		return "INTEGER NOT NULL DEFAULT 0"
	case colSeconds:
		return fmt.Sprintf("INTEGER NOT NULL DEFAULT %d", defaultRestSeconds)
	case colReal:
		return "REAL NOT NULL DEFAULT 0"
	case colTextNull:
		return "TEXT"
	default:
		return "TEXT NOT NULL DEFAULT ''"
	}
}

func (t *table) createSQL() string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n  id TEXT PRIMARY KEY,\n  updated_at INTEGER NOT NULL,\n  deleted INTEGER NOT NULL DEFAULT 0,\n  seq INTEGER NOT NULL", t.name)
	for _, c := range t.cols {
		fmt.Fprintf(&b, ",\n  %s %s", c.name, sqlType(c.typ))
	}
	b.WriteString("\n);\n")
	fmt.Fprintf(&b, "CREATE INDEX %s_seq ON %s(seq);\n", t.name, t.name)
	return b.String()
}

func (t *table) colNames() []string {
	names := []string{"id", "updated_at", "deleted", "seq"}
	for _, c := range t.cols {
		names = append(names, c.name)
	}
	return names
}

func (t *table) upsertSQL() string {
	names := t.colNames()
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ")
	sets := make([]string, 0, len(names)-1)
	for _, n := range names[1:] {
		sets = append(sets, fmt.Sprintf("%s = excluded.%s", n, n))
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(id) DO UPDATE SET %s WHERE excluded.updated_at > %s.updated_at",
		t.name, strings.Join(names, ", "), placeholders, strings.Join(sets, ", "), t.name,
	)
}

func (t *table) selectSinceSQL() string {
	return fmt.Sprintf("SELECT %s FROM %s WHERE seq > ? ORDER BY seq", strings.Join(t.colNames(), ", "), t.name)
}
