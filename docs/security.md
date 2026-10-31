# Security and tenancy

## Current implementation status

Business signup creates an admin. Customer signup selects a business ID and can create portal access only when that business already has a customer record with the same normalized email; production access requires the verification email. Customer session reads are scoped to that linked customer. Staff membership is admin-only and currently requires the invitee to already have a verified ORCA account.

## Identity and session

Email is normalized and globally unique. Password policy follows the source brief and expanded plan: at least 9 characters with uppercase, lowercase, and a special character; hash with Argon2id using current safe parameters and a per-password salt. Never log passwords or reset tokens. Verification and reset tokens are random, single-use, time-limited, and stored only as hashes. Responses should not reveal whether an email is registered.

Use revocable server-side sessions with unpredictable identifiers in `HttpOnly`, `Secure`, `SameSite=Lax` cookies. Rotate session IDs after authentication and privilege changes; expire idle and absolute lifetimes. Require CSRF tokens/origin checks for state-changing cookie-authenticated requests. Apply per-IP and per-account rate limits to sign-in, signup, reset, invitation, and verification endpoints. Email delivery must not disclose secrets in logs.

## Tenant and role model

- Each business owns its catalog, plans, taxes, discounts, customers, quotes, subscriptions, invoices, payments, exports, and audit history.
- Roles: `admin` (business configuration, staff, discount creation, and all business operations), `staff` (permitted day-to-day operations), and `customer` (only their customer record, portal subscriptions, invoices, and permitted self-service actions).
- Business signup creates an admin workspace. Customer signup creates customer portal access according to verified invitation/business relationship; it must not silently create internal staff membership. Only admins invite staff.
- Every query and mutation scopes by authenticated membership/business; customer queries also scope by customer ID. Avoid ID-only lookups and check object ownership before revealing existence. Apply the same policy in HTTP, jobs, agent tools, MCP tools, and exports.
- Database row-level security may provide defense in depth but does not replace explicit service checks. Tenant ID should be set transaction-locally if RLS is used.

## Agent and MCP

Model output is untrusted. Tool input is schema-validated, independently authorized, and bounded. Approval is backend state tied to actor, business, normalized exact action/input, expected resource version, expiry, and idempotency key. Recheck permissions and versions at execution. External MCP clients receive only granted scopes and read/propose access. The current OAuth implementation uses PKCE, exact redirect URI matching, state/nonce validation, short-lived authorization codes and access grants, revocation, and audit records. Refresh-token rotation is not implemented; see [the MCP contract](mcp.md).

## Data and operations

Secrets are injected via runtime environment/secret stores and are never exported. Exports and invoices are private, authorization-checked at request and download time, and use short-lived access links. Audit security-sensitive actions without recording credentials or full payment instruments. Stripe stores only provider identifiers/status required for reconciliation; never store PAN/CVC. Webhooks verify signatures before acting. Provider URLs are allowlisted to prevent SSRF. Validate upload/archive size and content if file import is ever added; archive import is out of scope.
