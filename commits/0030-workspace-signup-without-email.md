# 0030 — Allow workspace access without email delivery

## Result

- Business workspace signup now creates an admin account, business membership, and authenticated session without Resend. The production user record stays `verified: false`; the frontend displays the no-budget email notice and continues into the new workspace.
- Admin/staff login works without email verification. Unverified customer memberships remain denied; customer portal linking, password reset, and document delivery still require email service configuration.
- Staff membership is granted using only the email of an existing ORCA account. The API continues to enforce workspace-admin authorization and records an audit event; no invitation email is sent.
- Replaced the invented ORCA mark with the owner-supplied logo in the public homepage, auth screens, app shell, and README. Added the brand and email-only staff-entry rules to contributor guidance.
- Updated OpenAPI, security, integration, MCP, deployment, requirements, README, screenshot, and design documentation.

## Verification

- `GOCACHE=/tmp/orca-go-cache go test ./...` — passed.
- `npm run typecheck` — passed.
- `npm run lint` — passed (the configured lint script runs TypeScript checking).
- `ORCA_PUBLIC_URL=https://orca-orcin.vercel.app VERCEL_ENV=production npm run build` — passed.
- OpenAPI YAML parsed; `git diff --check` passed.

## Limits

- Vercel and VPS deployment, live account creation, and the account-backed billing workflow still need post-push verification.
- Workspace email addresses are not verified in this build. Admins must confirm the intended staff member before granting access; password recovery remains unavailable without Resend.
