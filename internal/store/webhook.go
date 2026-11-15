package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// The webhook outbox holds one row per finished workout that has to be sent
// (again). Sync schedules a send each time a finished workout, or one of its
// exercises or sets, changes; the delay groups a burst of edits into one send.
// due_at is NULL once the latest version has been delivered.

func createWebhookOutbox(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE webhook_outbox (
  workout_id TEXT PRIMARY KEY,
  due_at INTEGER,
  attempts INTEGER NOT NULL DEFAULT 0,
  sent INTEGER NOT NULL DEFAULT 0
)`)
	return err
}

// EnableWebhookOutbox makes Sync schedule a webhook send delay after a
// finished workout changes. Call it before serving requests.
func (s *Store) EnableWebhookOutbox(delay time.Duration) {
	s.outboxDelay = delay
	s.outboxOn = true
}

// PendingWebhook is a workout whose latest version has not been delivered.
type PendingWebhook struct {
	WorkoutID string
	DueAt     int64 // ms; pass back to the Mark* methods
	Attempts  int
	Sent      bool // an earlier version was delivered
}

// WorkoutSnapshot is a workout with its live exercises and sets, in order.
type WorkoutSnapshot struct {
	ID         string
	Name       string
	Date       string
	StartedAt  *int64 // ms; nil for workouts from before it was recorded
	FinishedAt *int64 // ms
	Deleted    bool
	// BodyWeightKg is the weigh-in for the workout's date (see BodyWeightOn);
	// nil when no body weight was ever logged.
	BodyWeightKg *float64
	Exercises    []ExerciseSnapshot
}

type ExerciseSnapshot struct {
	Name       string
	SupersetID *string
	// BodyweightPct is the share of body weight each rep moves (0..100).
	BodyweightPct int64
	Sets          []SetSnapshot
}

type SetSnapshot struct {
	Reps        int64
	WeightKg    float64
	Warmup      bool
	RestSeconds int64
}

func (s *Store) nowMs() int64 {
	if s.now != nil {
		return s.now().UnixMilli()
	}
	return time.Now().UnixMilli()
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

// scheduleWebhooks queues a send for every finished workout touched by the
// rows Sync just wrote (args in colNames order, per table).
func (s *Store) scheduleWebhooks(ctx context.Context, tx *sql.Tx, written map[*table][][]any) error {
	ids := map[string]bool{}
	for t, rows := range written {
		for _, args := range rows {
			switch t.name {
			case "workouts":
				ids[args[0].(string)] = true
			case "workout_exercises":
				ids[args[t.colIndex("workout_id")].(string)] = true
			case "sets":
				var wid string
				err := tx.QueryRowContext(ctx, "SELECT workout_id FROM workout_exercises WHERE id = ?",
					args[t.colIndex("workout_exercise_id")]).Scan(&wid)
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				if err != nil {
					return err
				}
				ids[wid] = true
			}
		}
	}
	due := s.nowMs() + s.outboxDelay.Milliseconds()
	for id := range ids {
		// Only finished workouts are sent. A deleted one keeps its finished_at,
		// so its deletion is scheduled too; a discarded running one never is.
		_, err := tx.ExecContext(ctx, `INSERT INTO webhook_outbox (workout_id, due_at)
SELECT id, ? FROM workouts WHERE id = ? AND finished_at IS NOT NULL
ON CONFLICT(workout_id) DO UPDATE SET due_at = excluded.due_at, attempts = 0`, due, id)
		if err != nil {
			return err
		}
	}
	return nil
}

// DueWebhooks returns up to limit pending sends whose time has come, oldest first.
func (s *Store) DueWebhooks(ctx context.Context, limit int) ([]PendingWebhook, error) {
	rs, err := s.db.QueryContext(ctx,
		"SELECT workout_id, due_at, attempts, sent FROM webhook_outbox WHERE due_at IS NOT NULL AND due_at <= ? ORDER BY due_at LIMIT ?",
		s.nowMs(), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rs.Close() }()
	var out []PendingWebhook
	for rs.Next() {
		var p PendingWebhook
		var sent int64
		if err := rs.Scan(&p.WorkoutID, &p.DueAt, &p.Attempts, &sent); err != nil {
			return nil, err
		}
		p.Sent = sent == 1
		out = append(out, p)
	}
	return out, rs.Err()
}

// MarkWebhookSent records a delivery. If the workout changed again since
// DueWebhooks returned it (due_at moved), the newer send stays pending, but
// the receiver still counts as having seen the workout.
func (s *Store) MarkWebhookSent(ctx context.Context, p PendingWebhook) error {
	_, err := s.db.ExecContext(ctx, `UPDATE webhook_outbox SET sent = 1,
  attempts = CASE WHEN due_at = ?1 THEN 0 ELSE attempts END,
  due_at = CASE WHEN due_at = ?1 THEN NULL ELSE due_at END
WHERE workout_id = ?2`, p.DueAt, p.WorkoutID)
	return err
}

// SkipWebhook clears a pending send that turned out to have nothing to deliver.
func (s *Store) SkipWebhook(ctx context.Context, p PendingWebhook) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE webhook_outbox SET due_at = NULL, attempts = 0 WHERE workout_id = ? AND due_at = ?",
		p.WorkoutID, p.DueAt)
	return err
}

// RetryWebhook reschedules a failed send after the given delay.
func (s *Store) RetryWebhook(ctx context.Context, p PendingWebhook, after time.Duration) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE webhook_outbox SET due_at = ?, attempts = attempts + 1 WHERE workout_id = ? AND due_at = ?",
		s.nowMs()+after.Milliseconds(), p.WorkoutID, p.DueAt)
	return err
}

// Workout returns the current state of a workout, or ok=false if it is unknown.
func (s *Store) Workout(ctx context.Context, id string) (w WorkoutSnapshot, ok bool, err error) {
	var started, finished sql.NullInt64
	var deleted int64
	err = s.db.QueryRowContext(ctx, "SELECT id, name, date, started_at, finished_at, deleted FROM workouts WHERE id = ?", id).
		Scan(&w.ID, &w.Name, &w.Date, &started, &finished, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return w, false, nil
	}
	if err != nil {
		return w, false, err
	}
	if started.Valid {
		w.StartedAt = &started.Int64
	}
	if finished.Valid {
		w.FinishedAt = &finished.Int64
	}
	w.Deleted = deleted == 1
	if w.BodyWeightKg, err = s.BodyWeightOn(ctx, w.Date); err != nil {
		return w, false, err
	}
	shares, err := s.BodyweightShares(ctx)
	if err != nil {
		return w, false, err
	}

	rs, err := s.db.QueryContext(ctx, `SELECT e.id, e.name, e.superset_id, s.reps, s.weight_kg, s.warmup, s.rest_seconds
FROM workout_exercises e
LEFT JOIN sets s ON s.workout_exercise_id = e.id AND s.deleted = 0 AND s.done = 1
WHERE e.workout_id = ? AND e.deleted = 0
ORDER BY e.position, e.id, s.position, s.id`, id)
	if err != nil {
		return w, false, err
	}
	defer func() { _ = rs.Close() }()
	lastID := ""
	for rs.Next() {
		var exID, name string
		var superset sql.NullString
		var reps, warmup, rest sql.NullInt64
		var weight sql.NullFloat64
		if err := rs.Scan(&exID, &name, &superset, &reps, &weight, &warmup, &rest); err != nil {
			return w, false, err
		}
		if exID != lastID {
			ex := ExerciseSnapshot{Name: name, BodyweightPct: shares[ExerciseKey(name)], Sets: []SetSnapshot{}}
			if superset.Valid {
				ex.SupersetID = &superset.String
			}
			w.Exercises = append(w.Exercises, ex)
			lastID = exID
		}
		if reps.Valid {
			ex := &w.Exercises[len(w.Exercises)-1]
			ex.Sets = append(ex.Sets, SetSnapshot{
				Reps: reps.Int64, WeightKg: weight.Float64, Warmup: warmup.Int64 == 1, RestSeconds: rest.Int64,
			})
		}
	}
	return w, true, rs.Err()
}

// ExerciseKey is how exercises are matched by name: trimmed and lowercased.
// The client uses the same rule (exerciseKey in web/src/lib/stats.ts).
func ExerciseKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// BodyWeightOn returns the body weight for a date: the latest weigh-in on or
// before it, or the earliest one for dates before the first weigh-in. It is
// nil when no body weight was ever logged.
func (s *Store) BodyWeightOn(ctx context.Context, date string) (*float64, error) {
	var kg float64
	err := s.db.QueryRowContext(ctx, `SELECT weight_kg FROM body_weights WHERE deleted = 0
ORDER BY date <= ?1 DESC, CASE WHEN date <= ?1 THEN date END DESC, date, updated_at DESC LIMIT 1`, date).Scan(&kg)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &kg, nil
}

// BodyweightShares maps exercise keys to their bodyweight_pct. When several
// settings rows share a key (e.g. created on two phones), the newest wins.
func (s *Store) BodyweightShares(ctx context.Context) (map[string]int64, error) {
	rs, err := s.db.QueryContext(ctx,
		"SELECT name, bodyweight_pct FROM exercise_settings WHERE deleted = 0 ORDER BY updated_at, id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rs.Close() }()
	out := map[string]int64{}
	for rs.Next() {
		var name string
		var pct int64
		if err := rs.Scan(&name, &pct); err != nil {
			return nil, err
		}
		out[ExerciseKey(name)] = pct
	}
	return out, rs.Err()
}
