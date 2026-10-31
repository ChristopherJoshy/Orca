# 0009 — Automatic recurring billing catch-up

## Result

- Extracted recurring invoice generation into a deterministic, tenant-scoped service shared by the staff endpoint and a background worker.
- The API starts a catch-up pass on startup and repeats at `ORCA_BILLING_INTERVAL` (default `1m`). The worker enumerates only business IDs, runs one transaction per business, logs and isolates tenant errors, and stops cleanly with the API context.
- Existing advisory transaction locks serialize worker/manual runs for a business. Persisted cursors and the unique `(business_id, subscription_id, period_start)` constraint preserve retry and downtime catch-up behavior.
- Added an indexed in-memory period lookup so catch-up does not rescan all invoices for each generated period.
- Updated runtime examples, contributor guidance, billing/architecture/deployment documentation, README, and acceptance evidence.

## Main files

- `backend/internal/orca/billing_handlers.go`
- `backend/internal/orca/billing_worker.go`
- `backend/internal/orca/billing_worker_test.go`
- `backend/internal/orca/billing_catchup_test.go`
- `backend/internal/orca/store.go`
- `backend/cmd/api/main.go`
- `.env.example`, `backend/.env.example`, `AGENTS.md`
- `docs/billing.md`, `docs/architecture.md`, `docs/development.md`, `docs/deployment.md`, `docs/requirements.md`, `docs/history.md`, `readme.md`

## Verification

- Focused catch-up and worker tests passed: month-end anchor restoration, missed-period recovery, repeat-run idempotency, future starts, worker cancellation, and per-business failure isolation.
- `make verify` passed: Go tests, race detector, `go vet`, frontend typecheck, lint alias, and production build.
- `git diff --check` passed.

## Known limits

- Worker tests currently use the in-memory store; clean-schema PostgreSQL worker and sustained multi-worker/1,000-subscription load acceptance remain open.
- The worker creates invoices and outbox records; email/provider delivery is not implemented.
- The scheduler is in-process and requires a running API replica; it is not a durable external job queue.
