# 0010 — Vercel and VPS hosting preparation

## Result

- Created the dedicated Vercel project `orca` and linked this repository. Its verified settings use Vite, Node 24, `npm ci --prefix frontend`, `npm --prefix frontend run build`, and `frontend/dist`.
- Added root Vercel routing for SPA deep links, uncached same-origin `/api/*` and `/mcp*` paths, and baseline security response headers. The external rewrites target the current VPS IP through `/orca-api/` and `/orca-mcp/` prefixes so the existing root application remains routed as before.
- Added a build-time generator for static MCP OAuth discovery JSON. The metadata uses the Vercel production origin; the Go API must use that same value for `ORCA_PUBLIC_URL`.
- Added `.vercelignore`, a loopback-only systemd unit for `127.0.0.1:8081`, an Nginx location snippet, and a production VPS environment template.
- Read-only VPS inspection confirmed Nginx listeners on 80/443, a valid IP SAN TLS certificate with an active Certbot renewal timer, and port 8081 unused. No AWS security group or existing Nginx route was changed.

## Main files

- `vercel.json`, `.vercelignore`
- `frontend/scripts/generate-mcp-discovery.mjs`, `frontend/package.json`
- `ops/nginx/orca-locations.conf`, `ops/systemd/orca-api.service`, `ops/orca.env.example`
- `docs/deployment.md`, `docs/mcp.md`, `docs/history.md`

## Verification

- Frontend typecheck, lint alias, and production build passed.
- Vercel build dry-run recognized the Vite framework, ignored env files/backend/docs, and listed both `.well-known` metadata files in `frontend/dist`.
- Vercel project settings were inspected after the update.
- `git diff --check` — passed.

## Known limits

- The VPS service, Nginx snippets, and environment file have not yet been installed or activated.
- Production PostgreSQL URL and API provider credentials are not configured; the production Vercel hostname is assigned at deployment.
- Static `/.well-known` delivery still needs a live Vercel request check; the Vercel dry-run confirms inclusion in the build artifact only.
