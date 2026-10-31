# Change history and verification

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
