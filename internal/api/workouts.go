package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/agusespa/gym-tracker/internal/store"
	"github.com/agusespa/gym-tracker/internal/webhook"
)

const maxRangeDays = 366

// workoutList is the response of GET /api/workouts.
type workoutList struct {
	From     string             `json:"from"`
	To       string             `json:"to"`
	Totals   listTotals         `json:"totals"`
	Workouts []*webhook.Workout `json:"workouts"`
}

// listTotals add up the per-workout totals (working sets only).
// DurationSeconds counts only workouts with a recorded start time.
type listTotals struct {
	Workouts        int     `json:"workouts"`
	WorkingSets     int     `json:"working_sets"`
	Reps            int64   `json:"reps"`
	VolumeKg        float64 `json:"volume_kg"`
	DurationSeconds int64   `json:"duration_seconds"`
}

// requireToken accepts only "Authorization: Bearer <t>" for one of tokens.
// Empty tokens never match.
func requireToken(next http.Handler, tokens ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		match := 0
		for _, t := range tokens {
			if t != "" {
				// Check every token so timing doesn't reveal which one matched.
				match |= subtle.ConstantTimeCompare([]byte(got), []byte(t))
			}
		}
		if !ok || match != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="gym-tracker"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func listWorkoutsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		out, err := workoutRange(r.Context(), s, q.Get("from"), q.Get("to"))
		if err != nil {
			var re rangeError
			if errors.As(err, &re) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": re.Error()})
				return
			}
			log.Printf("list workouts: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// rangeError is a bad from/to pair.
type rangeError string

func (e rangeError) Error() string { return string(e) }

// workoutRange lists finished workouts dated from..to (YYYY-MM-DD, inclusive)
// with their totals.
func workoutRange(ctx context.Context, s *store.Store, fromQ, toQ string) (workoutList, error) {
	from, errFrom := time.Parse(time.DateOnly, fromQ)
	to, errTo := time.Parse(time.DateOnly, toQ)
	switch {
	case errFrom != nil || errTo != nil:
		return workoutList{}, rangeError("from and to are required as YYYY-MM-DD")
	case to.Before(from):
		return workoutList{}, rangeError("to is before from")
	case to.Sub(from) >= maxRangeDays*24*time.Hour:
		return workoutList{}, rangeError("range is longer than 366 days")
	}
	fromS, toS := from.Format(time.DateOnly), to.Format(time.DateOnly)

	snaps, err := s.FinishedWorkouts(ctx, fromS, toS)
	if err != nil {
		return workoutList{}, err
	}
	out := workoutList{From: fromS, To: toS, Workouts: make([]*webhook.Workout, 0, len(snaps))}
	for _, snap := range snaps {
		wo := webhook.FromSnapshot(snap)
		out.Workouts = append(out.Workouts, wo)
		out.Totals.WorkingSets += wo.Totals.WorkingSets
		out.Totals.Reps += wo.Totals.Reps
		out.Totals.VolumeKg += wo.Totals.VolumeKg
		out.Totals.DurationSeconds += wo.DurationSeconds
	}
	out.Totals.Workouts = len(out.Workouts)
	out.Totals.VolumeKg = math.Round(out.Totals.VolumeKg*100) / 100
	return out, nil
}

func getWorkoutHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, ok, err := s.Workout(r.Context(), r.PathValue("id"))
		if err != nil {
			log.Printf("get workout: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		// Running, discarded and deleted workouts are not part of the read API.
		if !ok || snap.Deleted || snap.FinishedAt == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, webhook.FromSnapshot(snap))
	}
}
