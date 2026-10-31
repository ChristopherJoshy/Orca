# 0006 — Reusable quotation templates and cleaner auth screens

**Commit:** `feat: add reusable quotation templates`

## Result

- Added tenant-scoped quotation template creation and listing. Staff can save a plan line plus an optional product line with quantities, copied decimal-string prices and taxes, and a validity period.
- Added the required plan reference migration and composite business/plan foreign key.
- The Quotations page now lists templates, creates them, and lets a user prepare a quotation from a saved template. The generated quotation retains the template's plan, name, lines, exact amount, and default validity.
- Removed generic session-security, copyright, and workspace-footer copy from the auth and signed-in shell.

## Files

- `backend/internal/orca/` — template handler, model, routes, PostgreSQL load/persist, API integration and isolation tests.
- `backend/migrations/008_quotation_template_plans.sql` — business-scoped plan relation.
- `frontend/src/` — template domain type/loading, form, list, use-in-quotation flow, and removal of redundant footer copy/styles.
- `docs/api/openapi.yaml`, `docs/requirements.md`, `readme.md`, `docs/history.md` — API contract, acceptance state, product overview, and verification record.

## Verification

- `make verify` passed: Go unit tests, Go race detector, `go vet`, frontend typecheck/lint, and production build.
- Isolated PostgreSQL HTTP billing flow passed through migration 008, including template create/list/use.
- Browser flow with a signed-in PostgreSQL-backed account created a saved plan/product template, reloaded it from the API, and created a quotation from the template.
- Desktop 1440×900 and mobile 390×844 had no horizontal overflow. Axe reported zero violations after waiting for mobile navigation to settle. `git diff --check` passed.

## Known limits

- Template update and delete actions are not implemented.
- The mobile/desktop Axe pass does not verify every color pairing manually; avatar and amber badge contrast remain part of the wider design review.
- Automated email delivery and the full accepted-quotation-to-subscription role workflow remain open in the acceptance matrix.
