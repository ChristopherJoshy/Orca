# Billing and financial invariants

This document is the source of truth for calculation and posting behavior.

## Money and calculation order

- Base currency is INR (`INR`) with two minor-unit digits. API money is a decimal string, e.g. `"1234.50"`; never use JavaScript or Go binary floating point for monetary arithmetic.
- Store calculation operands at adequate exact decimal precision. Round each invoice line's payable amount to INR paise using **half-up**; sum stored rounded line amounts for document totals.
- Calculation order: quantity × exact unit price, apply eligible discounts, floor at zero, calculate tax on the post-discount taxable amount, round line components according to the versioned engine policy, then total stored lines. Tax is after discount.
- A calculation snapshot captures source catalog/plan/variant/discount/tax versions, quantity, unit amounts, dates, discount allocation, taxable amount, tax rate and amount, line total, currency, engine version, ordered explanation steps, and canonical hash.
- Canonical serialization uses sorted keys, normalized decimal strings, UTC timestamps, and a documented hash algorithm. Persist the canonical bytes or enough immutable fields to reproduce them. Explain values from persisted evidence, never a new calculation against current catalog data.

## Period rules

- Service periods are half-open `[period_start, period_end)` in the business billing timezone; store instants in UTC and retain the timezone used to derive boundaries.
- Daily and weekly periods advance by calendar days/weeks. Monthly and yearly periods preserve the original anchor. If the anchor day does not exist in a short month, bill on that month's last day and restore the original anchor in later months (Jan 31 -> Feb 28/29 -> Mar 31). Leap-year yearly anchors follow the same rule.
- A period identity is `(business_id, subscription_id, period_start)`, protected by a unique constraint. Invoice generation retries return the existing result.
- Catch-up enumerates every due half-open period from the persisted cursor up to the cutoff and advances the cursor only in the transaction that records the invoice/outbox work. Each period is independent and idempotent.
- The API process runs a catch-up pass at startup and then on `ORCA_BILLING_INTERVAL` (default `1m`). Each business is processed in a separate tenant-scoped transaction; a failed business is logged and does not block the rest. A PostgreSQL advisory transaction lock serializes the worker with staff-triggered runs and other API instances for the same business. The persisted cursor and unique period constraint allow the next pass to recover periods missed during downtime. This is an in-process scheduler; production still needs process supervision and at least one live API worker.
- A subscription activates at its start date. Pause intervals are separately persisted and only excluded from service when plan rules define that behavior. Proration is a separately identified adjustment; it does not rewrite a prior invoice.

## Discount rules

Discount eligibility is evaluated at the invoice transaction time against the effective service date, date range, product/subscription scope, minimum purchase or quantity, and remaining use cap. Consumption is an atomic conditional update in the same transaction that creates the invoice. Failed calculation or invoice creation consumes nothing. Fixed discounts cannot reduce a line below zero. Percentage discount precision and allocation across eligible lines are captured in the snapshot.

## Invoice lifecycle and balances

Workflow state: `Draft -> Confirmed -> Paid`; cancellation is an explicit void/cancel action with audit and any required reversal. Balance is separate: `open_amount = total - allocated_payments - credits`. A confirmed invoice can be partially paid while remaining `Confirmed`; overdue is a computed predicate (`open_amount > 0 && due_at < now`). `Paid` requires zero open amount. Confirm/cancel and payment allocation are idempotent.

Finance summaries count only `Confirmed` and `Paid` documents as invoiced receivables. Draft and cancelled invoices do not contribute to posted invoice totals or outstanding balances; overdue counts only confirmed invoices with a positive balance past due.

Confirmed invoice data is immutable. A correction after finalization uses a credit/reversal or debit adjustment with its own linked snapshot and journal. Draft changes create a new calculation snapshot revision and preserve audit history.

## Double-entry journal

Each source operation posts balanced entries per currency. Minimum mapping:

| Operation | Debit | Credit |
|---|---|---|
| Confirm invoice | Accounts receivable: gross due | Revenue: net of tax; tax payable: tax |
| Record received payment | Cash/clearing: received | Accounts receivable: allocated |
| Issue credit/reversal | Revenue/tax/receivable according to source reversal | Matching balancing accounts |

Journal rows reference business, source type/id, operation id, currency, and exact amount. A deferred constraint or posting service validates debits equal credits before commit. Unique source-operation identities prevent duplicates. No finalized journal row is edited or deleted.

## Atomicity and provider reconciliation

The same DB transaction stores the invoice, snapshot, journal, audit event, outbox event, and durable job. Provider network calls happen outside that transaction. The job records provider operation key and attempt state. On timeout, query by stable key/provider object before retrying; never assume a charge or email failed because the HTTP response was lost. Stripe webhook signatures are verified on raw bytes before idempotent event insertion.

For ORCA Stripe Checkout, the current adapter accepts test-mode `sk_test_` credentials only. A customer starts a hosted session only for their own confirmed invoice. ORCA calculates the amount from its current outstanding balance and submits that exact INR amount; the browser never supplies the amount. Persist the local checkout attempt before contacting Stripe and reuse its idempotency key on retry. Reconcile only signed `checkout.session.completed` events whose `payment_status` is `paid`, or `checkout.session.async_payment_succeeded`; verify the provider session maps to the stored test session and that amount, currency, invoice state, and outstanding balance still agree. The success redirect is informational and never posts a payment. Duplicate sessions/events cannot allocate twice. If a successful provider payment no longer matches the receivable, record it for reconciliation and do not silently alter the invoice balance.

## Integrity metrics (Control Tower)

Report values are measured queries: due periods discovered, invoices created, duplicate conflicts, failed/retried jobs, stale cursors, provider reconciliation states, unbalanced journals, hash mismatches, and outbox lag. Every metric shows its query time/window. Do not substitute demo constants or a green status when a check has not run.
