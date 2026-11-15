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

	"github.com/agusespa/gym-tracker/internal/accessjwt"
	"github.com/agusespa/gym-tracker/internal/api"
	"github.com/agusespa/gym-tracker/internal/store"
	"github.com/agusespa/gym-tracker/internal/ui"
	"github.com/agusespa/gym-tracker/internal/webhook"
)

func main() {
	if len(os.Args) > 1 {
		if len(os.Args) != 2 || os.Args[1] != "healthcheck" {
			log.Print("usage: gym-tracker [healthcheck]")
			os.Exit(2)
		}
		if err := checkReadiness(context.Background(), "http://127.0.0.1:"+env("PORT", "8080")+"/api/readyz"); err != nil {
			log.Printf("healthcheck: %v", err)
			os.Exit(1)
		}
		return
	}

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

	if os.Getenv("API_TOKEN") != "" {
		log.Print("API_TOKEN is no longer used: the API relies on Cloudflare Access like the app; remove it")
	}

	// With both set, /mcp only accepts requests that Cloudflare Access let
	// through, which Access's Managed OAuth (MCP clients such as Claude's
	// apps logging in with your Access account) requires.
	var opts api.Options
	team, aud := os.Getenv("CF_ACCESS_TEAM_DOMAIN"), os.Getenv("CF_ACCESS_AUD")
	switch {
	case team != "" && aud != "":
		opts.MCPGuard = accessjwt.New(team, aud).Require
		log.Printf("/mcp requires a Cloudflare Access token (team %s)", team)
	case team != "" || aud != "":
		log.Fatal("set both CF_ACCESS_TEAM_DOMAIN and CF_ACCESS_AUD, or neither")
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           api.New(s, ui.FS(), opts),
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
