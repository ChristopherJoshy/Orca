# Verification strategy

Required checks are mapped in [requirements.md](requirements.md). A successful compile is not evidence that the business workflow is correct.

## Layers

- **Unit:** exact decimal arithmetic, half-up rounding, calendar anchors, state transitions, server-resolved plan/product/variant prices, template price-tampering rejection, tax-inclusive quotation totals, discount eligibility/caps/failed attempts, inclusive report date filters in the business timezone, invoice issue-date versus payment-received-date semantics, catalog version conflicts and archive, expiry catch-up/auto-close audit, canonical serialization/hash, CSV sanitization, approval digest.
- **Database integration:** tenant scoping, unique period identities, transaction rollback, balanced journals, atomic discount consumption, payment allocation, outbox/job dedupe, migration up/down where safe.
- **Concurrency/resilience:** 20 workers processing 1,000 due subscriptions yields exactly one invoice per due period, no missing periods, balanced journals; crash after every durable boundary; downtime catch-up; duplicate webhooks; ambiguous provider result reconciliation.
- **Email outbox:** fake Resend HTTP tests verify invoice/quotation delivery payloads, escaped customer content, stable idempotency keys, persisted attempts, retry after ambiguous failure, and no re-send after delivery. Do not send live customer emails during automated tests.
- **End-to-end:** signup/session -> business/customer -> product/plan -> quote -> subscription -> invoice -> payment -> finance report, plus self-service and role denial cases.
- **Agent:** per-page action/tool matrix, provider failure, multiple tool steps, ambiguity question, approval binding, stale version, unauthorized request, cancellation, refresh/reconnect, safe resume and accurate partial failure.
- **MCP:** discovery, PKCE authorization, scoped reads, proposal-only behavior, revocation, expiry, cross-business denial, and ORCA approval before execution.
- **Export:** stable snapshot, record counts and relationships, checksums, private download, permission scoping, expiration and spreadsheet-formula neutralization.
- **UI/accessibility/performance:** inspect every page at desktop/tablet/mobile, keyboard-only navigation, accessibility checks, seeded thousands of rows and sub-two-second ordinary-page latency measurement.

## Evidence discipline

Record exact command, environment, seed size, result, and known limitation in `docs/history.md`. Never use live charges or send customer messages during tests. Use provider test/sandbox mode with disposable data. The direct Groq connectivity smoke is not a full Copilot workflow, and Stripe adapter tests do not replace a live test-mode checkout. Email delivery and those end-to-end provider workflows remain unverified.
