// Package api serves the sync endpoint, the read-only workout API and the
// embedded frontend.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/agusespa/gym-tracker/internal/store"
)

const maxBody = 10 << 20

// syncBody is both the request and the response of POST /api/sync.
type syncBody struct {
	Cursor  int64                  `json:"cursor"`
	Changes map[string][]store.Row `json:"changes"`
}

// New returns the HTTP handler. static is the built frontend (may be empty).
// The read-only workout API is served only when apiToken is set, and only to
// requests that carry it as a bearer token.
func New(s *store.Store, static fs.FS, apiToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/sync", syncHandler(s))
	if apiToken != "" {
		mux.Handle("GET /api/workouts", requireToken(apiToken, listWorkoutsHandler(s)))
		mux.Handle("GET /api/workouts/{id}", requireToken(apiToken, getWorkoutHandler(s)))
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	mux.Handle("/", staticHandler(static))
	return mux
}

func syncHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
		dec.UseNumber()
		var req syncBody
		if err := dec.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		cursor, changes, err := s.Sync(r.Context(), req.Cursor, req.Changes)
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
		writeJSON(w, http.StatusOK, syncBody{Cursor: cursor, Changes: changes})
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
