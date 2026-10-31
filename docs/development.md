# Local development

ORCA runs as a local React/Vite app, a Go API, and PostgreSQL. Vite forwards `/api`, `/mcp`, and `/.well-known` to the API so browser API calls and MCP OAuth discovery share the local origin. Development servers bind to loopback. Deployment and hosting are owner-managed and are outside this build.

## Prerequisites

- Go 1.27.1 or the version declared in `backend/go.mod`.
- Node.js 20 or newer and npm.
- PostgreSQL 15 or newer, with a database role and database that can run migrations.
- Optional provider credentials for email, Stripe, and Groq. Core local workflows do not require them.

## Configure the local environment

From the repository root:

```sh
cp .env.example .env
```

Edit `DATABASE_URL` to match the local PostgreSQL role, database, and connection method. The checked-in example uses a local Unix socket on port 54329 for the build environment; a typical local PostgreSQL installation may instead use `postgresql://orca:password@127.0.0.1:5432/orca?sslmode=disable`. Set `ORCA_ALLOWED_ORIGIN=http://localhost:5173` and `ORCA_COOKIE_SECURE=false` only for local development. Keep `.env` private and untracked. Go applies `backend/migrations/` automatically when the API starts.

## Install and run

```sh
cd frontend && npm ci
cd ..
make dev
```

Open `http://localhost:5173`. The API listens at `http://127.0.0.1:8080`; Vite proxies the API and MCP routes to it. In local development, signup automatically verifies the new account so core workflows can be exercised without an email provider.

To run either process separately:

```sh
make api
make web
```

## Verification commands

```sh
make test       # Go unit and database integration tests
make race       # Go race detector
make vet        # Go static analysis
make typecheck  # TypeScript type checking
make lint       # currently the TypeScript checker; no ESLint config is installed
make build      # optimized Vite build
make verify     # all commands above
```

The local Go build cache is written to `/tmp/orca-go-cache` by default. Database-backed tests use a disposable test schema and do not require a deployment account.

## Implementation boundaries

The code currently uses Go/chi, pgx, Goose, shopspring/decimal, React, TypeScript, and Vite. Implemented additions include a bounded Groq tool loop, approval execution for a limited billing action set, official Go SDK Streamable HTTP MCP with OAuth/PKCE, and repeatable-read filtered ZIP exports. The broader target still needs River workers, generated OpenAPI client types, TanStack, Tailwind, Stripe checkout/reconciliation, durable agent and export processing, provider delivery, and complete agent page-action coverage. See the live [requirements matrix](requirements.md) and [change history](history.md).

## Change workflow

1. Read the owning product contract before changing its behavior.
2. Implement authorization and financial rules in the Go service layer; frontend state is not authoritative.
3. Run checks relevant to the change, then `make verify` before delivery.
4. Keep OpenAPI and product documentation aligned with implemented behavior. Record dated verification and limits in `docs/history.md`.
5. For any future commit, write the next detailed record under `commits/` and cite it from the Conventional Commit body. Do not deploy as part of this local build phase.
