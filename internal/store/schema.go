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
	colTime                    // ms timestamp, or null when not set
	colNum                     // number from 0 to column.max
	colPosition                // any finite number; orders siblings
	colBool
)

// The client clamps input to these bounds (web/src/lib/limits.ts).
const (
	maxTextLen = 200
	// exercise_settings uses the exercise key (a lowercased name of up to
	// maxTextLen characters) as its id, so ids allow that many bytes.
	maxIDLen       = maxTextLen * utf8.UTFMax
	maxWeightKg    = 10000
	maxReps        = 10000
	maxRestSeconds = 3600
	// maxDurationSeconds bounds a timed set (e.g. a dead hang), 10 hours.
	maxDurationSeconds = 36000
	maxWeeklyGoal      = 7 // mirrors MAX_WEEKLY_GOAL in web/src/lib/repo.ts
	// SettingsID is the id of the only settings row.
	SettingsID = "user"
)

type column struct {
	name string
	typ  colType
	max  float64 // upper bound for colInt and colNum
	// def, when set, is used when a client leaves the column out, so app
	// versions from before the column was added keep syncing.
	def any
}

type table struct {
	name    string
	cols    []column
	indexes []string // columns other rows are looked up by
	// id, when set, returns the only id a row may have, from its column
	// values. Tables keyed by a date, an exercise or a singleton use it so
	// every device writes the same row instead of a duplicate.
	id func(col func(name string) any) string
}

// tables lists every synced table. Each also has id, updated_at, deleted and
// seq. The client (web/src/lib/db.ts) mirrors these definitions.
var tables = []table{
	{name: "workouts", cols: []column{
		{name: "name", typ: colText},
		{name: "date", typ: colDate},
		{name: "started_at", typ: colTime},
		{name: "finished_at", typ: colTime}, // null while in progress
	}, indexes: []string{"date"}},
	{name: "workout_exercises", cols: []column{
		{name: "workout_id", typ: colRef},
		{name: "name", typ: colText},
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
		// Set length for timed exercises (see exercise_settings.timed); 0 otherwise.
		{name: "duration_seconds", typ: colInt, max: maxDurationSeconds, def: int64(0)},
	}, indexes: []string{"workout_exercise_id"}},
	{name: "routines", cols: []column{
		{name: "name", typ: colText},
	}},
	{name: "routine_exercises", cols: []column{
		{name: "routine_id", typ: colRef},
		{name: "name", typ: colText},
		{name: "position", typ: colPosition},
		{name: "superset_id", typ: colRefNull},
	}, indexes: []string{"routine_id"}},
	{name: "routine_sets", cols: []column{
		{name: "routine_exercise_id", typ: colRef},
		{name: "position", typ: colPosition},
		{name: "reps", typ: colInt, max: maxReps},
		{name: "weight_kg", typ: colNum, max: maxWeightKg},
		{name: "warmup", typ: colBool},
		{name: "rest_seconds", typ: colInt, max: maxRestSeconds},
		{name: "duration_seconds", typ: colInt, max: maxDurationSeconds, def: int64(0)},
	}, indexes: []string{"routine_exercise_id"}},
	// One weigh-in per date, with the date as id. Workouts use the latest one
	// on or before their date.
	{name: "body_weights", cols: []column{
		{name: "date", typ: colDate},
		{name: "weight_kg", typ: colNum, max: maxWeightKg},
	}, id: func(col func(string) any) string { return col("date").(string) }},
	// Per-exercise settings, with ExerciseKey(name) as id. bodyweight_pct is
	// the share of body weight each rep moves, e.g. 100 for pull-ups. timed
	// exercises (e.g. dead hangs) log duration_seconds instead of reps.
	{name: "exercise_settings", cols: []column{
		{name: "name", typ: colText},
		{name: "bodyweight_pct", typ: colInt, max: 100},
		{name: "timed", typ: colBool, def: false},
	}, id: func(col func(string) any) string { return ExerciseKey(col("name").(string)) }},
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

// migrations[i] brings a database from user_version i to i+1. Released
// migrations are never edited; schema changes append a new one.
var migrations = []func(*sql.Tx) error{
	createSchema,
	addDefaultedColumns,
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

// addDefaultedColumns adds the columns that have a default (sets and
// routine_sets duration_seconds, exercise_settings timed) to databases
// created before them. createSchema builds new databases from the current
// tables, so a column may already be there.
func addDefaultedColumns(tx *sql.Tx) error {
	for i := range tables {
		t := &tables[i]
		have := map[string]bool{}
		rs, err := tx.Query(fmt.Sprintf("SELECT name FROM pragma_table_info('%s')", t.name))
		if err != nil {
			return err
		}
		for rs.Next() {
			var n string
			if err := rs.Scan(&n); err != nil {
				_ = rs.Close()
				return err
			}
			have[n] = true
		}
		if err := rs.Close(); err != nil {
			return err
		}
		for _, c := range t.cols {
			if c.def == nil || have[c.name] {
				continue
			}
			q := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", t.name, c.name, columnSQL(c))
			if _, err := tx.Exec(q); err != nil {
				return fmt.Errorf("%s: %w", q, err)
			}
		}
	}
	return nil
}

// columnSQL is a column's type and constraints, with its default if it has one.
func columnSQL(c column) string {
	sqlT := sqlType(c.typ)
	switch d := c.def.(type) {
	case nil:
	case bool:
		sqlT += fmt.Sprintf(" DEFAULT %d", boolInt(d))
	default:
		sqlT += fmt.Sprintf(" DEFAULT %v", d)
	}
	return sqlT
}

func sqlType(t colType) string {
	switch t {
	case colRefNull:
		return "TEXT"
	case colTime:
		return "INTEGER"
	case colInt, colBool:
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
		fmt.Fprintf(&b, ",\n  %s %s", c.name, columnSQL(c))
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
