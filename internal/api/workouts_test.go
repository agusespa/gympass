package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/agusespa/gym-tracker/internal/store"
)

const testToken = "test-token"

func newReadServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "read.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Two finished workouts in the week, one running, one outside it.
	w := func(id, date string, started, finished any) store.Row {
		return store.Row{"id": id, "updated_at": int64(100), "deleted": false, "name": id,
			"date": date, "started_at": started, "finished_at": finished}
	}
	ex := func(id, workout string) store.Row {
		return store.Row{"id": id, "updated_at": int64(100), "deleted": false,
			"workout_id": workout, "name": "Bench", "position": 1.0}
	}
	set := func(id, exercise string, reps int64, kg float64, warmup bool) store.Row {
		return store.Row{"id": id, "updated_at": int64(100), "deleted": false, "workout_exercise_id": exercise,
			"position": 1.0, "reps": reps, "weight_kg": kg, "warmup": warmup, "done": true}
	}
	_, _, err = s.Sync(context.Background(), 0, map[string][]store.Row{
		"workouts": {
			w("push", "2026-11-10", int64(1_000_000), int64(4_600_000)), // 3600 s
			w("pull", "2026-11-12", nil, int64(9_000_000)),
			w("running", "2026-11-13", int64(1_000_000), nil),
			w("old", "2026-11-01", nil, int64(9_000_000)),
		},
		"workout_exercises": {ex("e1", "push"), ex("e2", "pull")},
		"sets": {
			set("s1", "e1", 5, 40, true),
			set("s2", "e1", 8, 80, false),
			set("s3", "e2", 10, 50.25, false),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(s, fstest.MapFS{}, token))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url, auth string, out any) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return res
}

func TestReadAPIRequiresToken(t *testing.T) {
	srv := newReadServer(t, testToken)
	for _, auth := range []string{"", "Bearer wrong", testToken, "Basic " + testToken} {
		res := get(t, srv.URL+"/api/workouts?from=2026-11-10&to=2026-11-16", auth, nil)
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("auth %q: status %d, want 401", auth, res.StatusCode)
		}
		if res.Header.Get("WWW-Authenticate") == "" {
			t.Errorf("auth %q: missing WWW-Authenticate", auth)
		}
	}
	if res := get(t, srv.URL+"/api/workouts/push", "", nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("get by id without token: status %d, want 401", res.StatusCode)
	}
}

func TestReadAPIDisabledWithoutToken(t *testing.T) {
	srv := newReadServer(t, "")
	for _, auth := range []string{"", "Bearer "} {
		res := get(t, srv.URL+"/api/workouts?from=2026-11-10&to=2026-11-16", auth, nil)
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("auth %q: status %d, want 404", auth, res.StatusCode)
		}
	}
}

func TestListWorkouts(t *testing.T) {
	srv := newReadServer(t, testToken)
	var out struct {
		From, To string
		Totals   map[string]float64
		Workouts []struct {
			ID              string
			DurationSeconds int64 `json:"duration_seconds"`
		}
	}
	res := get(t, srv.URL+"/api/workouts?from=2026-11-10&to=2026-11-16", "Bearer "+testToken, &out)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", res.Header.Get("Cache-Control"))
	}
	if out.From != "2026-11-10" || out.To != "2026-11-16" {
		t.Errorf("range = %s..%s", out.From, out.To)
	}
	if len(out.Workouts) != 2 || out.Workouts[0].ID != "push" || out.Workouts[1].ID != "pull" {
		t.Fatalf("workouts = %+v, want push then pull", out.Workouts)
	}
	want := map[string]float64{
		"workouts": 2, "working_sets": 2, "reps": 18,
		"volume_kg": 8*80 + 10*50.25, "duration_seconds": 3600,
	}
	for k, v := range want {
		if out.Totals[k] != v {
			t.Errorf("totals[%s] = %v, want %v", k, out.Totals[k], v)
		}
	}
}

func TestListWorkoutsEmptyIsArray(t *testing.T) {
	srv := newReadServer(t, testToken)
	var out map[string]json.RawMessage
	get(t, srv.URL+"/api/workouts?from=2027-01-01&to=2027-01-07", "Bearer "+testToken, &out)
	if string(out["workouts"]) != "[]" {
		t.Errorf("workouts = %s, want []", out["workouts"])
	}
}

func TestListWorkoutsBadRange(t *testing.T) {
	srv := newReadServer(t, testToken)
	for _, q := range []string{
		"", "from=2026-11-10", "from=2026-11-10&to=nope", "from=2026-11-16&to=2026-11-10",
		"from=2025-01-01&to=2026-01-02",
	} {
		res := get(t, srv.URL+"/api/workouts?"+q, "Bearer "+testToken, nil)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400", q, res.StatusCode)
		}
	}
	// A full year (366 days, as in a leap year) is allowed.
	if res := get(t, srv.URL+"/api/workouts?from=2028-01-01&to=2028-12-31", "Bearer "+testToken, nil); res.StatusCode != http.StatusOK {
		t.Errorf("leap year: status %d, want 200", res.StatusCode)
	}
}

func TestGetWorkout(t *testing.T) {
	srv := newReadServer(t, testToken)
	var w struct {
		ID        string
		Exercises []struct{ Sets []any }
	}
	res := get(t, srv.URL+"/api/workouts/push", "Bearer "+testToken, &w)
	if res.StatusCode != http.StatusOK || w.ID != "push" || len(w.Exercises) != 1 || len(w.Exercises[0].Sets) != 2 {
		t.Fatalf("status %d, workout %+v", res.StatusCode, w)
	}
	for _, id := range []string{"running", "missing"} {
		if res := get(t, srv.URL+"/api/workouts/"+id, "Bearer "+testToken, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", id, res.StatusCode)
		}
	}
}
