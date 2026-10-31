# Change history and verification

## 2026-10-31 — Improve landing-page type scale and responsive layout

- Increased the public home's heading, body, card, navigation, and workflow text sizes and widened the content frame to reduce excessive margins on large displays.
- At tablet widths, the hero stacks, the primary navigation collapses, and the billing-principles rail wraps so its content remains visible.
- **Verification:** frontend typecheck/lint and production build passed. Browser checks at 2048×1168, 1440×900, 1024×768, 900×900, 834×1112, 768×1024, 390×844, 375×812, and 320×740 found no horizontal overflow; Axe reported zero violations and zero incomplete checks. Commit `65a00f9` reached `READY` in Vercel. The live page also passed Axe at 1920×1080 and 390×844; homepage, API health, and OAuth discovery returned HTTP 200.

## 2026-10-31 — Publish the public homepage to canonical Vercel

- Pushed landing-page commit `153beb5` to `main`; the canonical `orca` project deployed it to production and reached `READY` at [orca-orcin.vercel.app](https://orca-orcin.vercel.app).
- Verified the live homepage, same-origin API health (`database: postgres`, `status: ok`), and OAuth authorization-server discovery all returned HTTP 200. Production browser inspection confirmed the page, create-workspace navigation, return-to-home link, and zero Axe violations or incomplete checks. The signup form was opened but not submitted.
- Confirmed the Vercel account inventory contains one ORCA project (`orca`); unrelated projects belong to separate products and were left intact.
- **Remaining limit:** production signup is unavailable because no Resend sender/API key is configured. A prior signup probe returned 503 and left zero user rows; no production account or demo records were created.

## 2026-10-31 — Public product landing page and Vercel project cleanup

- Replaced the signed-out root screen with a public ORCA home page inspired by spacious, typography-led infrastructure sites. The page explains recurring billing, Invoice Truth, Billing Control Tower, and the Copilot using real product behavior, with direct sign-in and workspace creation and no demo CTA, invented customer logos, or unsupported metrics.
- Kept protected routes and auth links available; added responsive navigation and an original billing-cycle illustration.
- Removed three unused Vercel projects connected to the same repository (`orca-wich`, `chrisorca`, and `frontend-bwk1`) after identifying their mismatched build settings. Preserved the canonical `orca` project and its `orca-orcin.vercel.app` domain.
- **Local verification:** frontend typecheck/lint and production build passed with the canonical HTTPS origin. `agent-browser` reviewed desktop (1440×1000) and mobile (390×844); sign-up and return-home navigation worked, mobile width matched the viewport, and Axe reported zero violations and zero incomplete checks. The subsequent production deployment and live checks are recorded above.
- **Known limit:** live signup still requires a configured Resend sender; the production API returns 503 before creating an account when email delivery is unavailable.

## 2026-10-31 — Honest email limits and production signup guard

- Added an ORCA-styled dialog for invoice and quotation send actions, signup, customer access, password reset, and provider settings when email is unavailable. It explains that this build did not fund automatic email and confirms that nothing was sent or queued and no document/account state changed.
- Added backend configuration checks for signup, customer signup, password reset, invoice/quotation sends, and Copilot email proposals/approvals. Without both Resend credentials and a sender, production requests return 503 before creating an unusable account, token, document transition, approval, or outbox event.
- Updated README, OpenAPI, integration, agent, deployment, and acceptance documentation to reflect actual behavior.
- **Verification:** `ORCA_PUBLIC_URL=https://orca-orcin.vercel.app VERCEL_ENV=production make verify` passed (Go tests, race detector, `go vet`, frontend typecheck/lint, and production build). OpenAPI YAML parsing and `git diff --check` passed. A production signup probe later returned 503 before any database writes because email delivery is unconfigured; release and database non-mutation evidence is recorded in the leading entry.
- **Known limit:** signup, customer portal linking, and password recovery need a configured verified sender. No test account was created in production without a deliverable verification link.

## 2026-10-31 — Requirements, application foundation, and local workflow

- Added the contributor guide, product contracts, API outline, local setup, and acceptance matrix before implementing the app.
- Built the Go/chi API, PostgreSQL/Goose persistence, auth/session flows, tenant-scoped collection handlers, subscription lifecycle, recurring invoice calculation, snapshots/hashes, journal entries, manual payment allocation, reports, export jobs, and run/approval/MCP-management endpoints.
- Built the React/Vite workspace, auth entry points, create forms, invoice evidence view, reports, exports, responsive styles, search/filter controls, and clear demo-versus-workspace data behavior. Corrected quotation plan association, plan minimum quantities, discount minimum quantity, invoice deep-link loading, and local demo export re-download behavior.
- Added a regression for empty list responses and for plan-only subscriptions. Export output includes business tax rules; CSV output applies formula escaping.
- **Verification:** `make verify` passed: Go unit tests, Go race detector, `go vet`, TypeScript typecheck, the current `lint` alias, and Vite production build. The lint alias is typecheck, not ESLint.
- **PostgreSQL integration:** isolated-schema HTTP billing flow and invoice period uniqueness tests passed with migrations 001–004.
- **Local API smoke:** signup, missing-CSRF denial, create customer/product/plan/discount/quotation/subscription, activate, generate a draft invoice, verify snapshot/hash and ₹1,000 − ₹100 + ₹162 = ₹1,062, confirm, allocate full payment, verify Paid/zero balance, check balanced journal and zero hash mismatches, create/download a ZIP, and verify a second business sees `items: []` — passed.
- **Export contents smoke:** created a tax rule, requested and downloaded a private ZIP, and verified `tax_rules.json`, `manifest.json`, and `invoices.csv` entries — passed.
- Invoice print view now uses the current workspace business name. Demo invoice uses the demo name.
- **Contract/doc checks:** OpenAPI YAML parsed; trailing-whitespace scan was clean.
- **Not verified:** browser visual/accessibility walkthrough (browser host/automation unavailable), real Resend/Stripe/Groq activity, load/performance targets, and crash-safe worker behavior.
- **Not implemented at this checkpoint:** Groq tool execution, action execution from approval, River workers/outbox delivery, Stripe checkout/payment reconciliation, Streamable HTTP MCP/OAuth, filtered export application, invoice credit/proration, and full edit/archive CRUD. The later entry below records the first set of those features; hosting was not performed.

## 2026-10-31 — Copilot approvals, MCP, filtered exports, and handoff

- Added a bounded Groq tool loop for scoped search, reporting, invoice explanations, billing previews, and proposals for a limited set of actions. Added cancellation of an in-flight model request. The agent remains an incomplete subset of website actions and does not safely resume after process restart.
- Made ORCA approval execute supported invoice confirmation/cancellation/send request, manual payment allocation, and subscription close within the transaction that records the decision. Revalidates proposer membership, exact inputs, and resource version; persists action result; stale approvals have no side effects. Invoice email is queued in the outbox; no delivery worker is present.
- Added the official Go MCP SDK Streamable HTTP server, metadata, dynamic registration, S256 PKCE and ORCA consent, scoped one-hour access grants, revocation, and shared read/simulate/propose tools. Registration and pending authorization state remain process-local.
- Added filtered export selection with relationship closure, repeatable-read PostgreSQL snapshot, JSON/CSV, printable HTML, manifest/checksums, expiry checks, and CSV/HTML escaping. Jobs and files are process/local-disk bound; periodic deletion remains open.
- Added PostgreSQL migration 005 for approval execution results and MCP grant tests. Added same-origin Vite proxy for MCP and OAuth discovery routes. Updated the README, contracts, acceptance matrix, and handoff notes to describe implemented behavior and remaining work.
- **Verification:** `make verify` passed after the final code changes: Go tests, race detector, `go vet`, frontend typecheck, current lint alias, and production build. Isolated-schema PostgreSQL HTTP billing/approval/MCP-grant flow and invoice-period uniqueness test passed through migrations 001–005. Targeted Copilot, approval, export, and MCP tests passed. OpenAPI YAML parsed. Browser walkthrough remained unavailable.
- **Not verified:** real Groq, email, or Stripe activity; load/performance targets; durable worker recovery; third-party MCP client compatibility; browser visual/accessibility inspection. No deployment was performed.
- **Repository handoff:** after the user requested initialization, the local repository was initialized on `main` and this implementation was recorded as commit record 0001. No remote is configured, so no push was made; provide or configure the intended remote to continue the first-push handoff.

## 2026-10-31 — Stripe Checkout, finance report filters, and responsive accessibility

- Added customer-initiated Stripe Checkout for the customer's own confirmed INR invoice. The server derives the amount from the current receivable, persists a stable attempt before calling Stripe, accepts test-mode keys only, validates hosted URLs, and posts payments only from verified, idempotent webhook events. Amount mismatches and unmatched successes are retained for reconciliation.
- Added payment retry/reconciliation persistence and transactional webhook posting with payment allocation, balanced journal, audit, and outbox records. Added finance report tabs and All time / This month / This quarter / This year filters, with invoice issue dates and payment dates applied to their respective summaries.
- Improved tablet/mobile layouts across billing and settings pages; made data-table scrollers keyboard focusable; added accessible mobile drawer state, Escape handling, and focus return; corrected report action names and status contrast. Rewrote the README for setup, workflow, architecture, provider configuration, verification, and known limits.
- Updated the OpenAPI contract, billing/integration/design/development/testing/demo/requirements docs, and recorded the discovered/fixed accessibility issue in `mistakes.md`.
- **Verification:** `make verify` passed after the final implementation changes: Go tests, Go race detector, `go vet`, frontend typecheck, the current lint alias (also TypeScript typecheck), and Vite production build. `git diff --check` and OpenAPI YAML parsing passed.
- **PostgreSQL:** isolated-schema billing/approval/MCP integration and invoice-period uniqueness checks passed through migration 005 earlier in this session. Stripe checkout/webhook PostgreSQL coverage passed through migrations 006–007. The schema tests leave the shared database unchanged.
- **Browser:** `agent-browser` scanned all 17 application routes at 1440×900, 834×1112, and 390×844. Every route had zero horizontal overflow and zero Axe violations. Report tabs/date filters were exercised; mobile navigation opened, closed with Escape, and returned focus to the trigger. This is UI/accessibility evidence, not full role-based end-to-end acceptance.
- **Provider smoke:** one direct Groq chat-completions request returned HTTP 200, confirming connectivity only. No live Stripe sandbox checkout, email delivery, or complete Copilot tool workflow was run.
- **Still open:** load/performance measurement, durable worker crash recovery, complete role and workflow acceptance, external MCP client certification, and remaining partial/pending items in [requirements](requirements.md). No deployment was performed.

## 2026-10-31 — Auth-first workspace, MCP/team settings, and billing reload fixes

- Removed the demo/file-store path and browser-local business-data fallbacks. The unauthenticated entry screen now provides sign-in, workspace signup, customer access, verification, reset, retry, and connection-error states.
- Fixed PostgreSQL state cloning to preserve private Argon2 password hashes. Added a regression test after a fresh account could be created but failed to log in because a subsequent session update erased its hash.
- Fixed the workspace mapper to accept exact decimal strings for stored invoice unit prices; stopped recomputing subscription totals in the browser when the API supplies the authoritative total.
- Added a Billing Control Tower action for running billing, made Copilot visible in the main navigation, added inline icons, and added Settings sections for team access and External MCP endpoint/discovery/scopes/OAuth grant revocation.
- Updated contributor rules, API/MCP docs, and local test instructions to prohibit demo records and keep numbered commit records and verified pushes.
- **Verification:** `make verify` passed after these code changes (Go tests, race detector, `go vet`, TypeScript typecheck, current lint alias, and production build). `git diff --check` passed. The browser signup → sign-out → login flow passed on a fresh account. A real local customer, plan, subscription, and recurring draft invoice were created; Invoice Truth displayed stored totals and hash. A test invoice was confirmed and fully paid in the earlier populated account. Copilot UI opened at mobile width; Axe reported zero violations on `/copilot`.
- **Not verified:** prompt submission to Groq was rejected by automatic review because it could send workspace billing records to the external provider. Real Groq tool execution, Resend, Stripe checkout, external MCP client compatibility, all-role browser acceptance, and performance/load targets remain open. No hosting was performed.

## 2026-10-31 — Verified team access across workspaces

- Fixed staff creation to resolve already-verified ORCA users across workspace boundaries while rechecking destination-workspace admin access in the write transaction. Only the destination business receives a staff membership; other roles remain unchanged.
- Added audit logging with the member's UUID as the resource ID and tests for cross-business access, audit identity, and rejection of unverified accounts.
- **Verification:** Go tests passed. Against local PostgreSQL, adding an existing verified account returned HTTP 201 and a page reload showed the active Staff member in the workspace table. `git diff --check` passed.
- **Known limit:** staff members must already have a verified ORCA account; this path does not send invitation email.

## 2026-10-31 — Product narrative and backend setup guide

- Reworked the README to state the competition problem, ORCA's evidence-first product thesis, implemented behavior, team credits, architecture, and remaining limits without claiming unfinished automation or production readiness.
- Added backend runtime, cookie, tenant, environment, and private ingress guidance to the canonical contributor guide. Added the local Vite proxy and export-directory variables to environment examples; no secrets were added.
- **Research:** linked official Stripe subscription/proration/invoice-preview references and PostgreSQL locking/constraint references. The product differentiation is framed as inspectable billing evidence and operational integrity, not a claim that ORCA has more features than established platforms.
- **Verification:** manual content/claim review and `git diff --check` passed. No application behavior changed.
- **Deferred:** Vercel rewrite configuration and VPS setup are scheduled for the later hosting phase requested by the owner.

## 2026-10-31 — Reusable quotation templates and cleaner auth screens

- Added tenant-scoped create/list endpoints for quotation templates. Templates snapshot one plan line and an optional product line with quantities, exact decimal-string prices, tax rates, and validity days; the schema stores the plan reference with a composite business constraint.
- Added a Quotations-page template list, create form, and “start from a saved template” flow. A created quotation inherits the selected template's plan, title, lines, and default validity. Added API contract and in-memory plus isolated PostgreSQL coverage through migration 008.
- Removed generic session-security and copyright/workspace footer text from the sign-in and signed-in screens at the user's request.
- **Verification:** `make verify` passed. The local PostgreSQL browser flow created and reloaded a plan/product template, then created a ₹1,250 quotation from it. Desktop (1440×900) and mobile (390×844) had no horizontal overflow; Axe reported zero violations after the mobile drawer settled.
- **Known limits:** template edit/delete are not implemented. The browser Axe run does not replace manual verification of every color pairing; avatar and amber status contrast remain in the broader visual review.

## 2026-10-31 — Customer quotation acceptance and conversion

- Added a customer portal view for sent offers, including validity, total, status, and an action to accept an unexpired offer. The API enforces customer ownership, sent status, and business-timezone expiry.
- Staff can convert an accepted quotation into a draft subscription. The handoff retains the quoted customer, plan, product lines, quantities, and prices; API response totals now sum every stored line, and quote conversion is recorded in the same store transaction.
- Updated the OpenAPI contract, README workflow description, and acceptance matrix. Added tests for successful acceptance/conversion, expired offers, and foreign-customer rejection.
- **Verification:** focused Go tests and frontend typecheck passed. The saved local browser workflow also completed acceptance and conversion. Browser automation could not be re-opened in this session; full `make verify` is being run before commit.
- **Known limits:** quotation email delivery remains queued through the outbox and was not sent by a live provider; template editing and deletion remain open.

## 2026-10-31 — Clean up authentication microcopy and contrast

- Removed the redundant auth card eyebrow and the small `Private by workspace / INR billing / Audit-ready records` footer line. The entry screen now uses the page title and task-specific description for context.
- Darkened inactive account-tab text to clear WCAG AA contrast against its lavender-gray surface while preserving the existing plum/lavender palette.
- **Verification:** frontend typecheck and production build passed. Isolated agent-browser inspection at 1440×900, 834×1112, and 390×844 found no horizontal overflow; Axe reported zero violations and zero incomplete checks at all three sizes. `git diff --check` passed.

## 2026-10-31 — Automatic recurring billing catch-up

- Extracted invoice generation into a shared business-scoped service used by the staff endpoint and an in-process worker. The API now catches up every business at startup and on `ORCA_BILLING_INTERVAL` (default one minute), logs a tenant failure and continues, and waits for the worker to stop during shutdown.
- Added sorted business-ID enumeration without loading tenant records, plus an in-transaction period index to avoid rescanning every invoice for each subscription period. Database advisory locks serialize manual runs and worker passes for the same business; persisted cursors and unique period identities recover missed work.
- Added tests for January 31 monthly anchors, multiple missed periods, repeat runs, future start dates, business isolation, failure isolation, and cancellation. Updated the README, contributor/environment guide, billing contract, architecture, deployment handoff, and acceptance matrix.
- **Verification:** `make verify` passed (Go tests, race detector, `go vet`, frontend typecheck, lint alias, production build); focused catch-up/worker tests passed; `git diff --check` passed.
- **Known limits:** worker tests use the in-memory store; clean-schema PostgreSQL worker and sustained multi-worker/1,000-subscription load acceptance remain open. Email/outbox delivery is still not implemented.

## 2026-10-31 — Vercel and VPS hosting preparation

- Created and linked a dedicated Vercel `orca` project, configured it for Vite, Node 24, `npm ci --prefix frontend`, the frontend build, and `frontend/dist` output.
- Added a root `vercel.json` with SPA deep-link fallback, same-origin uncached API/MCP rewrites to unique Nginx prefixes, and baseline response headers. Added a production-build step that creates MCP OAuth discovery files as static assets because Vercel reserves `/.well-known` from rewrites.
- Added deployment templates for the loopback-only `127.0.0.1:8081` systemd service and additive Nginx routes, plus a VPS environment template. The existing VPS has Nginx on 80/443, an IP TLS certificate with an active renewal timer, and no listener on 8081; AWS ingress was not changed.
- **Verification:** frontend typecheck/lint/build passed; `vercel deploy --dry` recognized Vite, honored `.vercelignore`, found the configured output, and included both generated `.well-known` files. The Vercel project settings were read back after update.
- **Not yet deployed:** production database credentials and exact project production domain must be set before the API service and public routing can be verified end to end.

## 2026-10-31 — Product README and team credits

- Refined the README around ORCA's verified billing-evidence and recovery approach, clarified that workspace records are account-backed with no demo workspace, updated the current acceptance gaps, and included the two confirmed hackathon team members.
- **Verification:** `git diff --check -- readme.md` passed. No team details were inferred from evaluator information.

## 2026-10-31 — Discount validity windows

- Added admin inputs for discount start and end dates. The client sends explicit UTC day boundaries so the selected end date remains eligible through the full day.
- The API rejects a reversed date range and negative use limits, defaults omitted minimum quantity to one, returns both dates, and reports rules as inactive before their start, after expiry, or when their usage cap is reached.
- Added API coverage for a future-starting discount and reversed date bounds, updated the OpenAPI contract and requirements matrix, and made the discount table show the validity window.
- **Verification:** focused discount API test passed; `make verify` passed, including race checks, vet, typecheck, lint alias, and production build; `git diff --check` passed.
- **Known limits:** concurrency and failed-attempt discount consumption still need acceptance coverage; availability decisions for invoices remain based on the billing engine's effective service date.

## 2026-10-31 — Customer subscription pause and resume

- Added a resume action to the customer portal for paused subscriptions whose plan permits pausing; renamed the pause action to describe the immediate state change accurately.
- Added an API test proving an authenticated customer can pause and resume their own subscription, with an open pause interval created and then closed on resume. The endpoint continues to enforce customer ownership and plan permission.
- **Verification:** focused Go route test passed; `make verify` and `git diff --check` passed.
- **Known limits:** customer self-service remains limited to pause/resume; broader lifecycle policy coverage remains open.

## 2026-10-31 — Posted receivables and partial payments

- Corrected finance summaries to exclude Draft and Cancelled invoices from invoiced totals and receivables; overdue counts only confirmed invoices with a remaining past-due balance.
- Added an HTTP flow covering partial allocation (invoice stays Confirmed), final settlement (Paid), and the resulting receivable/overdue totals while unrelated Draft and Cancelled invoices remain excluded.
- **Verification:** focused Go test and `make verify` passed; `git diff --check` passed.

## 2026-10-31 — Resend outbox delivery

- Added a cancellable worker that sends queued invoice and quotation messages using stored document evidence, escaped HTML templates, and stable tenant-bound Resend idempotency keys. Delivery attempts/errors and completion timestamps are saved on the existing outbox rows.
- Wired the worker into API startup and graceful shutdown with configurable `ORCA_EMAIL_INTERVAL` (default one minute). Missing Resend credentials leave the worker disabled and queued events intact.
- Added fake-provider tests for delivery, ambiguous failure/retry, duplicate processing, stable keys, and HTML escaping; no live customer messages were sent. Resend retains idempotency keys for 24 hours, so ambiguous retries must occur within that window.
- Updated environment templates, provider contracts, README, deployment notes, and acceptance evidence.
- **Verification:** `make verify` passed, including the race detector; OpenAPI unaffected; `git diff --check` passed.
- **Known limits:** live sender/provider behavior is unverified; email worker is in-process, and account verification/reset emails still use direct provider calls.

## 2026-10-31 — Plan-aware subscription lifecycle actions

- Replaced the dead “Manage” action with only the valid staff actions for each subscription state and plan policy: quote/confirm/activate, pause/resume, early close, and renew.
- Renewal now starts on the business-local date and clears an already-expired term end so the recurring worker does not immediately skip the renewed subscription.
- Added route coverage for renewing a closed subscription whose prior expiry has passed; the customer portal's owner-scoped pause/resume coverage remains in place.
- **Verification:** focused lifecycle tests, frontend typecheck/build, `make verify`, and `git diff --check` passed.
- **Known limits:** full cross-product policy matrix and subscription future-start browser acceptance remain open.

## 2026-10-31 — Catalog maintenance, quote pricing, and filtered finance exports

- Added version-checked admin editing and soft archival for products and plans, including variants, plan lifecycle flags, and explicit confirmation that archived catalog terms remain on existing subscriptions. Catalog creation is admin-only and public create input cannot set internal archive state.
- Added plan and product variant selection to direct quotations, subscriptions, and quotation templates. The API resolves prices, taxes, names, and additive variant adjustments from the business catalog; template-backed quotes ignore caller-supplied replacement lines and quote totals use the exact calculation engine including line taxes.
- Added business-local inclusive report date filters to the summary API and CSV export for the selected Revenue, Invoicing, Collections, or Receivables report. CSV output neutralizes spreadsheet formulas and control prefixes.
- Corrected term-end catch-up: billing now creates any due periods that started on or before the inclusive expiry date before stopping later periods; auto-close is audited.
- **Verification:** `make verify` passed after implementation: Go tests, race detector, `go vet`, TypeScript typecheck, lint alias, and production build. `git diff --check` and OpenAPI parsing are recorded with the numbered change record. A real-account browser journey cannot be repeated until the local PostgreSQL/API services are available; previous account-backed evidence remains in the earlier entries.
- **Open:** clean-schema migration acceptance for catalog archiving, full role acceptance, and provider/performance checks remain incomplete.

## 2026-10-31 — ORCA-reviewed Copilot subscription lifecycle proposals

- Extended Copilot proposals and server-side ORCA approval execution to pause, resume, and renew subscriptions, with plan permission, actor membership, subscription state, exact input, and version revalidation. MCP remains proposal-only.
- Updated the product-agent action matrix and review guidance, including business-local renewal dates and status/version checks.
- **Verification:** covered by the combined `make verify` run after integration. Real Groq tool execution and restart-safe agent runs remain unverified.
- **Open:** complete Copilot website-action coverage and durable run resumption remain incomplete.

## 2026-10-31 — Contributor VPS deployment access notes

- Documented the owner-approved VPS target and external SSH key location in `AGENTS.md`; the private key contents remain outside the repository.
- Clarified that deployment requires an explicit hosting phase and available target credentials.
- **Verification:** `git diff --check` passed. No remote service was changed by this documentation update.

## 2026-10-31 — Correct Vercel MCP cache-header routing

- Removed the invalid `headers` property from the `/mcp` rewrite and moved its private, uncached response header into the top-level Vercel `headers` configuration.
- **Verification:** JSON parsing and `vercel deploy --dry` passed; the dry-run inspected the Vite project and uploaded nothing.
- **Not deployed:** the owner will run the Vercel deployment after the VPS API is ready.

## 2026-10-31 — Visual README and full screenshot gallery

- Rebuilt the README around the invoice question, the three product ideas, a workflow feature map, real screenshots, a Mermaid architecture view, local setup, verification, and explicit remaining limits.
- Captured every workspace and customer-portal screen at 1440 px wide, plus the workspace and portal at 390×844, from a disposable local account. Optimized the set to 20 WebP files under `docs/assets/screenshots/` and added a custom SVG banner.
- Added `docs/screenshots.md` with grouped galleries and context for overview, reports, Copilot, customers, catalog, plans, quotations, subscriptions, invoices, Invoice Truth, payments, settings, discounts, taxes, team, exports, portal, and mobile views. Linked it from the README and documentation index.
- Deleted the disposable businesses, users, memberships, and all seeded billing, payment, export, and audit records after capture. Removed temporary credentials and helper files. No demo data, seed mode, or browser fallback was added to the product.
- **Verification:** `make verify` passed (tests, race detector, vet, TypeScript typecheck, lint alias, and production build). Every local image and document link resolves; README and gallery anchors resolve; `git diff --check` passed.
- **Known limits:** a few captures show development copy defects, including Invoice Truth label concatenation and an export status rendering issue. They are recorded as UI follow-up work rather than presented as finished polish.

## 2026-10-31 — Vercel frontend and VPS API deployment

- Connected the canonical Vercel project `orca` to `ChristopherJoshy/Orca`, configured its production origin, and deployed the frontend at [orca-orcin.vercel.app](https://orca-orcin.vercel.app).
- Installed PostgreSQL-backed API service and Goose migrations 001–009 on the owner-approved VPS. Nginx now routes the isolated `/orca-api/` and `/orca-mcp/` prefixes over its existing HTTPS certificate; the Go and PostgreSQL listeners remain loopback-only.
- Updated the README and deployment guide with the hosted URL, production routing, service state, and remaining provider/acceptance work. No demo account or sample production records were added.
- **Verification:** local production frontend build passed; Vercel deployment reached `READY`; direct VPS API health, Vercel same-origin API health, and OAuth discovery returned HTTP 200; `nginx -t` passed; PostgreSQL reported all nine application migrations applied.
- **Open:** Resend and Stripe credentials are not configured, no real test account has been created, automated database backups are not configured, and the full browser workflow remains to be exercised.
