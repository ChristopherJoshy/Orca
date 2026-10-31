# 0008 — Simplify authentication entry copy

## Result

- Removed the redundant all-caps auth eyebrow, its decorative icon, and the tiny three-item footer from the sign-in screen.
- Kept task-relevant labels and descriptions for login, workspace signup, customer portal, and account recovery.
- Darkened the inactive account-tab label to address the agent-browser Axe contrast finding while keeping the plum/lavender palette.

## Main files

- `frontend/src/App.tsx`
- `frontend/src/styles.css`
- `docs/history.md`

## Verification

- `npm run typecheck` — passed.
- `npm run build` — passed.
- Isolated agent-browser at 1440×900, 834×1112, and 390×844 — zero horizontal overflow; Axe zero violations and zero incomplete checks at each size.
- `git diff --check` — passed.
