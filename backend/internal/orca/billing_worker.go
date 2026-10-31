package orca

import (
	"context"
	"log/slog"
	"time"
)

const DefaultBillingWorkerInterval = time.Minute

// StartBillingWorker starts one immediate catch-up pass followed by periodic
// passes. The returned channel closes after cancellation and all current work
// has stopped; the caller owns ctx cancellation and may wait on the channel
// during graceful shutdown.
func StartBillingWorker(ctx context.Context, store *Store, interval time.Duration, logger *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = DefaultBillingWorkerInterval
	}
	go func() {
		defer close(done)
		if store == nil {
			logger.Error("recurring billing worker stopped: store is required")
			return
		}
		run := func() {
			businessIDs, err := store.BusinessIDs(ctx)
			if err != nil {
				if ctx.Err() == nil {
					logger.Error("recurring billing worker could not list businesses", "error", err)
				}
				return
			}
			cutoff := time.Now().UTC()
			for _, businessID := range businessIDs {
				if ctx.Err() != nil {
					return
				}
				actor := Actor{BusinessID: businessID, Role: "system"}
				created, err := runBillingForBusiness(ctx, store, businessID, actor, cutoff)
				if err != nil {
					if ctx.Err() == nil {
						logger.Error("recurring billing failed for business", "business_id", businessID, "error", err)
					}
					continue
				}
				if created > 0 {
					logger.Info("recurring invoices generated", "business_id", businessID, "created", created, "as_of", cutoff)
				}
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
