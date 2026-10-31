# ORCA

**Subscription management with reliable billing, explainable invoices, portable data, and an AI agent for business workflows.**

ORCA brings the recurring revenue lifecycle into one workspace: configure products and plans, prepare quotations, manage subscriptions, generate invoices, record payments, and review receivables. Invoice calculations keep their source evidence so a business can explain totals later.

> **Project status:** Local MVP under active development. Hosting and deployment are owner-managed and have not been performed. This is not production-ready billing software; the remaining work and evidence are tracked in the [acceptance matrix](docs/requirements.md).

## What ORCA supports

| Workflow | Current implementation |
|---|---|
| Identity and tenancy | Business-admin signup, customer portal signup, password verification/reset flows, revocable sessions, CSRF protection, and business-scoped records |
| Catalog and subscriptions | Products with variants, billing plans, taxes, discounts, quotations, and subscription lifecycle workflows |
| Invoice Truth | Draft recurring invoices with decimal calculations, discount-before-tax ordering, stored snapshots, calculation explanations, engine version, and canonical hash |
| Billing and payments | Invoice confirmation/cancellation, manual payment allocation, balanced journal entries, and customer Stripe Checkout for confirmed INR invoices |
| Stripe safety | Test-mode Checkout session creation, signed webhook verification, idempotent event processing, and reconciliation records for unmatched or inconsistent successful payments |
| Finance reports | Revenue, invoicing, collections, and receivables summaries with all-time, month, quarter, and year filters |
| Copilot | Optional Groq tool calling for scoped search, reports, invoice explanations, simulations, and approval proposals for a limited set of actions |
| Data portability | Full or filtered private ZIP exports with related JSON/CSV records, invoice HTML, manifest, counts, and checksums |
| External MCP | Official Go SDK Streamable HTTP server with OAuth discovery, PKCE, scoped grants, ORCA consent, and read/propose tools |
| Workspace | Responsive React screens for billing operations, customer portal, Invoice Truth, reports, exports, and Copilot |

The API enforces authorization and tenant boundaries. The UI does not determine payable amounts or mark Stripe invoices as paid; the verified provider webhook and ORCA ledger do.

## Quick start

### Prerequisites

- Go **1.27.1** (pinned in `backend/go.mod`)
- Node.js **20+** and npm
- PostgreSQL **15+**, with a database role allowed to run migrations

### Configure and run

```sh
cp .env.example .env
# Set DATABASE_URL for your PostgreSQL installation.
npm ci --prefix frontend
make dev
```

The checked-in database URL targets the local build environment's Unix socket on port `54329`. A typical local PostgreSQL URL looks like:

```dotenv
DATABASE_URL=postgresql://orca:your-local-password@127.0.0.1:5432/orca?sslmode=disable
```

Open [http://localhost:5173](http://localhost:5173). The Vite development server and Go API bind to loopback; Vite proxies `/api`, `/mcp`, and `/.well-known` to the API. Database migrations run when the API starts. Local signup automatically verifies accounts so the core workflow can be exercised without email delivery.

Use the built-in demo preview to inspect the workspace without signing in. Demo records are local presentation data and are never sent to the API.

### Optional providers

Provider keys are optional. Keep them in the ignored root `.env`, never in source, screenshots, logs, commits, or exports.

| Variable | Purpose |
|---|---|
| `GROQ_API_KEY` | Enables Copilot model calls; `GROQ_MODEL` defaults to `openai/gpt-oss-120b` |
| `STRIPE_SECRET_KEY` | Enables customer Checkout; use a Stripe **test-mode** secret key (`sk_test_…`) |
| `STRIPE_WEBHOOK_SECRET` | Verifies signed Stripe webhook requests; point the test endpoint at `POST /api/v1/webhooks/stripe` |
| `RESEND_API_KEY` | Optional email provider configuration; delivery workflows are not complete |

Stripe Checkout is currently limited to a customer's own confirmed INR invoice with an outstanding balance. The browser return URL is informational; only a verified webhook can record a Stripe payment. Provider sandbox flows should be exercised before relying on them. See [integration contracts](docs/integrations.md) and [security rules](docs/security.md).

## Verify changes

Run from the repository root:

```sh
make verify
```

This runs Go unit tests, the race detector, `go vet`, TypeScript type checking, the current `lint` alias, and a production frontend build. The `lint` alias currently runs TypeScript checking; ESLint is not configured.

PostgreSQL integration tests use an isolated disposable schema. Example:

```sh
cd backend
ORCA_TEST_DATABASE_URL='postgresql://orca@localhost/orca?host=/tmp&port=54329&sslmode=disable' \
  go test ./internal/orca -run 'TestPostgres(HTTPBillingFlow|InvoicePeriodUniqueConstraint)$' -count=1
```

The complete verification history and browser evidence are in [docs/history.md](docs/history.md). Check the [acceptance matrix](docs/requirements.md) for work that is still partial or pending.

## Architecture

```mermaid
flowchart LR
  Browser[React and Vite workspace] -->|same-origin requests| API[Go API and domain services]
  API --> DB[(PostgreSQL)]
  API --> Groq[Groq, optional]
  API --> Stripe[Stripe Checkout and signed webhooks]
  API --> MCP[Streamable HTTP MCP]
```

- **Frontend:** React 18, TypeScript, Vite, accessible custom controls, and responsive CSS.
- **Backend:** Go, chi, pgx, Goose, PostgreSQL, exact decimal arithmetic, and the official Go MCP SDK.
- **Financial records:** invoice snapshots and hashes, transactional payment allocation, journal, audit, and outbox evidence.
- **Local routing:** Vite proxies authenticated API and MCP traffic to the Go service; authenticated API responses are not cached.

## Current limits

ORCA is an active local MVP. Durable River workers and crash recovery, complete Copilot page-action coverage and restart-safe execution, durable export jobs and cleanup, server-side filtered report APIs and report export, credit notes and separately prorated adjustments, complete edit/archive flows, production email delivery, and performance targets remain open. MCP registration/authorization state and export processing are currently process-bound. The exact status, evidence, and gaps are listed in [docs/requirements.md](docs/requirements.md).

## Project map

```text
backend/                 Go API, domain logic, PostgreSQL migrations, and tests
frontend/                React, TypeScript, Vite, and responsive workspace
docs/                    Requirements, architecture, contracts, and verification evidence
commits/                 Numbered records referenced by Conventional Commits
AGENTS.md                Canonical contributor entry point and repository rules
Makefile                 Local run and verification commands
readme.md                Product overview and local setup
```

Start with the [documentation index](docs/index.md). The [billing contract](docs/billing.md), [API contract](docs/api/openapi.yaml), [Copilot policy](docs/agents.md), [export contract](docs/exports.md), and [MCP contract](docs/mcp.md) describe current behavior and constraints. The original brief and its interpretation are captured in [the requirements source](docs/references/requirements-source.md).
