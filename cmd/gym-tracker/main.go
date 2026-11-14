package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/agusespa/gym-tracker/internal/api"
	"github.com/agusespa/gym-tracker/internal/store"
	"github.com/agusespa/gym-tracker/internal/ui"
	"github.com/agusespa/gym-tracker/internal/webhook"
)

func main() {
	dbPath := env("DB_PATH", "data/gym.db")
	port := env("PORT", "8080")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Finished workouts are POSTed to WEBHOOK_URL (e.g. an n8n Webhook node).
	if url := os.Getenv("WEBHOOK_URL"); url != "" {
		delay, err := time.ParseDuration(env("WEBHOOK_DELAY", "30s"))
		if err != nil {
			log.Fatalf("WEBHOOK_DELAY: %v", err)
		}
		s.EnableWebhookOutbox(delay)
		sender := &webhook.Sender{Store: s, URL: url, Secret: os.Getenv("WEBHOOK_SECRET")}
		go sender.Run(ctx)
		log.Printf("webhook enabled (delay %s)", delay)
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           api.New(s, ui.FS()),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	log.Printf("gym-tracker listening on :%s (db %s)", port, dbPath)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
