# Release and hosting handoff

This phase builds and verifies locally. The user handles Vercel/VPS hosting and provider account setup; do not publish, deploy, or change production systems as part of this work.

## Local release gate

Before handoff, require the checks in [testing.md](testing.md): backend tests/race/static analysis, frontend type/lint/build, database migrations, full journey, tenant/approval/export checks, and a browser pass. Confirm `.env` values are absent from the staged diff. Record outstanding provider-backed checks and the precise required variables without exposing values.

## Owner-managed deployment checklist

When the owner is ready to deploy, provision production PostgreSQL and private object storage; configure TLS and same-origin API routing; set session key, cookie domain, allowed origins, email sender, Stripe test/live keys and webhook secret, Groq key/model, MCP OAuth issuer/client settings, export signing/storage settings, and monitoring through a secret manager. Set `ORCA_BILLING_INTERVAL` to the required catch-up cadence and keep at least one API process alive; each API process starts a billing worker, so multiple replicas coordinate with per-business database advisory locks. Use separate credentials per environment. Run reviewed migrations, create a least-privilege admin, verify backups/restore, webhook signatures, email deliverability, export privacy/expiry, and worker reconciliation before enabling recurring jobs. Start in Stripe test mode and make any live-charge decision explicitly outside this build.

No deployment has been performed by this project phase.
