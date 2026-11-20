package store

import (
	"context"
	"database/sql"
	"errors"
)

// WorkoutSnapshot is a workout with its live exercises and sets, in order.
type WorkoutSnapshot struct {
	ID         string
	Name       string
	Date       string
	StartedAt  int64  // ms
	FinishedAt *int64 // ms
	Deleted    bool
	Exercises  []ExerciseSnapshot
}

type ExerciseSnapshot struct {
	Name       string
	SupersetID *string
	Sets       []SetSnapshot
}

type SetSnapshot struct {
	Reps        int64
	WeightKg    float64
	Warmup      bool
	RestSeconds int64
	// DurationSeconds is how long a timed set lasted; 0 for sets counted in reps.
	DurationSeconds int64
}

// Workout returns the current state of a workout, or ok=false if it is unknown.
func (s *Store) Workout(ctx context.Context, id string) (w WorkoutSnapshot, ok bool, err error) {
	var finished sql.NullInt64
	var deleted int64
	err = s.db.QueryRowContext(ctx, "SELECT id, name, date, started_at, finished_at, deleted FROM workouts WHERE id = ?", id).
		Scan(&w.ID, &w.Name, &w.Date, &w.StartedAt, &finished, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return w, false, nil
	}
	if err != nil {
		return w, false, err
	}
	if finished.Valid {
		w.FinishedAt = &finished.Int64
	}
	w.Deleted = deleted == 1

	rs, err := s.db.QueryContext(ctx, `SELECT e.id, e.exercise_id, e.superset_id, s.reps, s.weight_kg, s.warmup, s.rest_seconds, s.duration_seconds
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
		var exID, exerciseID string
		var superset sql.NullString
		var reps, warmup, rest, duration sql.NullInt64
		var weight sql.NullFloat64
		if err := rs.Scan(&exID, &exerciseID, &superset, &reps, &weight, &warmup, &rest, &duration); err != nil {
			return w, false, err
		}
		if exID != lastID {
			ex := ExerciseSnapshot{Name: exerciseName(exerciseID), Sets: []SetSnapshot{}}
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
				DurationSeconds: duration.Int64,
			})
		}
	}
	return w, true, rs.Err()
}

// FinishedWorkout returns a finished, not deleted workout, or ok=false.
func (s *Store) FinishedWorkout(ctx context.Context, id string) (WorkoutSnapshot, bool, error) {
	w, ok, err := s.Workout(ctx, id)
	if err != nil || !ok || w.Deleted || w.FinishedAt == nil {
		return WorkoutSnapshot{}, false, err
	}
	return w, true, nil
}

// FinishedWorkouts returns the finished, not deleted workouts whose date is
// between from and to (YYYY-MM-DD, both inclusive), ordered by date and then
// finish time. Running and discarded workouts are left out.
func (s *Store) FinishedWorkouts(ctx context.Context, from, to string) ([]WorkoutSnapshot, error) {
	rs, err := s.db.QueryContext(ctx, `SELECT id FROM workouts
WHERE deleted = 0 AND finished_at IS NOT NULL AND date >= ? AND date <= ?
ORDER BY date, finished_at, id`, from, to)
	if err != nil {
		return nil, err
	}
	ids, err := scanIDs(rs)
	if err != nil {
		return nil, err
	}

	// One query per workout keeps a single shape with Workout; a week or a
	// month holds a handful of workouts.
	out := make([]WorkoutSnapshot, 0, len(ids))
	for _, id := range ids {
		w, ok, err := s.Workout(ctx, id)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, w)
		}
	}
	return out, nil
}
