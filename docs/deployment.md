# Release and hosting handoff

The owner requested Vercel frontend hosting and a VPS Go API. Hosting preparation is in progress; no production deployment has been completed yet. Local verification remains the release gate.

## Local release gate

Before handoff, require the checks in [testing.md](testing.md): backend tests/race/static analysis, frontend type/lint/build, database migrations, full journey, tenant/approval/export checks, and a browser pass. Confirm `.env` values are absent from the staged diff. Record outstanding provider-backed checks and the precise required variables without exposing values.

## Owner-managed deployment checklist

The Vercel project `orca` is created and linked to this repository. It builds the Vite frontend from the repository root and publishes `frontend/dist`; the project is pinned to Node 24. `vercel.json` keeps `/api/*` and `/mcp*` same-origin while forwarding to the IP TLS endpoint through isolated `/orca-api/` and `/orca-mcp/` Nginx prefixes. Authenticated routes receive `Cache-Control: no-store, private`.

Vercel reserves `/.well-known` and does not allow those paths to be rewritten. ORCA therefore generates the MCP OAuth discovery JSON as static Vite assets during the Vercel build using `VERCEL_PROJECT_PRODUCTION_URL` (or the explicit `ORCA_PUBLIC_URL`). The API's `ORCA_PUBLIC_URL` must match that production origin exactly. The corresponding files are included in `frontend/dist/.well-known/` and are not forwarded to the VPS.

The EC2 host already has Nginx on ports 80 and 443, an IP-address TLS certificate for `3.110.118.112`, and an active Certbot renewal timer. Port 8081 is free. ORCA's systemd unit binds only to `127.0.0.1:8081`; the Nginx location snippet uses unique prefixes to avoid replacing the existing root application. The AWS ingress rules were not changed.

Before enabling the service, configure a dedicated production `DATABASE_URL` and provider credentials in `/etc/orca/orca.env` (template: [ops/orca.env.example](../ops/orca.env.example)). Set exact production `ORCA_ALLOWED_ORIGIN` and `ORCA_PUBLIC_URL`, use the HTTPS Vercel production domain, and keep export archives under `/var/lib/orca/exports`. `ORCA_EMAIL_INTERVAL` controls invoice/quotation outbox delivery. Credentials used by the Go API belong on the VPS because Vercel hosts only static frontend assets; do not put backend secrets in Vercel build variables. Use separate credentials per environment. Start Stripe in test mode and verify the webhook path before considering live-charge handling.

No deployment has been performed by this project phase.
