# Provider integration contracts

The product has narrow adapters so optional providers do not become business logic. No credential values belong in source, documentation samples, logs, exports, or agent conversations.

## Stripe

Use Stripe test mode for checkout. The ORCA invoice/ledger remains authoritative. A customer can initiate a hosted session only for their own confirmed invoice; ORCA derives the amount from the current receivable. Session creation carries a persisted ORCA operation key and opaque local checkout ID. Verify webhook signatures against the raw request body; process `checkout.session.completed` only when `payment_status=paid` and process delayed methods on `checkout.session.async_payment_succeeded`. Persist unique provider event IDs and the payment allocation in the same transaction; handle duplicate and out-of-order events idempotently. Reconcile ambiguous session-creation outcomes with the same idempotency key. Never mark an invoice paid on browser redirect alone. If a completed session no longer matches its local attempt or outstanding balance, retain a reconciliation exception instead of silently posting it.

## Email

Send verification/reset links, quotations, invoices, and collection notices through a delivery interface. Templates use stored document evidence and tenant sender settings. Persist message/outbox ID, recipient, purpose, provider ID, status, attempt, and timestamps; do not place reset tokens in logs. A failed email is retryable and does not roll back a committed invoice.

## Groq and agent model calls

The Go API currently uses Groq's OpenAI-compatible chat-completions API with configurable model; default: `openai/gpt-oss-120b`. The bounded loop supports scoped record search, report summary, invoice explanation, invoice simulation, and proposals for a limited set of reviewed billing actions. The backend executes an approved supported action only after rechecking actor, tenant, input hash, and resource version. Set request timeouts, tool limits, and structured schemas. Model availability is not a dependency for ordinary product operations. Tool responses are authoritative for numbers and mutations; the model may summarize but not invent a result. Run records persist, but in-flight work does not resume after API restart.

One direct Groq chat-completions request returned HTTP 200 in the local environment. This verifies credential connectivity only; the full Copilot multi-step tool workflow has not been verified against Groq. Keep this distinct from offline tool/approval tests.

## Configuration and status

Environment names are documented in `.env.example`. Configure `STRIPE_SECRET_KEY` with a test-mode secret key and `STRIPE_WEBHOOK_SECRET` with the endpoint signing secret. Readiness should report whether required configuration exists and whether a measured provider health check succeeded; never display secret values. When unconfigured/unreachable, keep manual payment, invoices, reports, and non-agent UI usable. Provider-specific installation/account provisioning is owner-managed. Live provider behavior is pending until sandbox evidence appears in history.
