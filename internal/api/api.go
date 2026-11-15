// Package api serves the sync endpoint, the automation API (workouts and
// routines) and the embedded frontend.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/agusespa/gym-tracker/internal/store"
)

const maxBody = 10 << 20

// syncBody is both the request and the response of POST /api/sync. Rejected
// is only in responses: rows that were not stored, with the reason. The
// other rows of the request are stored.
type syncBody struct {
	Cursor   int64                  `json:"cursor"`
	Changes  map[string][]store.Row `json:"changes"`
	Rejected []store.Rejection      `json:"rejected,omitempty"`
}

// New returns the HTTP handler. static is the built frontend (may be empty).
//
// The app has no login of its own: it is meant to be reached only through
// Cloudflare Access (see the README). Requests that change data are refused
// when a browser marks them as coming from another site, so a page you visit
// can't use your Access session to edit your data.
func New(s *store.Store, static fs.FS) http.Handler {
	mux := http.NewServeMux()
	metrics := newSyncMetrics()
	mux.Handle("POST /api/sync", metrics.instrument(syncHandler(s, metrics)))
	mux.Handle("GET /api/workouts", listWorkoutsHandler(s))
	mux.Handle("GET /api/workouts/{id}", getWorkoutHandler(s))
	mux.Handle("GET /api/routines", listRoutinesHandler(s))
	mux.Handle("GET /api/routines/{id}", getRoutineHandler(s))
	mux.Handle("POST /api/routines", createRoutineHandler(s))
	mux.Handle("PUT /api/routines/{id}", replaceRoutineHandler(s))
	mux.Handle("/mcp", newMCPHandler(s))
	mux.Handle("GET /metrics", metrics.handler())
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.Ready(ctx); err != nil {
			log.Printf("readiness: %v", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	mux.Handle("/", staticHandler(static))

	csrf := http.NewCrossOriginProtection()
	csrf.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin request refused"})
	}))
	return csrf.Handler(mux)
}

func syncHandler(s *store.Store, metrics *syncMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
		dec.UseNumber()
		var req syncBody
		if err := dec.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		cursor, changes, rejected, err := s.Sync(r.Context(), req.Cursor, req.Changes)
		if err != nil {
			var ve *store.ValidationError
			if errors.As(err, &ve) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": ve.Error()})
				return
			}
			log.Printf("sync: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if changes == nil {
			changes = map[string][]store.Row{}
		}
		if len(rejected) > 0 {
			metrics.rejected.Add(float64(len(rejected)))
			log.Printf("sync: rejected %d rows, first %s %q: %s", len(rejected), rejected[0].Table, rejected[0].ID, rejected[0].Error)
		}
		writeJSON(w, http.StatusOK, syncBody{Cursor: cursor, Changes: changes, Rejected: rejected})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// staticHandler serves the SPA: existing files as-is, everything else index.html.
func staticHandler(static fs.FS) http.Handler {
	files := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" {
			if st, err := fs.Stat(static, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "_app/immutable/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(static, "index.html")
		if err != nil {
			http.Error(w, "frontend not built", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
