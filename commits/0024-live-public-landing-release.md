# 0024 — Verify the public homepage production release

## Summary

Recorded the release and production checks for the public landing page published by commit `153beb5`. Confirmed the deployment, same-origin API, OAuth discovery, and canonical Vercel project inventory.

## Changes

- Updated deployment and verification history with the canonical Vercel production result.
- Linked the landing-page implementation record to this release verification record.
- Confirmed the Vercel account has one ORCA project (`orca`); unrelated projects remain untouched.

## Verification

- `vercel list orca --json -n 8` reports the production deployment for commit `153beb58c644c7b5645c0ee3ab468d0293a9ec9b` as `READY`.
- Production homepage, `/api/v1/health`, and `/.well-known/oauth-authorization-server` returned HTTP 200. The health response reported PostgreSQL `ok`.
- `agent-browser` rendered the live homepage and checked the create-workspace route and return-to-home action without submitting the form. Axe reported zero violations and zero incomplete checks.
- Vercel project inventory listed `orca` as the only ORCA-related project.
- `git diff --check` passed.

## Limitations

- Production signup requires a configured verified Resend sender. Signup currently returns 503 before creating a user, so no production account was created.
- Live Stripe checkout and email delivery remain unavailable until their provider credentials are configured.
