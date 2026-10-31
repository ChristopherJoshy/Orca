# Verified mistakes and prevention

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
