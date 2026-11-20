package store

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

type colType int

const (
	colText     colType = iota // string of at most maxTextLen characters
	colDate                    // YYYY-MM-DD
	colRef                     // id of another row
	colRefNull                 // id of another row, or null
	colInt                     // integer from 0 to column.max
	colTime                    // ms timestamp
	colTimeNull                // ms timestamp, or null when not set
	colNum                     // number from 0 to column.max
	colPosition                // any finite number; orders siblings
	colBool
)

// The client clamps input to these bounds (web/src/lib/limits.ts).
const (
	maxTextLen     = 200
	maxIDLen       = maxTextLen * utf8.UTFMax
	maxWeightKg    = 10000
	maxReps        = 10000
	maxRestSeconds = 3600
	// maxDurationSeconds bounds a timed set (e.g. a dead hang), 10 hours.
	maxDurationSeconds = 36000
	maxWeeklyGoal      = 7
	// SettingsID is the id of the only settings row.
	SettingsID = "user"
)

type column struct {
	name string
	typ  colType
	max  float64 // upper bound for colInt and colNum
}

type table struct {
	name    string
	cols    []column
	indexes []string // columns other rows are looked up by
	// id, when set, returns the only id a row may have, from its column
	// values. A singleton table uses it so every device writes the same row
	// instead of a duplicate.
	id func(col func(name string) any) string
}

// tables lists every synced table. Each also has id, updated_at, deleted and
// seq. The client (web/src/lib/db.ts) mirrors these definitions.
var tables = []table{
	{name: "workouts", cols: []column{
		{name: "name", typ: colText},
		{name: "date", typ: colDate},
		{name: "started_at", typ: colTime},
		{name: "finished_at", typ: colTimeNull}, // null while in progress
	}, indexes: []string{"date"}},
	{name: "workout_exercises", cols: []column{
		{name: "workout_id", typ: colRef},
		{name: "exercise_id", typ: colRef}, // id of a catalogue entry (catalog.json)
		{name: "position", typ: colPosition},
		{name: "superset_id", typ: colRefNull},
	}, indexes: []string{"workout_id"}},
	{name: "sets", cols: []column{
		{name: "workout_exercise_id", typ: colRef},
		{name: "position", typ: colPosition},
		{name: "reps", typ: colInt, max: maxReps},
		{name: "weight_kg", typ: colNum, max: maxWeightKg},
		{name: "warmup", typ: colBool},
		{name: "rest_seconds", typ: colInt, max: maxRestSeconds},
		{name: "done", typ: colBool},
		// Set length for timed exercises (a catalogue entry with measure time); 0 otherwise.
		{name: "duration_seconds", typ: colInt, max: maxDurationSeconds},
		// Target rep range, e.g. 8 to 12, copied from the routine; both 0 when
		// the set has no range. reps is what was actually done.
		{name: "rep_min", typ: colInt, max: maxReps},
		{name: "rep_max", typ: colInt, max: maxReps},
	}, indexes: []string{"workout_exercise_id"}},
	{name: "routines", cols: []column{
		{name: "name", typ: colText},
	}},
	{name: "routine_exercises", cols: []column{
		{name: "routine_id", typ: colRef},
		{name: "exercise_id", typ: colRef}, // id of a catalogue entry (catalog.json)
		{name: "position", typ: colPosition},
		{name: "superset_id", typ: colRefNull},
	}, indexes: []string{"routine_id"}},
	{name: "routine_sets", cols: []column{
		{name: "routine_exercise_id", typ: colRef},
		{name: "position", typ: colPosition},
		{name: "reps", typ: colInt, max: maxReps},
		{name: "warmup", typ: colBool},
		{name: "rest_seconds", typ: colInt, max: maxRestSeconds},
		{name: "duration_seconds", typ: colInt, max: maxDurationSeconds},
		// Target rep range; both 0 for a fixed rep count. With a range, reps
		// equals rep_min.
		{name: "rep_min", typ: colInt, max: maxReps},
		{name: "rep_max", typ: colInt, max: maxReps},
	}, indexes: []string{"routine_exercise_id"}},
	// User settings: a single row with id SettingsID. weekly_goal is
	// workouts per week, 0 for no goal.
	{name: "settings", cols: []column{
		{name: "weekly_goal", typ: colInt, max: maxWeeklyGoal},
	}, id: func(func(string) any) string { return SettingsID }},
}

var tableByName = func() map[string]*table {
	m := make(map[string]*table, len(tables))
	for i := range tables {
		m[tables[i].name] = &tables[i]
	}
	return m
}()

// migrations[i] brings a database from user_version i to i+1. Older
// databases are not supported: after a schema change, start from a new one.
var migrations = []func(*sql.Tx) error{
	createSchema,
}

func createSchema(tx *sql.Tx) error {
	stmts := []string{
		"CREATE TABLE sync_state (id INTEGER PRIMARY KEY CHECK (id = 1), seq INTEGER NOT NULL)",
		"INSERT INTO sync_state VALUES (1, 0)",
		// One row per finished workout whose webhook has to be sent (again);
		// due_at is NULL once the latest version has been delivered.
		`CREATE TABLE webhook_outbox (
  workout_id TEXT PRIMARY KEY,
  due_at INTEGER,
  attempts INTEGER NOT NULL DEFAULT 0,
  sent INTEGER NOT NULL DEFAULT 0
)`,
		"CREATE INDEX webhook_outbox_due_at ON webhook_outbox(due_at)",
	}
	for i := range tables {
		stmts = append(stmts, tables[i].createSQL()...)
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
	}
	return nil
}

func sqlType(t colType) string {
	switch t {
	case colRefNull:
		return "TEXT"
	case colTimeNull:
		return "INTEGER"
	case colInt, colBool, colTime:
		return "INTEGER NOT NULL"
	case colNum, colPosition:
		return "REAL NOT NULL"
	default:
		return "TEXT NOT NULL"
	}
}

func (t *table) createSQL() []string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n  id TEXT PRIMARY KEY,\n  updated_at INTEGER NOT NULL,\n  deleted INTEGER NOT NULL,\n  seq INTEGER NOT NULL", t.name)
	for _, c := range t.cols {
		fmt.Fprintf(&b, ",\n  %s %s", c.name, sqlType(c.typ))
	}
	b.WriteString("\n)")
	out := []string{b.String(), fmt.Sprintf("CREATE INDEX %s_seq ON %s(seq)", t.name, t.name)}
	for _, col := range t.indexes {
		out = append(out, fmt.Sprintf("CREATE INDEX %s_%s ON %s(%s)", t.name, col, t.name, col))
	}
	return out
}

func (t *table) colNames() []string {
	names := []string{"id", "updated_at", "deleted", "seq"}
	for _, c := range t.cols {
		names = append(names, c.name)
	}
	return names
}

// colIndex returns the position of a column in colNames order.
func (t *table) colIndex(name string) int {
	for i, n := range t.colNames() {
		if n == name {
			return i
		}
	}
	panic("unknown column " + name)
}

// upsertSQL writes a row (values in colNames order) unless the stored one has
// the same or a newer updated_at.
func (t *table) upsertSQL() string {
	names := t.colNames()
	sets := make([]string, 0, len(names)-1)
	for _, n := range names[1:] {
		sets = append(sets, fmt.Sprintf("%s = excluded.%s", n, n))
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ")
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(id) DO UPDATE SET %s WHERE excluded.updated_at > %s.updated_at",
		t.name, strings.Join(names, ", "), placeholders, strings.Join(sets, ", "), t.name,
	)
}

func (t *table) selectSinceSQL() string {
	return fmt.Sprintf("SELECT %s FROM %s WHERE seq > ? ORDER BY seq", strings.Join(t.colNames(), ", "), t.name)
}
