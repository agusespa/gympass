// Package store persists synced records in SQLite.
//
// Every record has a client-generated id and a client-set updated_at (ms).
// On write the newest updated_at wins. Each accepted write is stamped with a
// server-side seq so clients can ask for "everything since seq N" without
// depending on their own clock.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"unicode/utf8"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Row is one record as exchanged with clients.
type Row map[string]any

// ValidationError means the client sent a malformed change.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &ValidationError{fmt.Sprintf(format, args...)}
}

const (
	maxIDLen   = 64
	maxTextLen = 200
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Single user: one connection serialises writes and avoids SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	migrations := []func(*sql.Tx) error{
		func(tx *sql.Tx) error {
			if _, err := tx.Exec("CREATE TABLE sync_state (id INTEGER PRIMARY KEY CHECK (id = 1), seq INTEGER NOT NULL); INSERT INTO sync_state VALUES (1, 0);"); err != nil {
				return err
			}
			for i := range tables {
				if _, err := tx.Exec(tables[i].createSQL()); err != nil {
					return err
				}
			}
			return nil
		},
		// Columns added to the set tables later. Databases created after a change
		// already have the column from migration 1, so these skip existing ones.
		addSetColumn("warmup", colBool),          // 2
		addSetColumn("rest_seconds", colSeconds), // 3
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if err := migrations[i](tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Sync applies the client's changes and returns every row written after cursor,
// along with the new cursor. Rows the client just pushed are included; they are
// harmless because the client applies the same newest-wins rule.
func (s *Store) Sync(ctx context.Context, cursor int64, changes map[string][]Row) (int64, map[string][]Row, error) {
	if cursor < 0 {
		return 0, nil, invalid("cursor must be >= 0")
	}
	// Validate everything before touching the database.
	prepared := make(map[*table][][]any, len(changes))
	for name, rows := range changes {
		t, ok := tableByName[name]
		if !ok {
			return 0, nil, invalid("unknown table %q", name)
		}
		for i, r := range rows {
			args, err := t.args(r)
			if err != nil {
				return 0, nil, invalid("%s[%d]: %v", name, i, err)
			}
			prepared[t] = append(prepared[t], args)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var seq int64
	if err := tx.QueryRowContext(ctx, "SELECT seq FROM sync_state WHERE id = 1").Scan(&seq); err != nil {
		return 0, nil, err
	}
	if cursor > seq {
		// Client is ahead of us (e.g. the database was replaced): resend everything.
		cursor = 0
	}
	if len(prepared) > 0 {
		next := seq + 1
		wrote := false
		for i := range tables {
			t := &tables[i]
			rows := prepared[t]
			if len(rows) == 0 {
				continue
			}
			stmt, err := tx.PrepareContext(ctx, t.upsertSQL())
			if err != nil {
				return 0, nil, err
			}
			for _, args := range rows {
				args[3] = next
				res, err := stmt.ExecContext(ctx, args...)
				if err != nil {
					_ = stmt.Close()
					return 0, nil, err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					wrote = true
				}
			}
			_ = stmt.Close()
		}
		if wrote {
			seq = next
			if _, err := tx.ExecContext(ctx, "UPDATE sync_state SET seq = ? WHERE id = 1", seq); err != nil {
				return 0, nil, err
			}
		}
	}

	out := make(map[string][]Row)
	for i := range tables {
		rows, err := tables[i].since(ctx, tx, cursor)
		if err != nil {
			return 0, nil, err
		}
		if len(rows) > 0 {
			out[tables[i].name] = rows
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, err
	}
	return seq, out, nil
}

// args validates r and returns the values in colNames order (seq left as 0).
func (t *table) args(r Row) ([]any, error) {
	id, ok := r["id"].(string)
	if !ok || id == "" || len(id) > maxIDLen {
		return nil, errors.New("id must be a non-empty string of at most 64 bytes")
	}
	updatedAt, ok := toInt(r["updated_at"])
	if !ok || updatedAt <= 0 {
		return nil, errors.New("updated_at must be a positive integer")
	}
	deleted, ok := r["deleted"].(bool)
	if !ok {
		return nil, errors.New("deleted must be a boolean")
	}
	args := []any{id, updatedAt, boolInt(deleted), int64(0)}
	for _, c := range t.cols {
		v, err := convert(c, r[c.name])
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	return args, nil
}

func convert(c column, v any) (any, error) {
	switch c.typ {
	case colText, colDate:
		s, ok := v.(string)
		if !ok || utf8.RuneCountInString(s) > maxTextLen {
			return nil, fmt.Errorf("%s must be a string of at most %d characters", c.name, maxTextLen)
		}
		if c.typ == colDate && !dateRe.MatchString(s) {
			return nil, fmt.Errorf("%s must be YYYY-MM-DD", c.name)
		}
		return s, nil
	case colTextNull:
		if v == nil {
			return nil, nil
		}
		s, ok := v.(string)
		if !ok || len(s) > maxIDLen {
			return nil, fmt.Errorf("%s must be null or a string of at most %d bytes", c.name, maxIDLen)
		}
		return s, nil
	case colInt:
		n, ok := toInt(v)
		if !ok || n < 0 {
			return nil, fmt.Errorf("%s must be a non-negative integer", c.name)
		}
		return n, nil
	case colReal:
		f, ok := toFloat(v)
		if !ok {
			return nil, fmt.Errorf("%s must be a number", c.name)
		}
		return f, nil
	case colBool:
		if v == nil {
			return int64(0), nil
		}
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be a boolean", c.name)
		}
		return boolInt(b), nil
	case colSeconds:
		if v == nil {
			return int64(defaultRestSeconds), nil
		}
		n, ok := toInt(v)
		if !ok || n < 0 || n > maxRestSeconds {
			return nil, fmt.Errorf("%s must be an integer from 0 to %d", c.name, maxRestSeconds)
		}
		return n, nil
	}
	return nil, fmt.Errorf("%s: unsupported column type", c.name)
}

func (t *table) since(ctx context.Context, tx *sql.Tx, cursor int64) ([]Row, error) {
	rs, err := tx.QueryContext(ctx, t.selectSinceSQL(), cursor)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rs.Close() }()
	names := t.colNames()
	var out []Row
	for rs.Next() {
		vals := make([]any, len(names))
		ptrs := make([]any, len(names))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := Row{
			"id":         vals[0],
			"updated_at": vals[1],
			"deleted":    vals[2] == int64(1),
		}
		for i, c := range t.cols {
			v := vals[4+i]
			switch c.typ {
			case colReal:
				if f, ok := toFloat(v); ok {
					v = f
				}
			case colBool:
				v = v == int64(1)
			}
			r[c.name] = v
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// addSetColumn returns a migration adding a column to both set tables.
func addSetColumn(col string, typ colType) func(*sql.Tx) error {
	return func(tx *sql.Tx) error {
		for _, t := range []string{"sets", "routine_sets"} {
			if err := addColumnIfMissing(tx, t, col, sqlType(typ)); err != nil {
				return err
			}
		}
		return nil
	}
}

func addColumnIfMissing(tx *sql.Tx, table, col, typ string) error {
	var n int
	if err := tx.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, col).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, typ))
	return err
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return toInt(f)
	case float64:
		if n != math.Trunc(n) || math.Abs(n) > 1<<53 {
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
	case float64:
		return n, !math.IsInf(n, 0) && !math.IsNaN(n)
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
