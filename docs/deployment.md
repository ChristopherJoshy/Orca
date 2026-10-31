# Deployment status

The ORCA frontend and PostgreSQL-backed Go API are deployed. The frontend is public at [https://orca-orcin.vercel.app](https://orca-orcin.vercel.app). The API and MCP traffic use same-origin Vercel routes, so browser sessions remain on the frontend origin.

## Vercel frontend

- Project: `orca`, connected to `https://github.com/ChristopherJoshy/Orca.git`, production branch `main`.
- `orca` is the sole ORCA Vercel project. Three unused duplicate projects that were also connected to this repository have been removed; their build failures did not affect the canonical production deployment.
- Production URL: `https://orca-orcin.vercel.app`.
- Build: Vite from the repository root; `npm ci --prefix frontend`, `npm --prefix frontend run build`, output `frontend/dist`; Node 24.x.
- The production `ORCA_PUBLIC_URL` project variable is set to the canonical HTTPS origin so MCP OAuth discovery points to the live site. No API database or provider secrets are stored in Vercel.
- `vercel.json` keeps `/api/*` and `/mcp*` same-origin and forwards them to the VPS through `/orca-api/` and `/orca-mcp/`. `/.well-known/*` is served from generated static assets because Vercel reserves that path.
- Response headers mark authenticated API/MCP responses as private and uncached.

## VPS API

- Host: `ec2-user@3.110.118.112`; API systemd unit: `orca-api.service`.
- Go process binds to `127.0.0.1:8081`; PostgreSQL binds to `127.0.0.1:5432`. Neither port is exposed publicly.
- Nginx adds `/orca-api/` and `/orca-mcp/` routes inside the existing TLS server block and preserves the root app. The pre-change Nginx file is backed up at `/etc/nginx/backups/typelo-https.conf.orca`.
- The existing IP-address certificate for `3.110.118.112` covers HTTPS; the existing Certbot renewal timer remains enabled. HTTP and HTTPS ingress were already available, so no AWS security-group change or new public port was needed.
- PostgreSQL is installed on the VPS. Goose migrations 001–009 applied at startup. The API environment is root-owned with mode `0600` at `/etc/orca/orca.env`; exports use `/var/lib/orca/exports` with private permissions.
- `GROQ_API_KEY` is configured on the VPS for Copilot. Resend and Stripe keys are not configured, so email delivery and Checkout are inactive. Production workspace signup creates an admin session without email verification and shows the no-budget email notice; customer signup, password reset, and send-email APIs refuse requests that cannot complete, with a branded explanation in the UI. No production account or sample records were seeded.

## Deployment checks

- Vercel production deployment completed and the canonical URL returned HTTP 200.
- Landing-page commit `153beb5` reached `READY` in the canonical `orca` production project; the account inventory contains no other ORCA projects.
- Responsive typography/layout commit `65a00f9` reached `READY`; the live page passed Axe at desktop and phone widths, and the API and OAuth discovery endpoints remained healthy.
- Production browser review confirmed the public landing page and workspace signup navigation; Axe reported zero violations and zero incomplete checks.
- `https://orca-orcin.vercel.app/api/v1/health` returned HTTP 200 through the same-origin rewrite.
- `https://orca-orcin.vercel.app/.well-known/oauth-authorization-server` returned HTTP 200.
- Direct HTTPS health through the VPS Nginx prefix returned HTTP 200; the API reported `database: postgres`.
- `nginx -t` passed before reload. Listener inspection confirmed only loopback listeners on 8081 and 5432.
- Local production frontend build passed with `ORCA_PUBLIC_URL=https://orca-orcin.vercel.app`.

## Remaining release work

- Configure a verified Resend sender and `RESEND_API_KEY` to enable customer-portal email verification, password reset, and invoice/quotation delivery. Workspace admin signup and login do not depend on Resend.
- Configure Stripe test credentials and a webhook signing secret before testing hosted Checkout.
- Create a real test account through the normal workspace signup flow and run the full business workflow. Signup establishes an admin session without email verification; customer portal linking and password reset remain unavailable until Resend is configured. No test credentials or business data are committed.
- Configure automated PostgreSQL backups and validate export archive cleanup before relying on the VPS database for real business records.
- Complete the remaining checks in [testing.md](testing.md) and [requirements.md](requirements.md). This hackathon MVP is hosted but is not production-ready billing software.

Provider credentials used by the Go API belong on the VPS, not in Vercel variables. `ORCA_API_TARGET` is only a local Vite proxy setting. Keep `DATABASE_URL`, Groq, Stripe, and Resend secrets out of Git and out of frontend build variables.
