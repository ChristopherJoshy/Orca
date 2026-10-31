# 0007 — Customer quotation acceptance and subscription handoff

## Result

- Customers can review offers addressed to their own portal account and accept a sent offer while it remains valid.
- The API checks customer ownership, sent status, and the expiry date in the business timezone. Expired and foreign offers are rejected without changing quotation state.
- Staff conversion copies the accepted quotation's customer, plan, quantities, and all quoted lines into a draft subscription, then marks the quotation converted as part of the same store update.
- Subscription response totals now include every stored line, so plan-plus-product offers retain the correct amount in the workspace.
- Updated the customer portal, OpenAPI description, README workflow summary, and acceptance matrix.

## Main files

- `backend/internal/orca/crud_handlers.go`
- `backend/internal/orca/server.go`
- `backend/internal/orca/quotation_acceptance_test.go`
- `frontend/src/App.tsx`
- `frontend/src/pages.tsx`
- `docs/api/openapi.yaml`
- `docs/requirements.md`
- `docs/history.md`
- `readme.md`

## Verification

- Focused customer acceptance and conversion tests passed, including expired and foreign-customer rejection.
- `make verify` passed: Go tests, race tests, `go vet`, frontend typecheck, lint, and production build.
- The account-backed browser workflow accepted an offer as its customer and converted it as staff; this was completed in the prior browser session. Browser automation service was unavailable for a fresh run in the current session.
- `git diff --check` passed.

## Known limits

- Quotation email requests are written to the outbox; live provider delivery was not verified.
- Template edit/delete and broader quotation lifecycle acceptance remain open.
