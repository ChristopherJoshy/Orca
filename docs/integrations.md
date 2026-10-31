# Provider integration contracts

The product has narrow adapters so optional providers do not become business logic. No credential values belong in source, documentation samples, logs, exports, or agent conversations.

## Stripe

Use Stripe test mode for checkout. The ORCA invoice/ledger remains authoritative. Checkout session creation carries a stable ORCA operation key and references business, customer, and invoice identifiers using provider metadata that contains no secrets. Verify webhook signatures against the raw request body; persist unique provider event IDs before processing; handle duplicate, delayed, and out-of-order events idempotently. Reconcile ambiguous outcomes by retrieving the checkout/payment intent before retry. Never mark an invoice paid on browser redirect alone.

## Email

Send verification/reset links, quotations, invoices, and collection notices through a delivery interface. Templates use stored document evidence and tenant sender settings. Persist message/outbox ID, recipient, purpose, provider ID, status, attempt, and timestamps; do not place reset tokens in logs. A failed email is retryable and does not roll back a committed invoice.

## Groq and agent model calls

The Go API currently uses Groq's OpenAI-compatible chat-completions API with configurable model; default: `openai/gpt-oss-120b`. The bounded loop supports scoped record search, report summary, invoice explanation, invoice simulation, and proposals for a limited set of reviewed billing actions. The backend executes an approved supported action only after rechecking actor, tenant, input hash, and resource version. Set request timeouts, tool limits, and structured schemas. Model availability is not a dependency for ordinary product operations. Tool responses are authoritative for numbers and mutations; the model may summarize but not invent a result. Run records persist, but in-flight work does not resume after API restart.

No live Groq call has been verified in the current environment. Provider configuration and behavior should remain clearly distinguished from offline tool/approval tests.

## Configuration and status

Environment names are documented in `.env.example`. Readiness should report whether required configuration exists and whether a measured provider health check succeeded; never display secret values. When unconfigured/unreachable, keep manual payment, invoices, reports, and non-agent UI usable. Provider-specific installation/account provisioning is owner-managed. Live provider behavior is pending until sandbox evidence appears in history.
