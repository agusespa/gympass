package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agusespa/gym-tracker/internal/store"
	"github.com/agusespa/gym-tracker/internal/webhook"
)

const mcpInstructions = `Gym Tracker holds the user's logged workouts and their routines (workout templates).

Weights are kilograms. Dates are YYYY-MM-DD in the user's own time zone.

To change a routine, call get_routine, edit the result and pass the whole routine to update_routine. Keep the id of every exercise and set you keep; anything you leave out is deleted, and anything without an id is added. For progression (e.g. adding weight), first check recent performance with list_workouts.

Changes reach the user's phone on its next sync.`

// newMCPHandler serves the Model Context Protocol at /mcp. Tools that change
// routines are offered only to requests carrying the write token; the read
// token sees the read-only tools.
func newMCPHandler(s *store.Store, tokens Tokens) http.Handler {
	readOnly := newMCPServer(s, false)
	readWrite := newMCPServer(s, true)
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		got, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tokens.Write != "" && got == tokens.Write {
			return readWrite
		}
		return readOnly
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// Requests are authenticated with a bearer token, which a DNS-rebound
		// page cannot supply. The check would otherwise reject a tunnel that
		// connects to localhost with the public host name.
		DisableLocalhostProtection: true,
		MaxRequestBodyBytes:        maxBody,
	})
}

type workoutRangeArgs struct {
	From string `json:"from" jsonschema:"first date, YYYY-MM-DD, inclusive"`
	To   string `json:"to" jsonschema:"last date, YYYY-MM-DD, inclusive; at most 366 days after from"`
}

type idArgs struct {
	ID string `json:"id" jsonschema:"the id"`
}

type createRoutineArgs struct {
	Name      string          `json:"name" jsonschema:"routine name, e.g. Push A"`
	Exercises []exerciseInput `json:"exercises,omitempty" jsonschema:"exercises in order"`
}

type updateRoutineArgs struct {
	ID        string          `json:"id" jsonschema:"id of the routine to change"`
	Name      string          `json:"name" jsonschema:"routine name"`
	Exercises []exerciseInput `json:"exercises,omitempty" jsonschema:"the complete list of exercises in order; exercises left out are deleted"`
}

type routineList struct {
	Routines []store.Routine `json:"routines"`
}

func newMCPServer(s *store.Store, write bool) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "gym-tracker", Version: "1"}, &mcp.ServerOptions{
		Instructions: mcpInstructions,
	})
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: new(false)}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_workouts",
		Title:       "List workouts",
		Description: "Finished workouts dated in a range, oldest first, with each one's exercises and performed sets, plus totals for the range (working sets only).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in workoutRangeArgs) (*mcp.CallToolResult, workoutList, error) {
		out, err := workoutRange(ctx, s, in.From, in.To)
		return nil, out, toolError("list workouts", err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workout",
		Title:       "Get workout",
		Description: "One finished workout by id.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, *webhook.Workout, error) {
		snap, ok, err := s.Workout(ctx, in.ID)
		if err != nil {
			return nil, nil, toolError("get workout", err)
		}
		if !ok || snap.Deleted || snap.FinishedAt == nil {
			return nil, nil, fmt.Errorf("no finished workout with id %q", in.ID)
		}
		return nil, webhook.FromSnapshot(snap), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_routines",
		Title:       "List routines",
		Description: "Every routine with its exercises and planned sets, sorted by name.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, routineList, error) {
		routines, err := s.Routines(ctx)
		return nil, routineList{Routines: routines}, toolError("list routines", err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_routine",
		Title:       "Get routine",
		Description: "One routine by id, with the exercise and set ids needed to update it.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, store.Routine, error) {
		r, ok, err := s.Routine(ctx, in.ID)
		if err != nil {
			return nil, r, toolError("get routine", err)
		}
		if !ok {
			return nil, r, fmt.Errorf("no routine with id %q", in.ID)
		}
		return nil, r, nil
	})

	if !write {
		return server
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_routine",
		Title:       "Create routine",
		Description: "Create a new routine. Returns it with its new ids.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createRoutineArgs) (*mcp.CallToolResult, store.Routine, error) {
		r, err := routineInput{Name: in.Name, Exercises: in.Exercises}.routine()
		if err != nil {
			return nil, store.Routine{}, err
		}
		saved, err := s.CreateRoutine(ctx, r)
		return nil, saved, toolError("create routine", err)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:  "update_routine",
		Title: "Update routine",
		Description: "Replace a routine with the given name and complete exercise list. " +
			"Exercises and sets that keep their id are updated, ones without an id are added, and existing ones left out are deleted. " +
			"Call get_routine first and send back everything you want to keep. Returns the saved routine.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateRoutineArgs) (*mcp.CallToolResult, store.Routine, error) {
		r, err := routineInput(in).routine()
		if err != nil {
			return nil, store.Routine{}, err
		}
		saved, err := s.ReplaceRoutine(ctx, r)
		if errors.Is(err, store.ErrNotFound) {
			return nil, saved, fmt.Errorf("no routine with id %q", in.ID)
		}
		return nil, saved, toolError("update routine", err)
	})
	return server
}

// toolError passes validation errors to the model so it can correct its
// call, and hides internal ones.
func toolError(what string, err error) error {
	if err == nil {
		return nil
	}
	var ve *store.ValidationError
	var re rangeError
	if errors.As(err, &ve) || errors.As(err, &re) {
		return err
	}
	log.Printf("mcp %s: %v", what, err)
	return errors.New("internal error")
}
