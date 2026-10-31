# 0021 — Deploy the Vercel frontend and VPS API

## Result

- Linked Vercel project `orca` to `ChristopherJoshy/Orca`, configured the production-only `ORCA_PUBLIC_URL`, and deployed the public frontend at `https://orca-orcin.vercel.app`.
- Installed the Go API as `orca-api.service` behind the existing Nginx TLS host. `/orca-api/` and `/orca-mcp/` route to the service on `127.0.0.1:8081`; PostgreSQL listens only on `127.0.0.1:5432`.
- Applied Goose migrations 001–009 and stored the generated database credential and configured Groq key only in `/etc/orca/orca.env` with mode `0600`. No backend secret was added to Vercel or Git.
- Updated the README, deployment guide, history, and verified-mistakes log with the live URL and actual deployment state.

## Verification

- Production Vite build — passed with the canonical HTTPS origin.
- Vercel production deployment — `READY`; public frontend returned HTTP 200.
- VPS direct HTTPS API health — HTTP 200; API reported PostgreSQL.
- Vercel same-origin API health and OAuth discovery — HTTP 200.
- Goose application migrations 001–009 — applied successfully.
- Nginx configuration test — passed; reload completed.
- Listener check — API `127.0.0.1:8081`, PostgreSQL `127.0.0.1:5432`; no new AWS port was required.
- `git diff --check` — passed.

## Known limits

- Resend and Stripe are not configured, so provider email and hosted Checkout remain inactive.
- No production test account or sample business data was created; account-backed acceptance still needs a real mailbox and normal signup verification.
- Automated PostgreSQL backups have not been configured. The deployment is a hosted hackathon MVP, not production-ready billing infrastructure.
