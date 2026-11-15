package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/agusespa/gym-tracker/internal/store"
)

func TestHealthEndpoints(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	// An empty frontend must not prevent the API probes from working.
	handler := New(s, fstest.MapFS{}, Options{})
	check := func(path string, code int, status string) {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != code {
			t.Fatalf("%s returned %d: %s", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unexpected probe headers: %v", response.Header())
		}
		var body map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body["status"] != status {
			t.Fatalf("unexpected probe body: %s (%v)", response.Body.String(), err)
		}
	}
	check("/api/healthz", http.StatusOK, "ok")
	check("/api/readyz", http.StatusOK, "ready")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	check("/api/readyz", http.StatusServiceUnavailable, "not_ready")
	check("/api/healthz", http.StatusOK, "ok")
}
