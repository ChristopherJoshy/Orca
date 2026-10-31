# Verified mistakes and prevention

## 2026-10-31 — Missing email configuration blocked workspace signup

- **Observed:** the production signup handler rejected new business workspaces with HTTP 503 when Resend was not configured. That made ordinary login/signup depend on an email service that this build did not fund.
- **Cause:** one provider-readiness gate ran before the handler distinguished workspace accounts from customer-portal verification.
- **Correction:** workspace signup now creates an admin membership and secure session without email; the account remains explicitly unverified. Admin/staff sessions work without verification, while customer access still requires verified ownership. The UI retains the no-budget email notice after workspace creation.
- **Regression evidence:** the production-mode handler test covers workspace signup and login without email, the 503 customer/reset cases, and denial of unverified customer sessions. Full Go tests and frontend typecheck/lint/production build passed.
- **Prevention:** test provider-off behavior separately for each identity flow. Email availability must gate only workflows that require a delivered message.

## 2026-10-31 — Sign-in screen was too dense and visually heavy

- **Observed:** owner review rejected the sign-in presentation and supplied a simpler split-card reference.
- **Cause:** the auth route used a full-height split layout with three product-feature blurbs competing with the login form, and duplicated account-mode choices in tabs.
- **Correction:** moved to a centered white-and-lavender split card, shortened the product copy, made the workspace switch a single clear action, and carried that switch into the compact mobile header. Added a single page-level heading, corrected small-label contrast, and shortened the password placeholder.
- **Regression evidence:** local browser review at 1440×960, 1024×768, 768×1024, 390×844, and 320×740 found no horizontal overflow; Axe reported zero violations at desktop and phone sizes. Typecheck, lint, and production build passed. Commit `354344a` reached `READY` on Vercel; the live auth session loaded without a connection error, and sign-in, signup, reset, and customer portal views rendered. No account form was submitted.
- **Prevention:** use a visual reference for auth screens, keep login copy subordinate to the form, and review a desktop, tablet, and phone screenshot plus accessibility before releasing changes.

## 2026-10-31 — Sign-in story retained unnecessary micro-labels

- **Observed:** owner review identified the “Billing workspace” and “Subscriptions · recurring billing” labels as unwanted in the login view.
- **Correction:** removed both labels and their decorative separator/dot; retained the ORCA mark and primary statement.
- **Verification:** not run at the user's request.
- **Prevention:** keep the sign-in story limited to the brand mark, one statement, supporting sentence, and workspace switch.

## 2026-10-31 — Public landing typography and tablet layout were too constrained

- **Observed:** owner review found the public-home copy too small and the content frame overly centered. Browser inspection also showed the header actions touching the navigation and the billing-principles row clipping at tablet width.
- **Cause:** the desktop content max-width was narrow, supporting copy and card details used undersized font values, and the desktop header/rail persisted below their comfortable width.
- **Correction:** expanded the desktop frame, increased body and supporting type, stacked the hero and collapsed the navigation and principle rail before tablet content became cramped.
- **Regression evidence:** responsive review at 2048, 1440, 1024, 900, 834, 768, 390, 375, and 320 CSS pixels found no horizontal overflow; Axe reported zero violations and zero incomplete checks.
- **Prevention:** review wide desktop plus breakpoint edges and phone widths; inspect visible text size and actual element bounds instead of relying on a single desktop screenshot.

## 2026-10-31 — Duplicate Vercel projects obscured the canonical build status

- **Observed:** three extra Vercel projects for this repository reported failed deployments while the canonical `orca` project's deployment for the same commit was ready. Two duplicate projects lacked the effective HTTPS public origin required to generate MCP discovery metadata; the third combined a `frontend` root directory with root-relative install settings.
- **Cause:** multiple projects were connected to the same Git repository without a single canonical project/build configuration, making failed duplicate builds look like a production failure.
- **Correction:** after the owner requested removing unused ORCA projects, deleted `orca-wich`, `chrisorca`, and `frontend-bwk1`; retained `orca` and its production domain. The canonical deployment, API health, and OAuth discovery returned HTTP 200.
- **Prevention:** keep one documented production Vercel project, verify its root/build/install settings and `ORCA_PUBLIC_URL`, and inspect the canonical deployment status before diagnosing a failed duplicate as the live release.

## 2026-10-31 — Public landing page had weak contrast and an unnamed figure

- **Observed:** the first browser accessibility scan found three contrast failures and a generic illustration element with an unsupported accessible-name attribute.
- **Cause:** low-contrast small labels and an `aria-label` on a generic `div` were not checked before the first preview.
- **Correction:** darkened the workflow labels and represented the billing-cycle graphic as a named `figure`; the follow-up Axe run reported zero violations.
- **Prevention:** inspect new public UI at desktop and mobile widths and resolve accessibility findings before release.

## 2026-10-31 — Email actions appeared usable without a delivery provider

- **Observed:** document send controls and agent flows could reach outbox behavior even though this deployment has no Resend provider. Signup and reset also depend on email verification.
- **Cause:** provider absence was handled by a dormant worker rather than treated as a precondition at every email-producing API boundary, and the interface did not make the limitation clear.
- **Correction:** email-dependent APIs now reject before writes unless both provider credentials and a sender are configured. The UI shows an ORCA-styled explanation; tests assert no account, business, token, document mutation, approval, or outbox record is left behind.
- **Prevention:** keep provider readiness checks at API entry points, document unavailable behavior in the OpenAPI contract, and test the no-provider path alongside successful fake-provider behavior.

## 2026-10-31 — Product variant labels were confused with submitted values

- **Observed:** the new quote/subscription form displayed a combined variant label such as `Brand · Odoo` and used that display text as the value sent to the API. The server accepts only a catalog variant name or value, so products with distinct name/value fields would be rejected.
- **Cause:** presentation text and the catalog identifier shared one frontend property.
- **Correction:** the frontend now keeps the display label and exact variant value separately; the server resolves the value and applies the additive adjustment. Saved quotation templates also snapshot the selected variant.
- **Regression evidence:** Go coverage checks server-resolved variant prices in direct quotes/subscriptions and a template-to-quotation flow. Frontend build/type checks are part of final verification.
- **Prevention:** keep display labels separate from submitted identifiers and test a variant whose `name` differs from its `value`.

## 2026-10-31 — Template IDs did not prevent caller-supplied quote lines

- **Observed:** quotation creation loaded saved template terms only when the request contained no lines. Supplying a template ID together with arbitrary lines skipped that path and could leave caller-supplied prices in the quotation.
- **Cause:** template selection was treated as a defaulting hint instead of the source of the saved commercial terms.
- **Correction:** template-backed quotations always copy the template's server-stored lines; quotation totals now use the shared decimal calculation engine and include rounded line taxes.
- **Regression evidence:** template conversion coverage submits a spoofed line and asserts the saved variant price and tax-inclusive amount are retained.
- **Prevention:** resolve authoritative commercial terms inside the domain transaction and test adversarial requests that combine a trusted record ID with conflicting client input.

## 2026-10-31 — Catalog create payloads could set internal archive state

- **Observed:** after adding persistent `Archived` fields to product and plan models, their create handlers decoded directly into those models. A caller could submit `archived: true` and hide a newly created record from active catalog lists.
- **Cause:** storage state and public create input shared the same model type.
- **Correction:** create handlers now force archive state to false; only the admin archive action can change it. Admin-only create, update, and archive paths are covered by focused handler tests.
- **Prevention:** never expose lifecycle flags solely because they exist on a persistence model; set server-controlled fields in domain handlers and test caller-supplied values.

## 2026-10-31 — Expired subscriptions skipped eligible billing catch-up

- **Observed:** the billing worker checked term expiry before enumerating due periods. A delayed worker could therefore close a subscription and skip an unpaid billing period whose start date was still inside the term.
- **Cause:** expiration was treated as an all-or-nothing early exit instead of a boundary on period generation.
- **Correction:** the worker now completes due periods whose local start date is on or before the inclusive expiry date, stops before later periods, and records an audit event when plan policy auto-closes the subscription.
- **Regression evidence:** a catch-up test covers both auto-close and manual-close plans, the eligible invoice, no later period, and the audit event.
- **Prevention:** test delayed catch-up at term boundaries and apply expiry per billing period rather than before catch-up.

## 2026-10-31 — Session updates erased password hashes

- **Observed:** a newly created account could reach its workspace after signup, but a later email/password login returned an authentication error.
- **Cause:** `cloneState` used JSON serialization for deep copying, while `User.PasswordHash` is tagged `json:"-"` to keep it out of API/state JSON. PostgreSQL state updates therefore compared and persisted cloned users with an empty password hash.
- **Correction:** restore private password hashes explicitly after decoding the rest of the deep copy. Added a regression test for `cloneState`.
- **Regression evidence:** fresh UI signup, sign-out, and regular email/password login succeeded against PostgreSQL after the fix.
- **Prevention:** when using serialization-based internal copies, explicitly preserve fields intentionally excluded from JSON; cover authentication secrets and other excluded fields in clone tests.

## 2026-10-31 — Staff membership audit used a composite map key as a UUID

- **Observed:** adding a verified user from another business returned HTTP 500 and did not add the membership.
- **Cause:** the membership map key combines business and user IDs, but PostgreSQL stores the audit event's `resource_id` as a UUID. Passing the composite key failed the database cast and rolled back the transaction.
- **Correction:** retain the composite key only for in-memory membership lookup and store the member's user UUID as the audit resource ID. Staff lookup now loads globally visible accounts while rechecking destination admin authority in the transaction.
- **Regression evidence:** unit tests verify the audit resource UUID, cross-business membership isolation, and rejection of unverified users. The local PostgreSQL-backed browser flow returned HTTP 201 and displayed the active staff row after reload.
- **Prevention:** use a schema-valid resource identifier for audit references, and verify new write paths against PostgreSQL rather than only an in-memory Store.

## 2026-10-31 — Approval could report success without applying the action

- **Observed:** the approval endpoint could persist an `approved` decision without running the proposed billing action, leaving the review result disconnected from the requested business effect.
- **Cause:** decision persistence and domain execution were separate paths; the UI could treat an approval response as completion without an execution result.
- **Correction:** the endpoint now revalidates the bound actor, input hash, and resource version, executes supported actions inside the same business transaction as the decision, and stores the resulting record/action details. The UI renders the returned execution result.
- **Regression evidence:** unit tests cover approved payment execution/idempotency, stale-version rejection, and balanced-journal invoice confirmation; the isolated-schema PostgreSQL HTTP flow verifies persisted subscription-close results.
- **Prevention:** treat approval as a transactional command, require an explicit persisted outcome, and test both decision state and domain side effects for every action type.

## 2026-10-31 — Approval schema migration omitted a required column

- **Observed:** PostgreSQL signup/session requests returned a server error because state loading queried `approvals.inputs` before the column existed.
- **Cause:** the original core migration did not include the later approval-input persistence field.
- **Correction:** added Goose migration `004_agent_approval_inputs.sql` and applied it locally.
- **Regression evidence:** isolated-schema PostgreSQL integration tests migrated through version 4 and passed the HTTP signup-to-payment workflow.
- **Prevention:** keep runtime state serialization and migration schema changes in the same reviewed change; exercise a clean schema in integration tests.

## 2026-10-31 — Empty collections serialized as `null`

- **Observed:** the tenant-isolation smoke found a fresh business received `items: null`; the workspace loader relies on a collection response and could misclassify a valid empty workspace as demo mode.
- **Cause:** Go nil slices serialize as JSON `null`.
- **Correction:** the collection handler now normalizes absent results to an empty JSON array.
- **Regression evidence:** the API test asserts a fresh tenant receives a non-nil empty collection; the PostgreSQL smoke confirmed `items: []` and no cross-business products.
- **Prevention:** define empty collection response shapes in API tests and preserve them when no rows exist.

## 2026-10-31 — Discount minimum quantity used the wrong request field

- **Observed:** discount creation returned 422 because the frontend/smoke request used `minQuantity` while the typed Go request accepts `minimumQuantity`.
- **Cause:** the UI payload and domain model used different field names while strict JSON decoding rejects unknown fields.
- **Correction:** the UI now sends `minimumQuantity`; response mapping accepts the documented field and a legacy alias.
- **Regression evidence:** the PostgreSQL HTTP smoke created and consumed a fixed discount, then produced the expected post-discount tax total.
- **Prevention:** use the API contract's canonical JSON field names and cover create flows with strict-decoder integration tests.

## 2026-10-31 — Plan-only subscription default referenced a non-product ID

- **Finding:** code review showed that a subscription without explicit lines created a default line whose `productId` was the plan ID, then rejected it while validating product ownership.
- **Correction:** the default line is now plan-backed and leaves `productId` empty; quotation defaults use the same rule.
- **Regression evidence:** the isolated-schema PostgreSQL HTTP flow now creates a plan-only subscription and asserts its default line is present.
- **Prevention:** keep plan and product identifiers distinct and test optional-line defaults through the database handler.

## 2026-10-31 — Responsive billing layout clipped data and missed contrast checks

- **Observed:** the Billing Control Tower count was forced into an 18×18px box, so the count and label stacked; the mobile dashboard invoice table exceeded the viewport and clipped status/amount; several small labels were below WCAG AA contrast; closed mobile navigation remained in the accessibility tree; and three action columns had unnamed table headers.
- **Cause:** the dashboard styles assumed a desktop text layout, secondary invoice columns were not hidden at narrow widths, muted colors were reused on tinted fills without measuring the pair, and the navigation state was only visual.
- **Correction:** made the pipeline summary responsive, hid secondary invoice columns on mobile while keeping status and amount visible, darkened text for measured contrast, gave the mobile drawer explicit `aria-hidden`/`inert` state and Escape/focus-return behavior, and labeled action headers. The Reports toolbar now stacks at mobile width and each report tab displays a distinct summary.
- **Regression evidence:** `agent-browser` measured no horizontal overflow at 1440×900, 834×1112, and 390×844; dashboard Axe scans reported zero violations at each width. Desktop Axe scans across the main business routes, invoice detail, settings sections, and customer portal reported zero violations. Mobile navigation open/close and report tab/date-filter behavior were exercised.
- **Prevention:** inspect real rendered layouts at desktop/tablet/mobile, run Axe on representative routes, measure actual foreground/background pairs, and give every action column an accessible header.

## 2026-10-31 — Production Vercel build had no effective public origin

- **Observed:** the first production build stopped in `generate-mcp-discovery.mjs` because its selected origin was not HTTPS.
- **Cause:** the Vercel project had no effective production `ORCA_PUBLIC_URL`, so discovery generation fell back to the local development origin and correctly rejected it under `VERCEL_ENV=production`.
- **Correction:** configured the explicit production origin `https://orca-orcin.vercel.app` on the Vercel project and verified both the local production build and hosted OAuth discovery endpoint.
- **Prevention:** set and verify the production origin as part of Vercel project setup; keep the production guard so OAuth metadata cannot silently publish localhost URLs.
