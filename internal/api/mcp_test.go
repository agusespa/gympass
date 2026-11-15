package api

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agusespa/gym-tracker/internal/store"
)

// authTransport adds the bearer token and, like a tunnel, a public Host.
type authTransport struct{ token string }

func (a authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Host = "gym.example.com"
	if a.token != "" {
		r.Header.Set("Authorization", "Bearer "+a.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func mcpSession(t *testing.T, url, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   url + "/mcp",
		HTTPClient: &http.Client{Transport: authTransport{token}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

// call runs a tool and decodes its structured result into out. It returns the
// error text when the tool reports an error.
func call(t *testing.T, cs *mcp.ClientSession, name string, args any, out any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		var msgs []string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				msgs = append(msgs, tc.Text)
			}
		}
		return strings.Join(msgs, " ")
	}
	if out != nil {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s: decode %s: %v", name, b, err)
		}
	}
	return ""
}

func TestMCPRequiresToken(t *testing.T) {
	srv := newRoutineServer(t, Tokens{Read: readToken, Write: writeToken})
	for _, token := range []string{"", "wrong"} {
		req, _ := http.NewRequest("POST", srv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, want 401", token, res.StatusCode)
		}
	}
	off := newRoutineServer(t, Tokens{})
	// With the API off, /mcp is just an app path that only serves GET.
	if res, err := http.Post(off.URL+"/mcp", "application/json", strings.NewReader("{}")); err != nil || res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("API off: %v %v, want 405", res.StatusCode, err)
	}
}

func TestMCPReadTokenIsReadOnly(t *testing.T) {
	srv := newRoutineServer(t, Tokens{Read: readToken, Write: writeToken})
	cs := mcpSession(t, srv.URL, readToken)
	want := []string{"get_routine", "get_workout", "list_routines", "list_workouts"}
	if got := toolNames(t, cs); !slices.Equal(got, want) {
		t.Fatalf("tools = %v, want %v", got, want)
	}
	var list routineList
	if msg := call(t, cs, "list_routines", map[string]any{}, &list); msg != "" || list.Routines == nil {
		t.Fatalf("list_routines: %q %+v", msg, list)
	}
	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_routine", Arguments: map[string]any{"name": "x"}})
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("create_routine with the read token: err = %v, want unknown tool", err)
	}
}

func TestMCPRoutineFlow(t *testing.T) {
	srv := newRoutineServer(t, Tokens{Read: readToken, Write: writeToken})
	cs := mcpSession(t, srv.URL, writeToken)
	if got := toolNames(t, cs); len(got) != 6 || !slices.Contains(got, "update_routine") {
		t.Fatalf("tools = %v", got)
	}

	var created store.Routine
	msg := call(t, cs, "create_routine", map[string]any{
		"name": "Push",
		"exercises": []any{map[string]any{
			"name": "Bench", "sets": []any{map[string]any{"reps": 8, "weight_kg": 80}},
		}},
	}, &created)
	if msg != "" || created.ID == "" || created.Exercises[0].Sets[0].RestSeconds != 120 {
		t.Fatalf("create_routine: %q %+v", msg, created)
	}

	var r store.Routine
	call(t, cs, "get_routine", map[string]any{"id": created.ID}, &r)
	r.Exercises[0].Sets[0].WeightKg = 82.5
	var saved store.Routine
	if msg := call(t, cs, "update_routine", r, &saved); msg != "" {
		t.Fatalf("update_routine: %s", msg)
	}
	if saved.Exercises[0].Sets[0].WeightKg != 82.5 || saved.Exercises[0].Sets[0].ID != r.Exercises[0].Sets[0].ID {
		t.Fatalf("saved = %+v", saved)
	}

	// Errors come back to the model as tool errors it can act on.
	cases := map[string]struct {
		tool string
		args any
		want string
	}{
		"misspelt field": {"create_routine", map[string]any{"name": "x", "exercises": []any{map[string]any{
			"name": "Bench", "sets": []any{map[string]any{"reps": 8, "weight": 80}}}}}, "weight"},
		"bad reps":        {"update_routine", map[string]any{"id": created.ID, "name": "x", "exercises": []any{map[string]any{"name": "Bench", "sets": []any{map[string]any{"reps": -1, "weight_kg": 1}}}}}, "reps"},
		"unknown routine": {"update_routine", map[string]any{"id": "missing", "name": "x"}, "no routine"},
		"unknown set id": {"update_routine", map[string]any{"id": created.ID, "name": "x", "exercises": []any{map[string]any{
			"id": saved.Exercises[0].ID, "name": "Bench", "sets": []any{map[string]any{"id": "made-up", "reps": 1, "weight_kg": 1}}}}}, "not a set"},
		"bad range":       {"list_workouts", map[string]any{"from": "2026-11-10", "to": "2026-11-01"}, "before"},
		"missing workout": {"get_workout", map[string]any{"id": "nope"}, "no finished workout"},
		"bad limit":       {"list_routines", map[string]any{"limit": 500}, "limit"},
		"bad cursor":      {"list_workouts", map[string]any{"from": "2026-11-01", "to": "2026-11-10", "cursor": "x"}, "cursor"},
	}
	for name, c := range cases {
		if msg := call(t, cs, c.tool, c.args, nil); !strings.Contains(msg, c.want) {
			t.Errorf("%s: error %q, want it to mention %q", name, msg, c.want)
		}
	}

	var week workoutList
	if msg := call(t, cs, "list_workouts", map[string]any{"from": "2026-11-09", "to": "2026-11-15"}, &week); msg != "" || week.Workouts == nil {
		t.Fatalf("list_workouts: %q %+v", msg, week)
	}
}
