package api

import (
	"net/http"
	"strings"
	"testing"
)

// Browsers mark where a request comes from; writes from another site must be
// refused so a page you visit can't ride your Cloudflare Access session.
func TestCrossOriginWritesRefused(t *testing.T) {
	srv := newRoutineServer(t)
	writes := []struct{ method, path, body string }{
		{"POST", "/api/sync", `{"cursor":0,"changes":{}}`},
		{"POST", "/api/routines", pushBody},
		{"PUT", "/api/routines/x", `{"name":"x"}`},
		{"POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`},
	}
	do := func(method, path, body string, header map[string]string) int {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		for k, v := range header {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}

	for _, w := range writes {
		for name, h := range map[string]map[string]string{
			"cross-site":     {"Sec-Fetch-Site": "cross-site"},
			"foreign origin": {"Origin": "https://evil.example"},
		} {
			if got := do(w.method, w.path, w.body, h); got != http.StatusForbidden {
				t.Errorf("%s %s from %s: status %d, want 403", w.method, w.path, name, got)
			}
		}
		// The app itself (same origin) and non-browser clients (curl, n8n,
		// MCP clients send no such headers) get through.
		for name, h := range map[string]map[string]string{
			"same-origin": {"Sec-Fetch-Site": "same-origin"},
			"no browser":  nil,
		} {
			if got := do(w.method, w.path, w.body, h); got == http.StatusForbidden {
				t.Errorf("%s %s from %s: refused", w.method, w.path, name)
			}
		}
	}

	if got := do("GET", "/api/routines", "", map[string]string{"Sec-Fetch-Site": "cross-site"}); got != http.StatusOK {
		t.Errorf("cross-site GET: status %d, want 200 (reads are not refused)", got)
	}
}
