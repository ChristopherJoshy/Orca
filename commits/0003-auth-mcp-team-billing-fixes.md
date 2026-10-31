# 0003 — Auth-first workspace, MCP/team settings, and billing reload fixes

**Date:** 2026-10-31  
**Type:** `feat` / `fix`  
**Summary:** make sign-in the initial product state, remove demo persistence, expose Copilot and MCP/team setup, and fix two live PostgreSQL workspace-loading failures.

## Behavior

- Replaced demo-mode startup with required PostgreSQL and an auth-first sign-in/workspace/customer/reset/verification experience.
- Fixed state cloning so private Argon2 password hashes survive unrelated updates and newly created accounts can log in again.
- Fixed exact decimal-string parsing for invoice snapshot unit prices and use of backend subscription totals.
- Added a staff-only Billing Control Tower run action, Copilot primary navigation, missing inline icons, team-member setup guidance, and External MCP OAuth discovery/setup/scopes/grant revocation in Settings.
- Removed non-OAuth MCP connection metadata mutation routes; only authorized OAuth grants are shown as connections.

## Main files

- `frontend/src/App.tsx`, `frontend/src/useWorkspace.ts`, `frontend/src/model.ts`, `frontend/src/pages.tsx`, `frontend/src/icons.tsx`, `frontend/src/styles.css`
- `backend/cmd/api/main.go`, `backend/internal/orca/store.go`, `backend/internal/orca/store_test.go`, `backend/internal/orca/mcp_server.go`, `backend/internal/orca/server.go`
- `docs/api/openapi.yaml`, `docs/mcp.md`, `docs/demo.md`, `docs/index.md`, `docs/history.md`, `docs/mistakes.md`, `AGENTS.md`, `readme.md`

## Verification

- `make verify` — passed: Go tests, race detector, `go vet`, frontend typecheck, lint alias, and production build.
- `git diff --check` — passed.
- Browser: new account signup → sign-out → password login passed with PostgreSQL; customer/plan/subscription creation and recurring draft invoice generation passed; invoice evidence showed ₹1,250.00 subtotal + ₹225.00 tax = ₹1,475.00.
- Browser: Copilot screen opened on mobile; Axe scan at 390×844 reported zero violations.
- Earlier account-backed run confirmed invoice and recorded full payment successfully.

## Limitations

- Groq prompt submission was rejected by automatic review because it could send business billing records to an external provider; the full Copilot provider workflow remains unverified.
- Email, live Stripe test checkout, external MCP client compatibility, performance/load, and comprehensive role-based browser acceptance remain open. No deployment was performed.
