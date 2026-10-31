# 0029 — Remove authentication micro-labels

## Summary

- Removed the “Billing workspace” caption and “Subscriptions · recurring billing” eyebrow from the login story panel, along with the divider and eyebrow dot.
- Kept the ORCA mark, main billing statement, supporting sentence, and workspace switch.

## Files

- `frontend/src/App.tsx`
- `frontend/src/styles.css`
- `docs/design.md`
- `docs/history.md`
- `docs/mistakes.md`

## Verification

- Not run, at the user's request.

## Limitations

- The previous auth release's responsive and live-page checks are recorded in `commits/0027-responsive-auth-entry.md` and `commits/0028-live-auth-entry-verification.md`; this small copy removal did not receive a new browser review.
