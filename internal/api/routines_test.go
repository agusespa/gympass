package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/agusespa/gym-tracker/internal/store"
)

const apiToken = "api-token"

func newRoutineServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "routines.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	srv := httptest.NewServer(New(s, fstest.MapFS{}, token))
	t.Cleanup(srv.Close)
	return srv
}

func send(t *testing.T, method, url, token, body string, out any) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

const pushBody = `{"name":"Push","exercises":[
  {"name":"Bench","sets":[{"reps":8,"weight_kg":80},{"reps":8,"weight_kg":80,"rest_seconds":90}]},
  {"name":"Dips","sets":[{"reps":10,"weight_kg":0,"warmup":true}]}]}`

func TestRoutineAPIToken(t *testing.T) {
	srv := newRoutineServer(t, apiToken)
	cases := []struct {
		method, path, token string
		want                int
	}{
		{"GET", "/api/routines", "", 401},
		{"GET", "/api/routines", "wrong", 401},
		{"GET", "/api/routines", apiToken, 200},
		{"GET", "/api/workouts?from=2026-11-01&to=2026-11-07", apiToken, 200},
		{"POST", "/api/routines", "", 401},
		{"POST", "/api/routines", "wrong", 401},
		{"POST", "/api/routines", apiToken, 201},
		{"PUT", "/api/routines/x", "", 401},
	}
	for _, c := range cases {
		if res := send(t, c.method, srv.URL+c.path, c.token, pushBody, nil); res.StatusCode != c.want {
			t.Errorf("%s %s with %q: status %d, want %d", c.method, c.path, c.token, res.StatusCode, c.want)
		}
	}
}

func TestRoutineCreateAndIncrement(t *testing.T) {
	srv := newRoutineServer(t, apiToken)
	var created store.Routine
	res := send(t, "POST", srv.URL+"/api/routines", apiToken, pushBody, &created)
	if res.StatusCode != http.StatusCreated || res.Header.Get("Location") != "/api/routines/"+created.ID {
		t.Fatalf("status %d, Location %q, id %q", res.StatusCode, res.Header.Get("Location"), created.ID)
	}
	bench := created.Exercises[0]
	if bench.Sets[0].RestSeconds != 120 || bench.Sets[1].RestSeconds != 90 || !created.Exercises[1].Sets[0].Warmup {
		t.Fatalf("created = %+v", created)
	}

	// Read, add 2.5 kg to every bench set, write back.
	var r store.Routine
	send(t, "GET", srv.URL+"/api/routines/"+created.ID, apiToken, "", &r)
	for i := range r.Exercises[0].Sets {
		r.Exercises[0].Sets[i].WeightKg += 2.5
	}
	body, _ := json.Marshal(r)
	var saved store.Routine
	if res := send(t, "PUT", srv.URL+"/api/routines/"+r.ID, apiToken, string(body), &saved); res.StatusCode != http.StatusOK {
		t.Fatalf("PUT status %d", res.StatusCode)
	}
	if saved.Exercises[0].Sets[0].WeightKg != 82.5 || saved.Exercises[0].Sets[0].ID != bench.Sets[0].ID {
		t.Fatalf("saved = %+v", saved.Exercises[0])
	}

	var list struct{ Routines []store.Routine }
	send(t, "GET", srv.URL+"/api/routines", apiToken, "", &list)
	if len(list.Routines) != 1 || list.Routines[0].Exercises[0].Sets[1].WeightKg != 82.5 {
		t.Fatalf("list = %+v", list)
	}
}

func TestRoutineBadRequests(t *testing.T) {
	srv := newRoutineServer(t, apiToken)
	var created store.Routine
	send(t, "POST", srv.URL+"/api/routines", apiToken, pushBody, &created)

	cases := []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/routines", `{"name":"x","exercises":[{"name":"Bench","sets":[{"reps":8,"weight":80}]}]}`, 400}, // misspelt field
		{"POST", "/api/routines", `{"name":"x","exercises":[{"name":"Bench","sets":[{"reps":8}]}]}`, 400},             // missing weight
		{"POST", "/api/routines", `{"name":""}`, 400},
		{"POST", "/api/routines", `not json`, 400},
		{"PUT", "/api/routines/" + created.ID, `{"id":"other","name":"x"}`, 400},
		{"PUT", "/api/routines/" + created.ID, `{"name":"x","exercises":[{"id":"made-up","name":"Bench"}]}`, 400},
		{"PUT", "/api/routines/missing", `{"name":"x"}`, 404},
	}
	for _, c := range cases {
		var out map[string]string
		res := send(t, c.method, srv.URL+c.path, apiToken, c.body, &out)
		if res.StatusCode != c.want || out["error"] == "" {
			t.Errorf("%s %s %s: status %d (%v), want %d with an error", c.method, c.path, c.body, res.StatusCode, out, c.want)
		}
	}
	if res := send(t, "GET", srv.URL+"/api/routines/missing", apiToken, "", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("GET missing: status %d, want 404", res.StatusCode)
	}
}

func TestRoutineListPages(t *testing.T) {
	srv := newRoutineServer(t, apiToken)
	for _, name := range []string{"Pull", "Legs", "Push"} {
		if res := send(t, http.MethodPost, srv.URL+"/api/routines", apiToken, `{"name":"`+name+`"}`, nil); res.StatusCode != http.StatusCreated {
			t.Fatalf("create %s: %d", name, res.StatusCode)
		}
	}
	var names []string
	cursor := ""
	for i := 0; i < 4; i++ {
		var p struct {
			Routines   []store.Routine
			NextCursor string `json:"next_cursor"`
		}
		if res := send(t, http.MethodGet, srv.URL+"/api/routines?limit=2&cursor="+cursor, apiToken, "", &p); res.StatusCode != http.StatusOK {
			t.Fatalf("page %d: %d", i, res.StatusCode)
		}
		for _, r := range p.Routines {
			names = append(names, r.Name)
		}
		if cursor = p.NextCursor; cursor == "" {
			break
		}
	}
	if strings.Join(names, ",") != "Legs,Pull,Push" {
		t.Fatalf("paged names = %v", names)
	}
	if res := send(t, http.MethodGet, srv.URL+"/api/routines?cursor="+encodeCursor("x", ""), apiToken, "", nil); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty-id cursor: %d", res.StatusCode)
	}
}
