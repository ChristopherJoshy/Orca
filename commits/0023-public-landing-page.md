# 0023 — Publish the public ORCA landing page

## Summary

Added an elegant public home page for signed-out visitors, based on the supplied minimal infrastructure-site references. Direct sign-in and workspace creation remain visible. Product claims describe implemented ORCA workflows; the page has no demo CTA, fabricated logos, or invented scale metrics. Removed three unused duplicate Vercel projects after confirming their deployment misconfiguration, retaining the canonical `orca` project.

## Changes

- Added a responsive public landing component and original billing workflow illustration.
- Connected the homepage CTA and auth modes to direct signup/login URLs while preserving protected-route authentication.
- Added responsive visual styles and updated the public design contract.
- Recorded the actual accessibility and Vercel project configuration issues, fixes, and prevention steps.
- Removed unused projects `orca-wich`, `chrisorca`, and `frontend-bwk1`; retained `orca` and `orca-orcin.vercel.app`.

## Verification

- Frontend typecheck, lint alias, and production build passed using `ORCA_PUBLIC_URL=https://orca-orcin.vercel.app` and `VERCEL_ENV=production`.
- `agent-browser` checked 1440×1000 and 390×844. Workspace signup and return-to-home navigation worked; mobile scroll width was 390 px; Axe reported zero violations and zero incomplete checks at both sizes.
- The canonical Vercel deployment for commit `7c5d301` and its API health/OAuth discovery endpoints returned ready/HTTP 200 before this landing-page push.
- `git diff --check` passed.

## Limitations

- The post-push canonical deployment and live browser checks are recorded in [commit record 0024](0024-live-public-landing-release.md).
- Production account creation remains unavailable until a verified Resend sender and key are configured; the API rejects signup before writing an unverified account.
