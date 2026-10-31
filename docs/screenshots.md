# ORCA screenshots

Every screen below was captured from a local development build at 1440 px wide (390 px for mobile), using a disposable workspace and illustrative records. No customer data is included.

The gallery follows the product workflow: sign in, run the workspace, sell, bill, collect, automate, administer, and hand the customer their own view.

**Jump to:** [Start here](#start-here) · [Daily workspace](#daily-workspace) · [Sell](#sell) · [Bill and collect](#bill-and-collect) · [Automate](#automate) · [Admin](#admin) · [Customer experience](#customer-experience) · [Mobile](#mobile)

## Start here

<a href="assets/screenshots/sign-in.webp"><img src="assets/screenshots/sign-in.webp" alt="ORCA sign-in screen with product story, sign-in form, create-workspace action, and customer portal link" width="100%" /></a>

The public entry point. Sign in, create a workspace, or switch to the customer portal. There is no demo workspace, sample business, or browser-local fallback behind this screen.

## Daily workspace

<a href="assets/screenshots/overview.webp"><img src="assets/screenshots/overview.webp" alt="ORCA overview dashboard with revenue, collections, active subscriptions, receivables, billing health, recent invoices, and renewals" width="100%" /></a>

**Overview.** Monthly recurring revenue, collections this month, active subscriptions, open receivables, billing pipeline health, recent invoices, and renewals that need attention.

<table>
  <tr>
    <td width="50%" valign="top">
      <a href="assets/screenshots/reports.webp"><img src="assets/screenshots/reports.webp" alt="ORCA reports page with revenue, invoicing, collections, and receivables views, date filters, metrics, and integrity checks" width="100%" /></a>
      <br />
      <sub><strong>Reports.</strong> Revenue, invoicing, collections, and receivables each use the correct date basis. Invoice reports use issue dates; collections use payment dates. Export the selected report as CSV.</sub>
    </td>
    <td width="50%" valign="top">
      <a href="assets/screenshots/copilot.webp"><img src="assets/screenshots/copilot.webp" alt="ORCA Copilot workspace with suggested prompts, connected workspace status, and human review enabled" width="100%" /></a>
      <br />
      <sub><strong>Copilot.</strong> Search records, explain an invoice, preview the next bill, or prepare a supported action. A proposal executes only after an ORCA approval rechecks access and record versions.</sub>
    </td>
  </tr>
</table>

## Sell

<table>
  <tr>
    <td width="50%" valign="top">
      <a href="assets/screenshots/customers.webp"><img src="assets/screenshots/customers.webp" alt="ORCA customer roster with subscription counts, status, and open balances" width="100%" /></a>
      <br />
      <sub><strong>Customers.</strong> Each relationship keeps its subscriptions and open balance together. Customer records never cross a business boundary.</sub>
    </td>
    <td width="50%" valign="top">
      <a href="assets/screenshots/catalog.webp"><img src="assets/screenshots/catalog.webp" alt="ORCA catalog with products, variants, prices, margins, tax rates, and archive actions" width="100%" /></a>
      <br />
      <sub><strong>Catalog.</strong> Products and services with optional variants, sales and cost prices, margin, tax rate, and soft archive. Archived terms stay on existing subscriptions.</sub>
    </td>
  </tr>
  <tr>
    <td width="50%" valign="top">
      <a href="assets/screenshots/plans.webp"><img src="assets/screenshots/plans.webp" alt="ORCA plans showing monthly and yearly billing, prices, tax, minimum seats, and lifecycle permissions" width="100%" /></a>
      <br />
      <sub><strong>Plans.</strong> Billing frequency, price, minimum seats, tax, and the lifecycle rules a plan allows: pause, early close, renew, or auto-close at term end.</sub>
    </td>
    <td width="50%" valign="top">
      <a href="assets/screenshots/quotations.webp"><img src="assets/screenshots/quotations.webp" alt="ORCA quotations with reusable templates, open and awaiting-response counts, validity, and offer totals" width="100%" /></a>
      <br />
      <sub><strong>Quotations.</strong> Reusable templates and customer offers with visible validity, status, seats, and totals. A customer accepts the offer from the portal.</sub>
    </td>
  </tr>
</table>

<a href="assets/screenshots/subscriptions.webp"><img src="assets/screenshots/subscriptions.webp" alt="ORCA subscriptions with lifecycle counts and per-row plan, seats, start date, status, recurring amount, and allowed actions" width="100%" /></a>

**Subscriptions.** Draft → quotation → confirmed → active → paused or closed. Each row shows the plan, seats, start date, recurring amount, and only the actions that are valid for its current state and plan policy.

## Bill and collect

<a href="assets/screenshots/invoices.webp"><img src="assets/screenshots/invoices.webp" alt="ORCA invoice register with billing periods, due dates, statuses, balances, totals, and Invoice Truth links" width="100%" /></a>

**Invoices.** The invoice register keeps the billing period, due date, status, outstanding balance, and total visible, with a direct route into the stored evidence for every invoice.

<a href="assets/screenshots/invoice-truth.webp"><img src="assets/screenshots/invoice-truth.webp" alt="ORCA Invoice Truth page with line items, discount, tax, total, payment allocations, audit trail, and stored calculation snapshot" width="100%" /></a>

**Invoice Truth.** Line descriptions, quantities, unit prices, tax, subtotal, discount, tax after discount, amount due, payment allocations, and an audit trail. The calculation snapshot records the exact stored inputs, the engine version, and a canonical hash. The browser does not recompute the authoritative total.

<a href="assets/screenshots/payments.webp"><img src="assets/screenshots/payments.webp" alt="ORCA payments page with recorded payments, allocated totals, methods, dates, notes, and invoice allocations" width="100%" /></a>

**Payments.** Manual and provider payments are reconciled against invoices, with method, date, note, and allocated amount. Partial payments leave the remaining balance visible. A Stripe browser return never marks an invoice paid; only a verified, idempotent webhook records payment.

## Automate

The Copilot screen above is the built-in assistant. External MCP clients use the same typed domain tools through OAuth discovery, PKCE, scoped grants, and revocation in **Settings → External MCP**. Both paths are read/propose by default; neither gets unrestricted SQL access, and neither bypasses approval.

## Admin

<table>
  <tr>
    <td width="50%" valign="top">
      <a href="assets/screenshots/settings.webp"><img src="assets/screenshots/settings.webp" alt="ORCA settings with business profile, taxes, discounts, team, exports, MCP connection instructions, provider status, and available scopes" width="100%" /></a>
      <br />
      <sub><strong>Settings.</strong> Business profile, taxes, discounts, team access, exports, provider status, and the MCP OAuth endpoint with discovery metadata, scopes, and authorized connections.</sub>
    </td>
    <td width="50%" valign="top">
      <a href="assets/screenshots/discounts.webp"><img src="assets/screenshots/discounts.webp" alt="ORCA discounts page with fixed and percentage rules, scope, minimum purchase, maximum uses, and validity" width="100%" /></a>
      <br />
      <sub><strong>Discounts.</strong> Fixed or percentage rules with product or subscription scope, minimum purchase and quantity, maximum uses, and a validity window. Eligibility is applied before tax.</sub>
    </td>
  </tr>
  <tr>
    <td width="50%" valign="top">
      <a href="assets/screenshots/taxes.webp"><img src="assets/screenshots/taxes.webp" alt="ORCA taxes page showing the fixed discount-before-tax calculation order and a tax rule table" width="100%" /></a>
      <br />
      <sub><strong>Taxes.</strong> The calculation order is fixed: each line applies its eligible discount first, calculates tax on the reduced amount, then rounds tax half-up per line.</sub>
    </td>
    <td width="50%" valign="top">
      <a href="assets/screenshots/team.webp"><img src="assets/screenshots/team.webp" alt="ORCA team members page with admin-only access notice, roles, and account status" width="100%" /></a>
      <br />
      <sub><strong>Team members.</strong> Admin-only staff access for existing ORCA accounts. Customer portal access stays separate from employee access, and no invitation email is implied.</sub>
    </td>
  </tr>
</table>

<a href="assets/screenshots/exports.webp"><img src="assets/screenshots/exports.webp" alt="ORCA data exports page with private archive scope, included record types, and export history" width="100%" /></a>

**Data exports.** A full business archive brings related records, stored invoice calculations, payment allocations, a manifest, and checksums. Exports are private to authorized workspace members; secrets and sessions are excluded.

## Customer experience

<a href="assets/screenshots/customer-portal.webp"><img src="assets/screenshots/customer-portal.webp" alt="ORCA customer portal with a pending offer, the customer's subscription, invoices, balances, and online payment actions" width="100%" /></a>

**Customer portal.** A signed-in customer sees only their own account: offers to accept, subscriptions to pause when the plan allows it, invoices with balances, and online payment where configured. Staff and customer sessions are separate.

## Mobile

<table>
  <tr>
    <td width="50%" valign="top">
      <a href="assets/screenshots/mobile-overview.webp"><img src="assets/screenshots/mobile-overview.webp" alt="ORCA overview on a 390 by 844 mobile viewport" width="100%" /></a>
      <br />
      <sub><strong>Workspace on mobile.</strong> The same records and actions at 390 × 844, with no horizontal overflow.</sub>
    </td>
    <td width="50%" valign="top">
      <a href="assets/screenshots/mobile-customer-portal.webp"><img src="assets/screenshots/mobile-customer-portal.webp" alt="ORCA customer portal on a 390 by 844 mobile viewport" width="100%" /></a>
      <br />
      <sub><strong>Portal on mobile.</strong> The customer view stays usable on a small screen.</sub>
    </td>
  </tr>
</table>

---

These captures show the current development UI, including work-in-progress copy in a few places. The provider rows in Settings reflect local configuration, not proof of live Stripe, Resend, or Groq delivery. See the [requirements matrix](requirements.md) for what is verified and what remains open.
