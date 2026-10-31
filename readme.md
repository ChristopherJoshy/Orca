# ORCA

**Subscription management with reliable billing, explainable invoices, portable data, and an AI agent for business workflows.**

ORCA brings the recurring revenue lifecycle into one workspace: configure products and plans, prepare quotations, manage subscriptions, generate invoices, record payments, and review receivables. Each invoice keeps its calculation evidence so its total can be explained later.

> **Build status — local MVP in progress**
>
> The core Go/PostgreSQL billing API and React workspace are implemented and verified locally. This is not a production-ready billing service: durable workers, provider reconciliation, broad agent coverage, and several recovery paths remain open. Hosting and Vercel configuration are owner-managed; this build does not deploy the app.

## Contents

- [Implemented workflows](#implemented-workflows)
- [What's still in progress](#whats-still-in-progress)
- [Run ORCA locally](#run-orca-locally)
- [Verify changes](#verify-changes)
- [Architecture](#architecture)
- [Documentation and repository map](#documentation-and-repository-map)

## Implemented workflows

| Area | Current behavior |
|---|---|
| Identity and tenancy | Business-admin signup, customer portal signup by matching business ID and email, password verification/reset, revocable sessions, CSRF protection, and business-scoped API records |
| Catalog and subscriptions | Create customers, products with variants, plans, taxes, discounts, quotations, and draft subscriptions; transition quotations and subscriptions through supported lifecycle states |
| Billing and evidence | Generate due draft invoices; calculate discounts before tax with decimal arithmetic; preserve billing anchors, calculation snapshots, engine version, explanation, and canonical hash |
| Payments and reports | Confirm/cancel invoices, record idempotent manual payments, update balances, post balanced journal entries, and inspect billing and measured integrity values |
| Copilot | Groq tool calling for scoped record search, reports, invoice explanations, next-invoice simulations, and proposals for supported billing actions |
| Approval execution | Approvals bind exact inputs and a record version; ORCA rechecks access and state, executes supported actions transactionally, and stores the result |
| Exports | Full or filtered private ZIP archives with JSON/CSV data, invoice HTML, a manifest, counts, and checksums; customer archives are scoped to their portal records |
| External MCP | Official Go SDK Streamable HTTP endpoint with OAuth discovery, dynamic registration, S256 PKCE, scoped grants, ORCA consent, and read/propose tools |
| Web workspace | Responsive React screens for billing operations, Invoice Truth, reports, exports, customer portal, Copilot, and a clearly separated local demo preview |

The Go API enforces tenant scope and writes decimal-string money. Billing changes are stored with audit/outbox evidence. The PostgreSQL integration flow exercises signup through payment, approval execution, and OAuth grant persistence in an isolated schema.

## What's still in progress

- Billing runs from an API request; durable River workers, crash recovery, and outbox delivery are not implemented.
- Stripe checkout and payment reconciliation are incomplete. Email delivery and Stripe/Groq behavior have not been verified against live provider sandboxes.
- Copilot currently reads, explains, simulates, and proposes a limited set of actions. It does not cover every page action, and in-flight runs do not resume after an API restart.
- MCP client registrations and pending authorization codes are held in process memory. Export-request tools and certification with real external clients remain open.
- Export jobs run in-process. Download expiry is enforced, but periodic artifact cleanup and crash-safe job recovery remain open. Printable invoice files are HTML, not PDF.
- Credit notes, separately prorated adjustments, full edit/archive CRUD, performance measurements, and browser accessibility walkthroughs remain open.

See the [requirements and acceptance matrix](docs/requirements.md) for status and evidence. Use test data only until provider and recovery paths have been verified.

## Run ORCA locally

### Requirements

- Go **1.27.1** (pinned in `backend/go.mod`)
- Node.js **20+** and npm
- PostgreSQL **15+** with a database and role allowed to run migrations

### Configure PostgreSQL

```sh
cp .env.example .env
```

Set `DATABASE_URL` in `.env` to your local PostgreSQL database. The checked-in value targets the local build environment's Unix socket on port `54329`; for a typical local PostgreSQL install, use a URL like:

```dotenv
DATABASE_URL=postgresql://orca:your-local-password@127.0.0.1:5432/orca?sslmode=disable
```

Keep `.env` private; it is ignored by Git. `ORCA_ENV=development` enables automatic verification for local signup. Goose migrations run when the API starts. Set `GROQ_API_KEY` only if you want to exercise Copilot; ordinary billing screens work without it.

### Install and start

```sh
npm ci --prefix frontend
make dev
```

Open [http://localhost:5173](http://localhost:5173). Vite and the API bind to loopback. Vite forwards `/api`, `/mcp`, and `/.well-known` to the Go API so local login and MCP discovery use the same origin.

To run the processes separately, use two terminals:

```sh
make api   # loads the root .env; PostgreSQL is required
make web
```

You can use the explicit demo preview without signing in. Demo data is local presentation data and is not sent to the ORCA API.

## Verify changes

From the repository root:

```sh
make verify
```

This runs Go tests, the race detector, `go vet`, frontend type checking, the current `lint` alias, and a production frontend build. The `lint` target currently runs TypeScript checking; ESLint configuration has not been added.

To run PostgreSQL integration tests against an isolated schema, provide a local test database URL:

```sh
cd backend
ORCA_TEST_DATABASE_URL='postgresql://orca@localhost/orca?host=/tmp&port=54329&sslmode=disable' go test ./internal/orca -run 'TestPostgres(HTTPBillingFlow|InvoicePeriodUniqueConstraint)$' -count=1
```

The test creates and drops a uniquely named schema and migrates it from scratch.

| Command | Check |
|---|---|
| `make test` | Go tests |
| `make race` | Go race detector |
| `make vet` | Go static analysis |
| `make typecheck` | TypeScript type check |
| `make lint` | Current lint alias (TypeScript check) |
| `make build` | Optimized Vite build |

## Architecture

- **Frontend:** React 18, TypeScript, Vite, accessible custom controls, and responsive CSS.
- **API:** Go, chi, pgx, PostgreSQL, Goose migrations, exact decimal arithmetic, and the official Go MCP SDK.
- **Persistence:** relational records scoped by business, transactional billing updates, secure sessions, and private local exports.
- **Local development:** Vite proxies authenticated API and MCP routes to the Go process; authenticated API responses are not cached.

The target architecture is broader than the current implementation. River workers, Stripe checkout, durable export jobs, complete Copilot page-action coverage, and generated OpenAPI client types remain future work.

## Documentation and repository map

```text
backend/                 Go API, billing logic, migrations, and tests
frontend/                React, TypeScript, and Vite workspace
docs/                    Requirements, architecture, contracts, and verification evidence
commits/                 Numbered change records for future commits
AGENTS.md                Canonical contributor entry point
Makefile                 Local run and verification commands
readme.md                Project overview and setup
```

Start with the [documentation index](docs/index.md). The [billing contract](docs/billing.md), [security rules](docs/security.md), [API contract](docs/api/openapi.yaml), [Copilot policy](docs/agents.md), [export contract](docs/exports.md), and [MCP contract](docs/mcp.md) describe current behavior and remaining work. [Local development](docs/development.md) and [testing](docs/testing.md) cover setup and checks.

The original subscription-management brief and its interpretation are captured in [the source reference](docs/references/requirements-source.md). Product-agent policy lives in `docs/agents.md`; contributor rules live in `AGENTS.md`.
