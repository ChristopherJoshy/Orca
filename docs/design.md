# Product design contract

## Product feel

ORCA is a finance workspace: calm, precise, spacious, and efficient. Use a light neutral canvas with restrained lavender accents, dark readable text, quiet borders, DM Sans, and tabular numerals for money. Use a consistent licensed icon set; keep assets local and record their source. Avoid decorative emoji, filler, arbitrary gradients, and generic warning banners.

The public home page uses a typography-led layout, generous white space, and a small lavender accent. It explains ORCA's actual product areas without invented customer logos, performance claims, sample business records, or demo actions. Sign in and workspace creation are available from the first screen; protected app routes keep their account entry.

## Authentication entry

Sign in and workspace creation use a centered, rounded split card on a pale lavender canvas. Keep the API-backed form on the left and a short ORCA product statement with the account-mode action on the right. Use only the owner-supplied ORCA logo at brand lockups; do not invent a replacement ORCA mark or add customer logos. Keep the logo and mode switch above the form on mobile; hide the story panel before the two-column form becomes cramped. Preserve customer access and password recovery links in the form footer. Avoid feature lists, eyebrow labels, micro-captions, or repeated marketing copy inside authentication. Workspace signup establishes a session immediately and then shows the budget notice for email-dependent features; do not imply that email was verified. API connection and validation errors stay visible and actionable; never present a disabled form as ready.

Keep public-home body copy at 16px or larger on desktop and 15px or larger on phones; short metadata may be smaller but should remain at least 10px. Use a wide content frame on large displays, then stack the hero, navigation, and principle rail before tablet widths make them cramped.

## Color roles and contrast

Keep the canvas light (`#f7f6f9`) and surfaces white. Use `#262433` for primary text, `#6f6d7b` for secondary labels, and the deeper lavender `#6e50b3` for links, eyebrow labels, and actions. Use pale lavender only for fills. Status text uses semantic dark colors over light tints: green `#286449` on `#eaf5ef` (6.25:1), amber `#875c0d` on `#fbf3df` (5.32:1), red `#b04750` on `#fbecee` (4.75:1), and blue `#3c5d88` on `#edf3fa` (6.04:1). Muted text measures 4.70:1 on the canvas and 5.06:1 on white; lavender measures 5.64:1 on the canvas and 6.07:1 on white. These values exceed the 4.5:1 WCAG AA minimum for normal text; calculate contrast from the actual foreground/background pair and don't rely on hue alone. See [W3C contrast guidance](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum).

## Navigation and page map

Business workspace navigation: Overview, Customers, Catalog, Plans, Quotations, Subscriptions, Invoices, Payments, Reports, and Settings. Settings groups business profile, taxes, discounts, staff, data exports, and integrations. A persistent Copilot entry opens task conversation and approval detail. Customer portal contains only the customer's subscriptions, invoices, payment history, and allowed self-service controls.

## Interaction rules

- Pages make the current workspace and time/filter range visible. Money and dates use business locale, with `INR` explicit where ambiguity is possible.
- Tables have server-backed filters, sorting, pagination, useful empty/loading/error states, and links to detail records. Keep row actions discoverable and keyboard accessible.
- Create/edit flows validate locally for guidance and again on the server. Errors identify the field and recovery step. Confirm consequential actions with exact target, amount, and effect.
- Invoices show itemized stored calculations and an explainable tax/discount breakdown. Preserve workflow status separately from balance/overdue indicators.
- Agent progress names the step and outcome; approval cards bind to exact inputs and link affected records. Provider unavailability leaves ordinary work accessible.
- Responsive at desktop, tablet, and mobile widths. Use semantic headings, labels, focus order, visible focus, accessible names, and sufficient contrast.

## Browser evidence

On 2026-10-31, `agent-browser` scanned all 17 application routes at 1440×900, 834×1112, and 390×844. Each route/viewport scan reported zero Axe violations and no horizontal overflow. The report tabs/date filters and mobile drawer Escape-close/focus-return interaction were also exercised. Full keyboard-only and role-based workflow acceptance remains open; see [the acceptance matrix](requirements.md) and [verification history](history.md).

After owner feedback, the public home page's type scale and content width were increased and the tablet layout was adjusted. `agent-browser` checked 2048×1168, 1440×900, 1024×768, 900×900, 834×1112, 768×1024, 390×844, 375×812, and 320×740. Each width matched document scroll width; the tablet header and principle rail no longer overlap or clip. Create-workspace navigation and return-to-home worked. Axe reported zero violations and zero incomplete checks on the local build and the live site at 1920×1080 and 390×844.
