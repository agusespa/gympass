package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/agusespa/gym-tracker/internal/store"
)

func TestSyncMetricsIncludeSuccessValidationAndDatabaseFailure(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	handler := New(s, fstest.MapFS{}, Options{})
	request := func(body string, status int) {
		t.Helper()
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body)))
		if res.Code != status {
			t.Fatalf("sync returned %d: %s", res.Code, res.Body.String())
		}
	}
	request(`{"cursor":0}`, http.StatusOK)
	request(`not JSON`, http.StatusBadRequest)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	request(`{"cursor":0}`, http.StatusInternalServerError)

	// Probes and scrapes must not inflate the sync request counters.
	for _, path := range []string{"/api/healthz", "/metrics", "/metrics"} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusOK {
			t.Fatalf("%s returned %d", path, res.Code)
		}
		if path == "/metrics" {
			if res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("metrics must not be cached")
			}
			for _, sample := range []string{
				`gym_tracker_sync_requests_total{code="200"} 1`,
				`gym_tracker_sync_requests_total{code="400"} 1`,
				`gym_tracker_sync_requests_total{code="500"} 1`,
				`gym_tracker_sync_request_duration_seconds_count 3`,
			} {
				if !strings.Contains(res.Body.String(), sample) {
					t.Errorf("missing sample %s", sample)
				}
			}
		}
	}
}
