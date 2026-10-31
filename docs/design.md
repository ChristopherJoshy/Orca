# Product design contract

## Product feel

ORCA is a finance workspace: calm, precise, spacious, and efficient. Use a light neutral canvas with restrained lavender accents, dark readable text, quiet borders, DM Sans, and tabular numerals for money. Use a consistent licensed icon set; keep assets local and record their source. Avoid decorative emoji, filler, arbitrary gradients, and generic warning banners.

## Navigation and page map

Business workspace navigation: Overview, Customers, Catalog, Plans, Quotations, Subscriptions, Invoices, Payments, Reports, and Settings. Settings groups business profile, taxes, discounts, staff, data exports, and integrations. A persistent Copilot entry opens task conversation and approval detail. Customer portal contains only the customer's subscriptions, invoices, payment history, and allowed self-service controls.

## Interaction rules

- Pages make the current workspace and time/filter range visible. Money and dates use business locale, with `INR` explicit where ambiguity is possible.
- Tables have server-backed filters, sorting, pagination, useful empty/loading/error states, and links to detail records. Keep row actions discoverable and keyboard accessible.
- Create/edit flows validate locally for guidance and again on the server. Errors identify the field and recovery step. Confirm consequential actions with exact target, amount, and effect.
- Invoices show itemized stored calculations and an explainable tax/discount breakdown. Preserve workflow status separately from balance/overdue indicators.
- Agent progress names the step and outcome; approval cards bind to exact inputs and link affected records. Provider unavailability leaves ordinary work accessible.
- Responsive at desktop, tablet, and mobile widths. Use semantic headings, labels, focus order, visible focus, accessible names, and sufficient contrast.
