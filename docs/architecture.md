# Architecture

## Intended system

- **Web:** React, TypeScript, Vite, accessible components, query/cache layer, forms with schema validation, and charts. The browser is a view/controller and never the source of authorization or financial truth.
- **API:** Go HTTP service with versioned REST under `/api/v1`, request validation, typed domain services, PostgreSQL transactions, and structured errors.
- **Database:** PostgreSQL owns tenant-scoped records, immutable financial snapshots, journals, audit trail, transactional outbox, agent runs, approvals, and export jobs. Schema changes are versioned migrations.
- **Workers:** Durable queue processing for recurring billing, catch-up, email, webhook reconciliation, export generation, and cleanup. Handlers must be idempotent and observable.
- **Providers:** Stripe test mode for checkout/webhooks, email provider behind a delivery interface, Groq-compatible tool calling behind an AI interface. Their failures do not stop manual operations or core reads.
- **MCP:** Separate HTTPS Streamable HTTP entry point that reuses typed domain services, has OAuth grants and read/propose policy, and cannot bypass ORCA approvals.

## Current implementation

The running API uses Go/chi and pgx with Goose migrations. PostgreSQL requests currently load a business-scoped state snapshot into domain maps and persist a transactional diff; this is an implementation bridge, not the planned sqlc repository layer. The official Go MCP SDK server is mounted on the API process, with OAuth registration/authorization state partly held in memory. Cancellable in-process workers catch up recurring billing and deliver invoice/quotation outbox events on configurable intervals; there is no River job queue or durable scheduler. Live provider delivery is not verified. See [development](development.md) and the [acceptance matrix](requirements.md) for the remaining gaps.

## Trust and data flow

```text
Browser -> same-origin API -> authenticated actor + business scope -> domain service
                                                       |-> PostgreSQL transaction
                                                       |   records + journal + audit + outbox
                                                       |-> worker -> provider adapter
Copilot -> policy-filtered domain tools -> proposal -> ORCA approval -> domain service
External MCP client -> OAuth grant -> read/propose tools -> proposal -> ORCA approval
```

The actor/business scope is derived from a verified session or OAuth grant. A caller-supplied business identifier is an input to a selector only after membership is checked; it never grants access. Domain services own validation and transitions. Workers reload ownership and current state before acting.

## Reliability

Implemented financial writes use a database transaction for invoice identity, snapshot, journal, audit, outbox, and related approval/payment records. Email delivery runs from persisted outbox rows through Resend with stable idempotency keys and recorded retry state. The worker remains process-bound; Resend retains keys for 24 hours, and live provider behavior is not verified. Agent runs and export generation are also process-bound, so they can lose in-flight work on restart. Complete Stripe outcome reconciliation remains open.

## API conventions

Use OpenAPI 3.1 from [api/openapi.yaml](api/openapi.yaml). Money and large identifiers are decimal/string values. Lists are paginated with opaque cursors and server-side filters. Successful writes return the stored resource. Validation failures use 422, conflicts use 409, and authorization is enforced server-side. Authenticated responses are private and uncached.

## Local topology

Run the API and web app as separate local processes with a development same-origin proxy. PostgreSQL and workers run locally; integration settings are environment variables and examples must contain no real secrets. Hosting and production provisioning are outside this project phase.
