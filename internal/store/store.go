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
	"sort"
	"time"
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

// maxClockSkew is how far ahead of the server a client's updated_at may be.
// A device with a clock further ahead would otherwise win every conflict on
// the rows it touches until real time caught up.
const maxClockSkew = 24 * time.Hour

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type Store struct {
	db *sql.DB

	outboxOn    bool
	outboxDelay time.Duration
	now         func() time.Time // for tests; nil means time.Now
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// FULL makes every commit durable before Sync answers: a phone marks its
	// rows synced on that answer and never resends them.
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

// Ready checks the sync metadata and write access without committing changes.
func (s *Store) Ready(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, "UPDATE sync_state SET seq = seq WHERE id = 1")
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("sync metadata is missing")
	}
	return nil
}

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("database schema version %d is newer than this build supports (%d)", version, len(migrations))
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

// Rejection is a row Sync refused, with the reason. The rest of the batch is
// still applied, so one bad row never blocks a device's other changes.
type Rejection struct {
	Table string `json:"table"`
	ID    string `json:"id"` // empty when the row had no usable id
	Error string `json:"error"`
}

// Sync applies the client's valid changes and returns every row written after
// cursor, the new cursor and the rows it refused. Rows the client just pushed
// are included; they are harmless because the client applies the same
// newest-wins rule. Only a bad cursor or a database failure is an error.
func (s *Store) Sync(ctx context.Context, cursor int64, changes map[string][]Row) (int64, map[string][]Row, []Rejection, error) {
	if cursor < 0 {
		return 0, nil, nil, invalid("cursor must be >= 0")
	}
	// Validate everything before touching the database.
	now := s.nowMs()
	rows := make(map[*table][][]any, len(changes))
	var rejected []Rejection
	for name, list := range changes {
		t, ok := tableByName[name]
		for _, r := range list {
			if !ok {
				rejected = append(rejected, Rejection{Table: name, ID: rowID(r), Error: fmt.Sprintf("unknown table %q", name)})
				continue
			}
			args, err := t.args(r, now)
			if err != nil {
				rejected = append(rejected, Rejection{Table: name, ID: rowID(r), Error: err.Error()})
				continue
			}
			rows[t] = append(rows[t], args)
		}
	}
	sort.Slice(rejected, func(i, j int) bool {
		a, b := rejected[i], rejected[j]
		return a.Table < b.Table || a.Table == b.Table && a.ID < b.ID
	})

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	seq, written, err := writeRows(ctx, tx, rows)
	if err != nil {
		return 0, nil, nil, err
	}
	if cursor > seq {
		// Client is ahead of us (e.g. the database was replaced): resend everything.
		cursor = 0
	}
	if s.outboxOn && len(written) > 0 {
		if err := s.scheduleWebhooks(ctx, tx, written); err != nil {
			return 0, nil, nil, err
		}
	}

	out := make(map[string][]Row)
	for i := range tables {
		rows, err := tables[i].since(ctx, tx, cursor)
		if err != nil {
			return 0, nil, nil, err
		}
		if len(rows) > 0 {
			out[tables[i].name] = rows
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, nil, err
	}
	return seq, out, rejected, nil
}

// rowID is r's id for a rejection, or "" when it has none.
func rowID(r Row) string {
	id, _ := r["id"].(string)
	if len(id) > maxIDLen {
		return ""
	}
	return id
}

// writeRows upserts rows (values in colNames order) where they are newer than
// the stored ones, stamping all that change with one new seq. It returns the
// current seq and the rows actually written.
func writeRows(ctx context.Context, tx *sql.Tx, rows map[*table][][]any) (int64, map[*table][][]any, error) {
	var seq int64
	if err := tx.QueryRowContext(ctx, "SELECT seq FROM sync_state WHERE id = 1").Scan(&seq); err != nil {
		return 0, nil, err
	}
	next := seq + 1
	written := make(map[*table][][]any)
	for i := range tables {
		t := &tables[i]
		if len(rows[t]) == 0 {
			continue
		}
		stmt, err := tx.PrepareContext(ctx, t.upsertSQL())
		if err != nil {
			return 0, nil, err
		}
		for _, args := range rows[t] {
			args[3] = next
			res, err := stmt.ExecContext(ctx, args...)
			if err != nil {
				_ = stmt.Close()
				return 0, nil, err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				written[t] = append(written[t], args)
			}
		}
		if err := stmt.Close(); err != nil {
			return 0, nil, err
		}
	}
	if len(written) == 0 {
		return seq, written, nil
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sync_state SET seq = ? WHERE id = 1", next); err != nil {
		return 0, nil, err
	}
	return next, written, nil
}

// args validates r and returns its values in colNames order, seq left as 0.
// now (ms) bounds updated_at.
func (t *table) args(r Row, now int64) ([]any, error) {
	id, ok := r["id"].(string)
	if !ok || id == "" || len(id) > maxIDLen {
		return nil, fmt.Errorf("id must be a non-empty string of at most %d bytes", maxIDLen)
	}
	updatedAt, ok := toInt(r["updated_at"])
	if !ok || updatedAt <= 0 {
		return nil, errors.New("updated_at must be a positive integer")
	}
	if updatedAt > now+maxClockSkew.Milliseconds() {
		return nil, errors.New("updated_at is more than a day ahead of the server; check the device clock")
	}
	deleted, ok := r["deleted"].(bool)
	if !ok {
		return nil, errors.New("deleted must be a boolean")
	}
	args := []any{id, updatedAt, boolInt(deleted), int64(0)}
	for _, c := range t.cols {
		v, has := r[c.name]
		if !has {
			return nil, fmt.Errorf("%s is missing", c.name)
		}
		v, err := convert(c, v)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	if t.id != nil {
		col := func(name string) any { return args[t.colIndex(name)] }
		if want := t.id(col); id != want {
			return nil, fmt.Errorf("id must be %q", want)
		}
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
	case colRef, colRefNull:
		if v == nil && c.typ == colRefNull {
			return nil, nil
		}
		s, ok := v.(string)
		if !ok || s == "" || len(s) > maxIDLen {
			return nil, fmt.Errorf("%s must be an id of at most %d bytes", c.name, maxIDLen)
		}
		return s, nil
	case colTime:
		if v == nil {
			return nil, nil
		}
		n, ok := toInt(v)
		if !ok || n < 0 {
			return nil, fmt.Errorf("%s must be null or a non-negative integer", c.name)
		}
		return n, nil
	case colInt:
		n, ok := toInt(v)
		if !ok || n < 0 || float64(n) > c.max {
			return nil, fmt.Errorf("%s must be an integer from 0 to %v", c.name, c.max)
		}
		return n, nil
	case colNum:
		f, ok := toFloat(v)
		if !ok || f < 0 || f > c.max {
			return nil, fmt.Errorf("%s must be a number from 0 to %v", c.name, c.max)
		}
		return f, nil
	case colPosition:
		f, ok := toFloat(v)
		if !ok {
			return nil, fmt.Errorf("%s must be a number", c.name)
		}
		return f, nil
	case colBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be a boolean", c.name)
		}
		return boolInt(b), nil
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
			case colNum, colPosition:
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
