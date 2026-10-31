package orca

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

const DefaultEmailOutboxWorkerInterval = time.Minute

// StartEmailOutboxWorker drains customer-facing mail requests without holding
// database transactions open while the provider performs network work.
func StartEmailOutboxWorker(ctx context.Context, api *API, interval time.Duration, logger *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = DefaultEmailOutboxWorkerInterval
	}
	if api == nil {
		close(done)
		logger.Error("email outbox worker not started: API is required")
		return done
	}
	if strings.TrimSpace(api.Config.ResendAPIKey) == "" || strings.TrimSpace(api.Config.EmailFrom) == "" {
		close(done)
		logger.Info("email outbox delivery is disabled; configure RESEND_API_KEY and EMAIL_FROM")
		return done
	}
	go func() {
		defer close(done)
		run := func() {
			delivered, err := api.ProcessEmailOutbox(ctx)
			if err != nil {
				if ctx.Err() == nil {
					logger.Error("email outbox pass completed with failures", "error", err, "delivered", delivered)
				}
				return
			}
			if delivered > 0 {
				logger.Info("customer email outbox events delivered", "delivered", delivered)
			}
		}
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
	return done
}
