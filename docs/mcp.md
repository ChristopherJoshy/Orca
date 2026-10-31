# External Model Context Protocol

ORCA exposes an HTTPS Streamable HTTP MCP endpoint at `/mcp`, implemented with the official Go SDK. OAuth discovery, dynamic client registration, authorization code with S256 PKCE, ORCA consent, scoped bearer tokens, and revocation are served by the API. Set `ORCA_PUBLIC_URL` to the exact public origin that proxies these routes. Metadata and registration require HTTPS except for loopback development URLs.

## Current implementation and limits

The server is mounted by `API.RegisterMCPRoutes`. MCP clients are bound to the granting user and business. Every token use rechecks expiry, revocation, and current membership; record tools also enforce customer ownership. Access tokens are stored as hashes and expire after one hour. No refresh tokens are issued.

Dynamic client registrations and pending authorization state/code live in process memory. A server restart requires MCP clients to register and authorize again. Persisted access grants, scopes, audits, and revocations remain in the database. This is an implementation limit of the current version.

The current tools are `search_records`, `explain_invoice`, `get_report_summary`, `preview_next_invoice`, and `request_action_review`. These share the same scoped backend tools as Copilot. External clients can read, explain, simulate, and propose supported actions; they cannot execute a mutation. An action proposal remains pending until a user reviews it in ORCA. ORCA's approval endpoint performs its own permission, input-hash, and record-version checks before executing a supported action.

Explicit export-request and separate integrity-check tools are not implemented. Integrity values are included in the staff-only report summary. No external AI application has been compatibility-certified; automated tests use the official SDK's Streamable HTTP client path.

## Authorization and scope

OAuth scopes are:

| Scope | Capability |
|---|---|
| `records:read` | Search records and explain stored invoice evidence |
| `reports:read` | Read business summary and integrity indicators |
| `billing:simulate` | Preview the next invoice without writes |
| `actions:propose` | Request a supported ORCA review; never execute directly |

Consent is a user action inside ORCA. Redirect URIs are matched exactly, non-loopback HTTP is rejected, and authorization codes are single-use and short-lived. Bearer tokens are returned with no-store headers. Revoking a grant blocks subsequent requests. Grant creation and revocation are audited.

Tool input is typed, size-limited, and validated by the shared domain layer. No tool exposes SQL, secrets, or worker controls. Customers remain restricted to their own portal records. Authorization errors avoid confirming records outside the actor's scope.

## Local setup and management

For local development, set `ORCA_PUBLIC_URL=http://localhost:5173` and use the same-origin API proxy. The OAuth protected-resource metadata is at `/.well-known/oauth-protected-resource/mcp`; authorization-server metadata is at `/.well-known/oauth-authorization-server`. The consent page requires an authenticated ORCA session. Settings currently exposes connection metadata and grant revocation; copyable client setup instructions are still to be added.

Before connecting a production MCP client, configure an HTTPS public URL and verify that its OAuth discovery, PKCE, and Streamable HTTP behavior works against ORCA. Automated tests cover registration, discovery, redirect matching, PKCE, scope denial, client/tool round-trip, tenant ownership, expiry, membership revocation, and bearer revocation. No deployment or real third-party client verification is included in this build.
