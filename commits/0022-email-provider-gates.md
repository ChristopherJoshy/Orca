# 0022 — Fail closed when email delivery is unavailable

## Result

- Added an ORCA-styled explanation for email-dependent UI actions. Invoice and quotation actions do not call the API; signup, customer access, and reset show the same explanation when the API reports no provider.
- Added server-side provider gates for account signup, customer signup, password reset, invoice and quotation sends, Copilot email proposals, and approval execution. Missing Resend credentials or sender produces HTTP 503 before account, business, token, document, approval, or outbox mutation.
- Corrected provider status and updated OpenAPI, integration, agent, deployment, acceptance, README, history, and verified-mistakes documentation.
- No test account or fake business data was created in production. A usable production account requires a verified sender so signup can deliver the required link.

## Verification

- `ORCA_PUBLIC_URL=https://orca-orcin.vercel.app VERCEL_ENV=production make verify` — passed: Go tests, race detector, `go vet`, frontend typecheck, current lint alias (TypeScript typecheck), and Vite production build.
- OpenAPI YAML parsed — passed.
- `git diff --check` — passed.
- Production API and database non-mutation checks remain pending deployment.

## Known limits

- Signup, customer portal linking, and password recovery remain unavailable until a verified Resend sender and API key are configured.
- This batch does not enable Stripe Checkout; the webhook signing secret is still needed before safely activating the Stripe flow.
