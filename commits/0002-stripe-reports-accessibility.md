# 0002 — Stripe checkout, finance reports, and responsive accessibility

## Purpose

Complete the next local ORCA feature batch: let portal customers initiate a safe Stripe test-mode payment for their own confirmed invoice, provide useful date-filtered finance summaries, and improve responsive and keyboard-accessible interaction. Keep provider and product limits explicit in the public README and requirements records.

## Changes

- Added Stripe Checkout session creation using the server-calculated outstanding invoice balance, persisted idempotency attempts, test-key restrictions, and a strict hosted-checkout URL allowlist.
- Hardened signed webhook processing for paid and delayed payment events. Duplicate and out-of-order events are idempotent; payment mismatches and unmatched successes are saved for reconciliation. Successful posting records allocation, balanced journal entries, audit, and outbox events transactionally.
- Added migrations 006 and 007 for checkout attempts and durable Stripe reconciliation/outbox state, plus checkout and webhook coverage.
- Added a portal payment action that redirects only to the validated Stripe Checkout URL; browser redirects do not mark invoices paid.
- Added Revenue, Invoicing, Collections, and Receivables report views and all-time/month/quarter/year filters. Invoices use issue dates; collections use payment dates; outstanding balances distinguish overdue from not-yet-due.
- Improved tablet/mobile layouts for workspace and settings pages, table keyboard access, and mobile navigation accessibility. Added Escape close and focus return, accessible action names, and stronger status contrast.
- Reworked `readme.md` and refreshed the API, billing, design, development, integrations, demo, testing, requirements, and mistake records.

## Verification

- `make verify` passed after implementation: `go test ./...`, `go test -race ./...`, `go vet ./...`, frontend typecheck, current lint alias (TypeScript typecheck), and Vite production build.
- Stripe checkout/webhook unit tests and isolated-schema PostgreSQL coverage through migrations 006–007 passed. Earlier isolated-schema billing/approval/MCP and invoice-period tests passed through migration 005.
- OpenAPI YAML parsed and `git diff --check` passed.
- `agent-browser` scanned all 17 application routes at 1440×900, 834×1112, and 390×844. All scans reported zero horizontal overflow and zero Axe violations. Report filters and mobile Escape/focus-return behavior were exercised.
- A direct Groq chat-completions smoke returned HTTP 200; this verified connectivity only.

## Limits

- No live Stripe sandbox checkout, email send, or full Copilot tool workflow was run.
- Full role-based end-to-end and keyboard-only acceptance, performance/load measurement, and durable worker crash recovery remain open. See `docs/requirements.md`.
- No deployment was performed.

## Files

- `backend/internal/orca/stripe_checkout.go`, `stripe_checkout_test.go`, `stripe_webhook.go`, `model.go`, `store.go`, `postgres_read.go`, `postgres_write.go`, `server.go`, `backend/cmd/api/main.go`
- `backend/migrations/006_stripe_checkout.sql`, `backend/migrations/007_stripe_reconciliation_outbox.sql`
- `frontend/src/App.tsx`, `frontend/src/pages.tsx`, `frontend/src/styles.css`
- `.env.example`, `backend/.env.example`, `docs/api/openapi.yaml`, `docs/billing.md`, `docs/demo.md`, `docs/design.md`, `docs/development.md`, `docs/history.md`, `docs/integrations.md`, `docs/mistakes.md`, `docs/requirements.md`, `docs/testing.md`, `readme.md`
