# 0004 — Verified team access across workspaces

**Commit:** `fix: add verified staff across workspaces`

## Result

- Staff creation now looks up an existing verified ORCA account across businesses, while rechecking that the requesting user is still an active administrator of the destination business inside the transaction.
- Staff access is created only for the current business; the user’s memberships in other businesses are unchanged.
- The action is recorded in the audit log with a UUID resource identifier accepted by PostgreSQL.
- Unverified accounts are rejected and receive no staff access.

## Files

- `backend/internal/orca/crud_handlers.go` — globally scoped user lookup with transactional destination-admin authorization and staff audit event.
- `backend/internal/orca/staff_handlers_test.go` — cross-business isolation, audit identifier, and unverified-account tests.
- `docs/history.md`, `docs/mistakes.md`, and `commits/README.md` — implementation and regression record.

## Verification

- `make test` passed, including Go package tests.
- Local PostgreSQL browser flow added an existing verified account to the current workspace: API returned `201`, and a reload showed the active Staff row.
- `git diff --check` passed before commit.

## Limitation

- Team access currently requires the teammate to create and verify an ORCA account first; this action does not send an invitation email.
