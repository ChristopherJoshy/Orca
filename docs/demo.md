# Demo journey

## Current status

This is a guided product demonstration with a clear boundary between exercised workflows and remaining acceptance work. The local API smoke covers signup, customer/product/plan/discount/quotation/subscription creation, recurring draft invoice generation, stored invoice evidence, confirmation, manual payment, report integrity, full ZIP export, and cross-business isolation. Focused tests cover Copilot tools and approved actions, filtered export output, MCP OAuth/Streamable HTTP behavior, and Stripe Checkout/webhook reconciliation. The clean-schema PostgreSQL flow exercises approval execution, MCP grant persistence, and the current Stripe schema. A direct Groq connectivity smoke returned HTTP 200, but the full Copilot/provider workflow, live Stripe test-mode checkout, and email delivery have not been exercised. Browser checks cover the dashboard at desktop, tablet, and mobile widths plus Axe scans of the main workspace routes; complete role-based and workflow browser acceptance remains open.

Use a disposable local business, fake customer data, and provider test mode. Expected journey:

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
11. In a development environment, exercise MCP discovery/authorization, a scoped read, and a proposal followed by ORCA review. Automated SDK tests are the current evidence; no third-party client has been certified.

Never imply that an unconfigured provider action succeeded. The demo is complete only after recorded run evidence exists.
