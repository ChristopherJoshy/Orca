# 0019 — Document owner-approved VPS deployment access

## Result

- Recorded the VPS target and the external SSH key location in the contributor guide so the authorized deployment workflow is reproducible.
- Kept private-key contents and provider secrets outside the repository.
- Clarified that remote deployment requires an explicit owner-requested hosting phase and exact target credentials.

## Verification

- `git diff --check` — passed.
- Confirmed `.env` and `.env.local` remain ignored by Git.

## Known limits

- This record documents the deployment access boundary; the actual VPS setup is tracked separately in the deployment handoff.
