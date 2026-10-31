# 0014 — Correct posted receivables and partial payments

## Result

- Finance summaries now count only Confirmed and Paid invoices as invoiced amounts and receivables. Draft and Cancelled records are excluded.
- Overdue counts only confirmed invoices with a positive remaining balance after the due date.
- Added an HTTP acceptance test that records a partial payment, confirms the invoice remains Confirmed with the correct balance, settles it, then checks that overdue and receivables decrease while Draft and Cancelled invoices do not affect totals.
- Updated the billing and requirements contracts.

## Verification

- Focused partial-payment/report HTTP test — passed.
- `make verify` — passed: Go tests, race detector, `go vet`, TypeScript, lint alias, and Vite production build.
- `git diff --check` — passed.

## Known limits

- Full report date-filter and role acceptance remains open.
