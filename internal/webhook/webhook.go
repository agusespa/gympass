// Package webhook delivers finished workouts from the store's outbox to an
// HTTP endpoint (e.g. an n8n Webhook node).
//
// Every send carries the whole workout, so the receiver can upsert by
// workout.id: a workout edited after it was finished is sent again, and a
// deleted one is sent as "workout.deleted".
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/agusespa/gym-tracker/internal/store"
)

const (
	EventSaved   = "workout.saved"
	EventDeleted = "workout.deleted"

	batchSize  = 20
	minBackoff = 30 * time.Second
	maxBackoff = time.Hour
)

type Sender struct {
	Store  *store.Store
	URL    string
	Secret string // sent as "Authorization: Bearer <Secret>" when set
	Client *http.Client
	Poll   time.Duration
}

// Payload is the JSON body of every request.
type Payload struct {
	Event   string   `json:"event"`
	SentAt  string   `json:"sent_at"`
	Workout *Workout `json:"workout"`
}

type Workout struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Date       string `json:"date,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	// DurationSeconds is finished_at - started_at.
	DurationSeconds int64      `json:"duration_seconds,omitempty"`
	Exercises       []Exercise `json:"exercises,omitempty"`
	Totals          *Totals    `json:"totals,omitempty"`
}

type Exercise struct {
	Name       string  `json:"name"`
	SupersetID *string `json:"superset_id"`
	Sets       []Set   `json:"sets"`
}

type Set struct {
	Reps        int64   `json:"reps"`
	WeightKg    float64 `json:"weight_kg"`
	Warmup      bool    `json:"warmup"`
	RestSeconds int64   `json:"rest_seconds"`
	// DurationSeconds is how long a timed set (e.g. a dead hang) lasted;
	// omitted for sets counted in reps.
	DurationSeconds int64 `json:"duration_seconds,omitempty"`
}

// Totals count working sets only; warm-ups are listed but not added up.
type Totals struct {
	Exercises   int   `json:"exercises"`
	WorkingSets int   `json:"working_sets"`
	Reps        int64 `json:"reps"`
}

// Run delivers due sends every Poll until ctx is cancelled.
func (s *Sender) Run(ctx context.Context) {
	poll := s.Poll
	if poll <= 0 {
		poll = 5 * time.Second
	}
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		if err := s.Flush(ctx); err != nil && ctx.Err() == nil {
			log.Printf("webhook: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Flush delivers every send that is due now.
func (s *Sender) Flush(ctx context.Context) error {
	for {
		due, err := s.Store.DueWebhooks(ctx, batchSize)
		if err != nil {
			return err
		}
		for _, p := range due {
			if err := s.deliver(ctx, p); err != nil {
				return err
			}
		}
		if len(due) < batchSize {
			return nil
		}
	}
}

// deliver sends one workout. Delivery failures are rescheduled and logged;
// only store errors are returned.
func (s *Sender) deliver(ctx context.Context, p store.PendingWebhook) error {
	w, ok, err := s.Store.Workout(ctx, p.WorkoutID)
	if err != nil {
		return err
	}
	payload, send := buildPayload(w, ok, p.Sent)
	if !send {
		return s.Store.SkipWebhook(ctx, p)
	}
	if err := s.post(ctx, payload); err != nil {
		wait := backoff(p.Attempts)
		log.Printf("webhook: %s %s failed (attempt %d), retrying in %s: %v", payload.Event, p.WorkoutID, p.Attempts+1, wait, err)
		return s.Store.RetryWebhook(ctx, p, wait)
	}
	return s.Store.MarkWebhookSent(ctx, p)
}

// buildPayload decides what to send for a workout's current state. A workout
// deleted (or never finished) before any version was delivered sends nothing.
func buildPayload(w store.WorkoutSnapshot, found, sentBefore bool) (Payload, bool) {
	now := time.Now().UTC().Format(time.RFC3339)
	if !found || w.Deleted || w.FinishedAt == nil {
		if !sentBefore {
			return Payload{}, false
		}
		return Payload{Event: EventDeleted, SentAt: now, Workout: &Workout{ID: w.ID}}, true
	}
	return Payload{Event: EventSaved, SentAt: now, Workout: FromSnapshot(w)}, true
}

// FromSnapshot converts a finished workout to its JSON form, shared by the
// webhook and the read API. Exercises with no performed sets are left out.
// w.FinishedAt must be set.
func FromSnapshot(w store.WorkoutSnapshot) *Workout {
	out := &Workout{
		ID:         w.ID,
		Name:       w.Name,
		Date:       w.Date,
		FinishedAt: time.UnixMilli(*w.FinishedAt).UTC().Format(time.RFC3339),
		Exercises:  make([]Exercise, 0, len(w.Exercises)),
		Totals:     &Totals{},
	}
	out.StartedAt = time.UnixMilli(w.StartedAt).UTC().Format(time.RFC3339)
	if d := *w.FinishedAt - w.StartedAt; d > 0 {
		out.DurationSeconds = d / 1000
	}
	for _, e := range w.Exercises {
		// Only performed (checked) sets are in the snapshot; an exercise with none wasn't done.
		if len(e.Sets) == 0 {
			continue
		}
		ex := Exercise{Name: e.Name, SupersetID: e.SupersetID, Sets: make([]Set, 0, len(e.Sets))}
		for _, st := range e.Sets {
			ex.Sets = append(ex.Sets, Set(st))
			if !st.Warmup {
				out.Totals.WorkingSets++
				out.Totals.Reps += st.Reps
			}
		}
		out.Exercises = append(out.Exercises, ex)
	}
	out.Totals.Exercises = len(out.Exercises)
	return out
}

func (s *Sender) post(ctx context.Context, p Payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "gym-tracker")
	if s.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+s.Secret)
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("status %d: %s", res.StatusCode, bytes.TrimSpace(msg))
	}
	_, _ = io.Copy(io.Discard, res.Body)
	return nil
}

// backoff doubles from minBackoff up to maxBackoff.
func backoff(attempts int) time.Duration {
	d := minBackoff
	for i := 0; i < attempts && d < maxBackoff; i++ {
		d *= 2
	}
	return min(d, maxBackoff)
}
