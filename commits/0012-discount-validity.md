# 0012 — Complete discount validity controls

## Result

- Added admin inputs for optional discount start and end dates, and displayed the inclusive validity window in the discount table.
- Convert chosen dates to explicit UTC boundary instants so the end date remains eligible through 23:59:59.999 UTC.
- Validate start/end ordering and reject negative use limits in the API. Normalize an omitted minimum quantity to one.
- Return start and end dates from the discount endpoint and compute current activity from the date window and remaining usage cap.
- Extend the OpenAPI contract and requirements matrix. Add an API test for future-start status and reversed date rejection.

## Verification

- Focused discount API test — passed.
- `make verify` — passed: Go tests, race detector, `go vet`, TypeScript, lint alias, and Vite production build.
- `git diff --check` — passed.

## Known limits

- Concurrency and failed-attempt discount usage acceptance coverage remain open. Invoice eligibility continues to use the billing engine's effective service date.
