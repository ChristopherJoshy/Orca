package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"example.com/orca/backend/internal/orca"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	environment := env("ORCA_ENV", "development")
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	statePath := ""
	if databaseURL == "" {
		statePath = strings.TrimSpace(os.Getenv("ORCA_DEMO_STATE"))
		if statePath == "" {
			slog.Error("DATABASE_URL is required; set ORCA_DEMO_STATE explicitly only for local demo mode")
			os.Exit(1)
		}
		slog.Warn("running explicit demo-only file store; PostgreSQL is the default workspace source of truth")
	}
	store, err := orca.OpenStore(ctx, databaseURL, statePath)
	if err != nil {
		slog.Error("open ORCA store", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	secure := env("ORCA_COOKIE_SECURE", "true") != "false"
	if environment == "development" && os.Getenv("ORCA_COOKIE_SECURE") == "" {
		secure = false
	}
	api := orca.NewAPI(store, orca.ServerConfig{
		CookieSecure:        secure,
		AllowedOrigin:       os.Getenv("ORCA_ALLOWED_ORIGIN"),
		Env:                 environment,
		ResendAPIKey:        os.Getenv("RESEND_API_KEY"),
		EmailFrom:           env("EMAIL_FROM", "ORCA <noreply@example.com>"),
		PublicURL:           env("ORCA_PUBLIC_URL", "http://localhost:5173"),
		GroqAPIKey:          os.Getenv("GROQ_API_KEY"),
		GroqModel:           env("GROQ_MODEL", "openai/gpt-oss-120b"),
		StripeAPIKey:        os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	})
	server := &http.Server{Addr: env("ORCA_ADDR", ":8080"), Handler: api, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("ORCA API listening", "addr", server.Addr, "postgres", store.IsPostgres())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("ORCA API stopped", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
