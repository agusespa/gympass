package store

import "context"

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
	var ids []string
	for rs.Next() {
		var id string
		if err := rs.Scan(&id); err != nil {
			_ = rs.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rs.Err()
	if cerr := rs.Close(); err == nil {
		err = cerr
	}
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
