# 0005 — Product narrative, team credits, and backend setup guide

**Commit:** `docs: sharpen ORCA product and backend guide`

## Result

- Reworked the README around the hackathon's billing problems and ORCA's evidence-first product angle: Invoice Truth, measured integrity, and reviewed Copilot actions.
- Added a clear status/limitations section and accurately described the current on-demand billing run, provider limits, and customer/staff access.
- Added the competition team names and project/repository links based on the supplied registration screenshot and URL.
- Expanded `AGENTS.md` with backend structure, runtime configuration, session/cookie deployment constraints, and safe VPS ingress guidance.
- Completed local environment templates with Vite API proxy and export-directory settings; no secret values were added.

## Files

- `readme.md` — product positioning, implemented capabilities, team, architecture, local setup, and current limits.
- `AGENTS.md` — contributor-facing backend and environment reference.
- `.env.example`, `backend/.env.example` — documented local configuration keys.
- `docs/history.md`, `commits/README.md` — change history and record index.

## Research basis

- [Stripe subscription update documentation](https://docs.stripe.com/api/subscriptions/update) and [invoice preview API](https://docs.stripe.com/api/invoices/create_preview) establish recurring subscription, preview, and proration behavior as common billing capabilities.
- [PostgreSQL `SELECT` locking documentation](https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE) documents `SKIP LOCKED` for queue-like multi-consumer work; [constraints documentation](https://www.postgresql.org/docs/current/ddl-constraints.html) covers database-enforced uniqueness.
- These references inform ORCA's focus on visible calculation evidence and verified operational state rather than unsupported feature-count claims.

## Verification

- `git diff --check` passed.
- Manual check confirmed the README reports incomplete features as incomplete and does not claim a production deployment.
- No application code or credentials changed.

## Limitation

- Vercel routing and VPS deployment are intentionally deferred to the user's requested later hosting phase. The live competition portal requires participant login; team names were taken from the supplied screenshot.
