# 0016 — Expose plan-aware subscription lifecycle actions

## Result

- Replaced the inert “Manage” control with state- and plan-aware actions in the staff subscription table: request quote, confirm, activate, pause, resume, close, and renew where allowed.
- Close still opens the ORCA review dialog. Pause and resume remain available to customers only for their own subscriptions when the plan allows them.
- Renewal now uses the business-local current date and removes a previous expiry date only after that date has passed, preventing a renewed subscription from being skipped by recurring billing.
- Added an HTTP test for renewal after expiry and updated the local workflow and requirements evidence.

## Verification

- Focused customer pause/resume and expired-renewal tests — passed.
- Frontend typecheck and production build — passed.
- `make verify` — passed: Go tests, race detector, `go vet`, TypeScript, lint alias, and Vite production build.
- `git diff --check` — passed.

## Known limits

- Full plan permission matrix and future-start subscription browser acceptance remain open.
