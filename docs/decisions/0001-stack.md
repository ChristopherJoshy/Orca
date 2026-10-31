# ADR 0001: Go domain services with React web client

- **Date:** 2026-10-31
- **Status:** Accepted as the target architecture

## Context

ORCA handles tenant-scoped subscription state and financial records. The user plan selects Go, PostgreSQL, and a React/TypeScript frontend, plus a durable worker path and provider adapters. User explicitly excludes hosting from the current build phase.

## Decision

Use Go for the API, domain services, worker, and Streamable HTTP MCP endpoint. Use PostgreSQL as the transactional source of truth. Use React/TypeScript for the responsive web UI and same-origin versioned REST API. Keep provider integrations behind interfaces and keep credentials out of the repository. Store exact decimal strings/values and immutable invoice calculation evidence.

## Consequences

This keeps billing and access policy in one backend and lets UI, agent, workers, and MCP call shared domain services. It requires explicit migration, retry, idempotency, and provider-reconciliation design. It does not make any feature verified or any external service provisioned.
