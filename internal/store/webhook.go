package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// The webhook outbox holds one row per finished workout that has to be sent
// (again). Sync schedules a send each time a finished workout, or one of its
// exercises or sets, changes; the delay groups a burst of edits into one send.
// due_at is NULL once the latest version has been delivered.

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
