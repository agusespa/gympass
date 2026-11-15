package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/agusespa/gym-tracker/internal/store"
)

const defaultRestSeconds = 120

// routineInput is the body of POST /api/routines and PUT /api/routines/{id}.
// Unknown fields are rejected so a misspelt field fails loudly instead of
// being dropped. The jsonschema tags describe the MCP tools' input; fields
// without omitempty are required there.
type routineInput struct {
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name"`
	Exercises []exerciseInput `json:"exercises,omitempty"`
}

type exerciseInput struct {
	ID         string     `json:"id,omitempty" jsonschema:"id of an existing exercise of this routine to keep it; leave out to add a new exercise"`
	Name       string     `json:"name" jsonschema:"exercise name, e.g. Bench press"`
	SupersetID *string    `json:"superset_id,omitempty" jsonschema:"adjacent exercises with the same value form a superset; any string works as a label for a new one"`
	Sets       []setInput `json:"sets,omitempty" jsonschema:"planned sets, in order"`
}

type setInput struct {
	ID          string   `json:"id,omitempty" jsonschema:"id of an existing set of this exercise to keep it; leave out to add a new set"`
	Reps        *int64   `json:"reps" jsonschema:"planned repetitions, 0 to 10000"`
	WeightKg    *float64 `json:"weight_kg" jsonschema:"planned weight in kilograms, 0 for bodyweight"`
	Warmup      bool     `json:"warmup,omitempty" jsonschema:"true for a warm-up set; warm-ups are left out of totals"`
	RestSeconds *int64   `json:"rest_seconds,omitempty" jsonschema:"rest after the set in seconds, 0 to 3600; defaults to 120"`
}

func (in routineInput) routine() (store.Routine, error) {
	r := store.Routine{ID: in.ID, Name: in.Name, Exercises: make([]store.RoutineExercise, 0, len(in.Exercises))}
	for i, e := range in.Exercises {
		ex := store.RoutineExercise{ID: e.ID, Name: e.Name, SupersetID: e.SupersetID, Sets: make([]store.RoutineSet, 0, len(e.Sets))}
		for j, st := range e.Sets {
			if st.Reps == nil || st.WeightKg == nil {
				return r, fmt.Errorf("exercises[%d].sets[%d]: reps and weight_kg are required", i, j)
			}
			rest := int64(defaultRestSeconds)
			if st.RestSeconds != nil {
				rest = *st.RestSeconds
			}
			ex.Sets = append(ex.Sets, store.RoutineSet{
				ID: st.ID, Reps: *st.Reps, WeightKg: *st.WeightKg, Warmup: st.Warmup, RestSeconds: rest,
			})
		}
		r.Exercises = append(r.Exercises, ex)
	}
	return r, nil
}

func decodeRoutine(w http.ResponseWriter, r *http.Request) (store.Routine, bool) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	var in routineInput
	if err := dec.Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return store.Routine{}, false
	}
	routine, err := in.routine()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return store.Routine{}, false
	}
	return routine, true
}

// writeStoreError answers a failed store call.
func writeStoreError(w http.ResponseWriter, what string, err error) {
	var ve *store.ValidationError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": ve.Error()})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	default:
		log.Printf("%s: %v", what, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func listRoutinesHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routines, err := s.Routines(r.Context())
		if err != nil {
			writeStoreError(w, "list routines", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"routines": routines})
	}
}

func getRoutineHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routine, ok, err := s.Routine(r.Context(), r.PathValue("id"))
		if err != nil {
			writeStoreError(w, "get routine", err)
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, routine)
	}
}

func createRoutineHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routine, ok := decodeRoutine(w, r)
		if !ok {
			return
		}
		saved, err := s.CreateRoutine(r.Context(), routine)
		if err != nil {
			writeStoreError(w, "create routine", err)
			return
		}
		w.Header().Set("Location", "/api/routines/"+saved.ID)
		writeJSON(w, http.StatusCreated, saved)
	}
}

func replaceRoutineHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routine, ok := decodeRoutine(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if routine.ID != "" && routine.ID != id {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id in the body does not match the URL"})
			return
		}
		routine.ID = id
		saved, err := s.ReplaceRoutine(r.Context(), routine)
		if err != nil {
			writeStoreError(w, "replace routine", err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}
