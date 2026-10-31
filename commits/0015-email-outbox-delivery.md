# 0015 — Deliver queued invoice and quotation emails

## Result

- Added a Resend-backed processor for `invoice.email` and `quotation.email` outbox events. It renders invoice values from the stored calculation snapshot and quotation values from saved lines.
- Uses `html/template` for untrusted record text, a deterministic tenant-bound idempotency key, and persisted attempt/error/delivery fields.
- Added an immediate, periodic, cancellable worker to API startup and graceful shutdown. `ORCA_EMAIL_INTERVAL` defaults to one minute; without a key or sender, the worker stays disabled and queued work remains pending.
- Updated environment examples, provider/deployment docs, README, and the acceptance matrix.

## Verification

- Fake-provider tests cover invoice/quotation delivery, HTML escaping, stable keys, retry after ambiguous transport failure, persisted attempts, provider errors, and duplicate processing.
- `make verify` — passed: Go tests, race detector, `go vet`, TypeScript, lint alias, and Vite production build.
- `git diff --check` — passed.

## Known limits

- No live email was sent. Production delivery needs a verified sender and credentials. Resend retains idempotency keys for 24 hours; ambiguous retries after that window may not have provider-side duplicate protection.
- The worker runs in-process rather than on a separate durable scheduler. Account verification and reset email use direct Resend calls rather than the outbox.
