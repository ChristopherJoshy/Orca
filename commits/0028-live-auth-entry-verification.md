# 0028 — Verify the live responsive authentication release

## Summary

- Recorded post-release verification of the responsive authentication screen from commit `354344a`.
- The canonical ORCA Vercel deployment reached `READY`; the live session loaded without an API connection error and left real API-backed sign-in fields enabled.
- Navigated the live workspace creation, password recovery, and customer portal screens without submitting credentials or creating an account.

## Files

- `docs/history.md`
- `docs/mistakes.md`
- `commits/0027-responsive-auth-entry.md`
- `commits/README.md`

## Verification

- Vercel production deployment for `354344a` — `READY`.
- Live browser at `https://orca-orcin.vercel.app/?auth=login` — sign-in fields enabled, no session error, workspace switch visible.
- Live desktop login, phone-width signup, and customer portal screen — Axe reported zero violations. Recovery and customer portal navigation opened the expected form controls.

## Limitations

No existing test account credentials were available for a login submission, and no signup was submitted. Email delivery remains unconfigured; this record verifies the hosted session and page states, not email-dependent account creation.
