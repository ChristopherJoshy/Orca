# Local end-to-end test journey

## Current status

ORCA does not include demo accounts or sample workspace records. Run the API and PostgreSQL, create a disposable account from the sign-up screen, then exercise records stored in that account's business. Never create test business records in browser-local storage. Use fake customer details and provider test mode.

Existing local evidence covers signup/login, customer/product/plan/discount/quotation/subscription creation, recurring draft invoice generation, stored invoice evidence, confirmation, manual payment, report integrity, full ZIP export, and cross-business isolation. Focused tests cover Copilot tools and approved actions, filtered export output, MCP OAuth/Streamable HTTP behavior, and Stripe Checkout/webhook reconciliation. PostgreSQL integration checks run in an isolated schema. A direct Groq connectivity smoke returned HTTP 200, but the full Copilot/provider workflow, live Stripe test-mode checkout, and email delivery have not been exercised. See [verification history](history.md) for the latest browser matrix and evidence.

Use a disposable local business and provider test mode. Expected journey:

1. Create and verify a business admin account; confirm invalid passwords and duplicate email are rejected.
2. Create a product with a variant price adjustment, tax rule, monthly plan, and quotation template.
3. Create a customer and a quotation from the template. Review validity and line totals, then move it to confirmed subscription.
4. Activate at its start date. Show the stored billing period anchor and next period.
5. Generate a draft invoice once; repeat the job/request and show the same invoice identity with no duplicate journal.
6. Review Invoice Truth: stored line calculation, discount-before-tax steps, engine version, and hash.
7. Confirm invoice and record a partial manual payment. Show remaining receivable and journal balance; record final allocation and show Paid.
8. Run filtered active-subscription, revenue, collection, and overdue views.
9. Ask Copilot to search records, explain an invoice, or preview a bill. For a supported action, inspect the exact proposal and approve in ORCA; show the persisted result. Invoice email approval queues an outbox request but does not deliver an email in this build.
10. Request a filtered export, inspect its manifest/count/checksum and scoped download. Expiry is enforced on download; physical archive cleanup remains open.
11. In a development environment, exercise MCP discovery/authorization, a scoped read, and a proposal followed by ORCA review. Automated SDK tests are current evidence; no third-party client has been certified.

Never imply that an unconfigured provider action succeeded. Treat the journey as complete only after the account, business records, browser run, and provider outcomes have been recorded in `docs/history.md`.
