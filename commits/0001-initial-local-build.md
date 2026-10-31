# 0001 — Initial ORCA local implementation

## Result

Established the first reviewable ORCA application: a Go/PostgreSQL API and React/Vite workspace for tenant-scoped subscription billing, invoice evidence, manual payments, reporting, Copilot proposals and approvals, private exports, and external MCP read/propose access. Added migration-backed persistence and documentation/contracts describing implementation status.

This is a local development build. It is not a production release, and no hosting or deployment was performed.

## Main areas

- `backend/`: Go API, identity and tenant checks, billing and invoice calculations, approval execution, Copilot tools, export generation, MCP server/OAuth, migrations, and tests.
- `frontend/`: React/Vite workspace, API client, billing screens, approval results, exports, and responsive styles.
- `docs/`: requirements and evidence matrix, API, architecture, billing/security/design contracts, agent/export/MCP policies, operations guidance, history, and verified mistakes.
- `readme.md`, `AGENTS.md`, `rules.txt`, `Makefile`: project entry point, contributor/product boundaries, and local development/verification commands.

## Verification

- `make verify` — passed: Go tests, race detector, `go vet`, TypeScript check, current lint alias, and production build.
- Isolated-schema PostgreSQL tests for the HTTP billing/approval/MCP-grant flow and invoice-period uniqueness — passed through migrations 001–005.
- OpenAPI YAML parsing, Go formatting check, and trailing-whitespace scan — passed.

## Known limitations

The acceptance matrix remains authoritative. Durable workers and outbox delivery, complete Stripe checkout/reconciliation, full agent page-action coverage and restart recovery, export cleanup/recovery and PDF output, live provider verification, full browser/accessibility inspection, and performance measurements remain incomplete. No Git remote was configured at the time of this local repository initialization.
