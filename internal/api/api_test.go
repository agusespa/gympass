package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/agusespa/gym-tracker/internal/store"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	static := fstest.MapFS{
		"index.html":            {Data: []byte("<html>app</html>")},
		"manifest.webmanifest":  {Data: []byte("{}")},
		"_app/immutable/app.js": {Data: []byte("js")},
	}
	srv := httptest.NewServer(New(s, static, Options{}))
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, url, body string) (*http.Response, map[string]any) {
	t.Helper()
	res, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return res, out
}

func TestSyncEndpoint(t *testing.T) {
	srv := newServer(t)
	body := `{"cursor":0,"changes":{"workouts":[{"id":"w1","updated_at":1731571200000,"deleted":false,"name":"Push","date":"2026-11-14","started_at":null,"finished_at":null}]}}`
	res, out := post(t, srv.URL+"/api/sync", body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %v", res.StatusCode, out)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type %q", ct)
	}
	if out["cursor"] != float64(1) {
		t.Fatalf("cursor %v", out["cursor"])
	}
	ws := out["changes"].(map[string]any)["workouts"].([]any)
	w := ws[0].(map[string]any)
	if w["updated_at"] != float64(1731571200000) || w["name"] != "Push" {
		t.Fatalf("workout %v", w)
	}

	// Empty sync returns an empty object, not null.
	_, out = post(t, srv.URL+"/api/sync", `{"cursor":1}`)
	if ch, ok := out["changes"].(map[string]any); !ok || len(ch) != 0 {
		t.Fatalf("changes %v", out["changes"])
	}
}

func TestSyncReportsRejectedRows(t *testing.T) {
	srv := newServer(t)
	body := `{"cursor":0,"changes":{"workouts":[{"id":"w1"},{"id":"w2","updated_at":1,"deleted":false,"name":"Push","date":"2026-11-14","started_at":null,"finished_at":null}]}}`
	res, out := post(t, srv.URL+"/api/sync", body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %v", res.StatusCode, out)
	}
	rej := out["rejected"].([]any)
	if len(rej) != 1 || rej[0].(map[string]any)["id"] != "w1" || rej[0].(map[string]any)["table"] != "workouts" {
		t.Fatalf("rejected %v", rej)
	}
	if ws := out["changes"].(map[string]any)["workouts"].([]any); len(ws) != 1 {
		t.Fatalf("workouts %v", ws)
	}
}

func TestSyncRejectsBadInput(t *testing.T) {
	srv := newServer(t)
	for _, body := range []string{
		`not json`,
		`{"cursor":-1}`,
		`{"cursor":0,"changes":{"workouts":{}}}`,
	} {
		res, out := post(t, srv.URL+"/api/sync", body)
		if res.StatusCode != http.StatusBadRequest || out["error"] == nil {
			t.Errorf("%s: status %d %v", body, res.StatusCode, out)
		}
	}
}

func TestUnknownAPIIs404JSON(t *testing.T) {
	srv := newServer(t)
	res, err := http.Get(srv.URL + "/api/sync")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/sync status %d", res.StatusCode)
	}
	res, err = http.Get(srv.URL + "/api/nope")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound || res.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("status %d ct %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
}

func TestStatic(t *testing.T) {
	srv := newServer(t)
	cases := []struct {
		path, body, cache string
	}{
		{"/", "<html>app</html>", "no-cache"},
		{"/workouts/abc", "<html>app</html>", "no-cache"},
		{"/manifest.webmanifest", "{}", "no-cache"},
		{"/_app/immutable/app.js", "js", "public, max-age=31536000, immutable"},
	}
	for _, c := range cases {
		res, err := http.Get(srv.URL + c.path)
		if err != nil {
			t.Fatal(err)
		}
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK || buf.String() != c.body || res.Header.Get("Cache-Control") != c.cache {
			t.Errorf("%s: status %d body %q cache %q", c.path, res.StatusCode, buf.String(), res.Header.Get("Cache-Control"))
		}
	}
	res, err := http.Post(srv.URL+"/", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST / status %d", res.StatusCode)
	}
}

func TestStaticNotBuilt(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	srv := httptest.NewServer(New(s, fstest.MapFS{}, Options{}))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", res.StatusCode)
	}
}
