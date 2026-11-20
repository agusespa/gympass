package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/agusespa/gym-tracker/internal/store"
	"github.com/agusespa/gym-tracker/internal/webhook"
)

const maxRangeDays = 366

// workoutList is the response of GET /api/workouts: one page of workouts,
// with totals for the whole range.
type workoutList struct {
	From       string             `json:"from"`
	To         string             `json:"to"`
	Totals     listTotals         `json:"totals"`
	Workouts   []*webhook.Workout `json:"workouts"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

// listTotals add up the per-workout totals (working sets only).
type listTotals struct {
	Workouts        int   `json:"workouts"`
	WorkingSets     int   `json:"working_sets"`
	Reps            int64 `json:"reps"`
	DurationSeconds int64 `json:"duration_seconds"`
}

func listWorkoutsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, err := queryLimit(q.Get("limit"))
		var out workoutList
		if err == nil {
			out, err = workoutRange(r.Context(), s, q.Get("from"), q.Get("to"), limit, q.Get("cursor"))
		}
		if err != nil {
			writeError(w, "list workouts", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// workoutRange lists one page of the finished workouts dated from..to
// (YYYY-MM-DD, inclusive), oldest first, with totals for the whole range.
func workoutRange(ctx context.Context, s *store.Store, fromQ, toQ string, limit int, cursor string) (workoutList, error) {
	limit, err := pageSize(limit)
	if err != nil {
		return workoutList{}, err
	}
	// A workout's place in the order: date, finished_at, id.
	after, err := decodeCursor(cursor, 3)
	if err != nil {
		return workoutList{}, err
	}
	from, errFrom := time.Parse(time.DateOnly, fromQ)
	to, errTo := time.Parse(time.DateOnly, toQ)
	switch {
	case errFrom != nil || errTo != nil:
		return workoutList{}, paramError("from and to are required as YYYY-MM-DD")
	case to.Before(from):
		return workoutList{}, paramError("to is before from")
	case to.Sub(from) >= maxRangeDays*24*time.Hour:
		return workoutList{}, paramError("range is longer than 366 days")
	}
	fromS, toS := from.Format(time.DateOnly), to.Format(time.DateOnly)

	snaps, err := s.FinishedWorkouts(ctx, fromS, toS)
	if err != nil {
		return workoutList{}, err
	}
	out := workoutList{From: fromS, To: toS, Workouts: make([]*webhook.Workout, 0, min(limit, len(snaps)))}
	var lastKey []string
	more := false
	for _, snap := range snaps {
		wo := webhook.FromSnapshot(snap)
		out.Totals.Workouts++
		out.Totals.WorkingSets += wo.Totals.WorkingSets
		out.Totals.Reps += wo.Totals.Reps
		out.Totals.DurationSeconds += wo.DurationSeconds

		key := []string{snap.Date, strconv.FormatInt(*snap.FinishedAt, 10), snap.ID}
		switch {
		case after != nil && !workoutAfter(key, after):
		case len(out.Workouts) == limit:
			more = true
		default:
			out.Workouts = append(out.Workouts, wo)
			lastKey = key
		}
	}
	if more {
		out.NextCursor = encodeCursor(lastKey...)
	}
	return out, nil
}

// workoutAfter compares workout order keys (date, finished_at ms, id).
func workoutAfter(key, after []string) bool {
	if key[0] != after[0] {
		return key[0] > after[0]
	}
	k, _ := strconv.ParseInt(key[1], 10, 64)
	a, _ := strconv.ParseInt(after[1], 10, 64)
	if k != a {
		return k > a
	}
	return key[2] > after[2]
}

func getWorkoutHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Running, discarded and deleted workouts are not part of the read API.
		snap, ok, err := s.FinishedWorkout(r.Context(), r.PathValue("id"))
		if err == nil && !ok {
			err = store.ErrNotFound
		}
		if err != nil {
			writeError(w, "get workout", err)
			return
		}
		writeJSON(w, http.StatusOK, webhook.FromSnapshot(snap))
	}
}
