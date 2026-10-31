# 0027 — Refine the responsive ORCA sign-in entry

## Summary

- Replaced the crowded full-height auth story and duplicate mode tabs with a centered split card using ORCA's lavender palette.
- Kept the sign-in form on the left and a concise product statement plus a single workspace-mode action on the right. On tablet and phone widths, the story panel collapses and the ORCA lockup retains the mode switch.
- Preserved the API-backed login, workspace creation, customer portal, and password recovery paths. No demo records or client-side auth fallback were added.
- Corrected the page heading hierarchy and small caption contrast; made the password placeholder readable at 320px and spaced API status feedback from the fields.

## Files

- `frontend/src/App.tsx`
- `frontend/src/styles.css`
- `docs/design.md`
- `docs/history.md`
- `docs/mistakes.md`

## Verification

- `npm run typecheck` — passed.
- `npm run lint` — passed (the repository lint script delegates to TypeScript checking).
- `ORCA_PUBLIC_URL=https://orca-orcin.vercel.app VERCEL_ENV=production npm run build` — passed.
- `git diff --check` — passed.
- `agent-browser` review at 1440×960, 1024×768, 768×1024, 390×844, and 320×740 — no horizontal overflow. Axe: zero violations at desktop and 390×844. Workspace mode switch opened the signup form.

## Limitations

The static local preview's auth-session request returned HTTP 500 and disabled form submission. The deployed auth session was subsequently verified without a connection error, but no login or signup was submitted and no test account was created. Email-dependent signup still requires configured email delivery.
