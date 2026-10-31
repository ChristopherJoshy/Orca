# 0017 — Harden catalog terms, quotations, and finance reports

## Result

- Added admin-only product and plan updates with optimistic version checks and audited soft archival. Archived items disappear from active catalog selection while existing subscription, invoice, and export references remain intact.
- Added server-resolved product/plan/variant prices and tax rates for direct subscriptions, quotations, and quotation templates. Template-backed quotations copy stored terms and calculate tax-inclusive totals with the shared exact-decimal engine.
- Added business-local inclusive report filters to the summary API and formula-safe CSV downloads for the selected Revenue, Invoicing, Collections, or Receivables view.
- Fixed billing catch-up at subscription expiry: due periods beginning on or before the inclusive expiry date are generated before later periods stop; configured auto-close records an audit event.
- Updated the API/billing/export contracts, acceptance matrix, README, testing guidance, and verified mistake notes.

## Verification

- `make verify` — passed: Go tests, race detector, `go vet`, frontend typecheck, lint alias, and Vite production build.
- OpenAPI YAML parsing — passed.
- `git diff --check` — passed.

## Known limits

- A local PostgreSQL/API process was unavailable, so migration 009 and a fresh account-backed browser journey were not repeated in this batch. The VPS database is being prepared separately.
- Full role-based acceptance, large-data performance, and provider delivery remain open in the requirements matrix.
