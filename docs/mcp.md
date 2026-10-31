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

Use **Settings → External MCP** to copy the Streamable HTTP endpoint and view the current OAuth grants. In an MCP client that supports remote Streamable HTTP and OAuth discovery, add the server URL shown there. The client discovers the authorization endpoints, registers through dynamic client registration, and starts the PKCE flow. ORCA opens the signed-in consent page; review the requested scopes there. No ORCA password, API key, or bearer token should be pasted into a client configuration.

The protected-resource metadata is at `/.well-known/oauth-protected-resource/mcp`; authorization-server metadata is at `/.well-known/oauth-authorization-server`. For local development, use the Vite same-origin proxy and set `ORCA_PUBLIC_URL` to the exact browser origin (for example, `http://localhost:5173`). The browser origin, copied `/mcp` URL, discovery metadata, and configured public origin must agree.

Vercel reserves `/.well-known` and does not allow those paths to be rewritten. ORCA generates the two discovery documents into the static Vite output at build time using `VERCEL_PROJECT_PRODUCTION_URL` (or an explicit `ORCA_PUBLIC_URL`). Vercel rewrites `/mcp*` to the Go API through the VPS Nginx prefix, while the static discovery files remain on the Vercel origin. Set the API's `ORCA_PUBLIC_URL` to the same production origin so OAuth issuer, resource metadata, callbacks, and session cookies stay aligned. Preview origins are not the production MCP origin.

Settings lists grant metadata created by actual OAuth consent: client ID, approved scopes, expiry, and status. It never returns access tokens. A workspace admin can revoke an active grant from this list; the backend records the revocation and subsequent requests fail. MCP grants are user- and business-bound, and current membership is checked for each request. The settings interface does not manufacture grants; an external client must complete OAuth first.

Available scopes are `records:read`, `reports:read`, `billing:simulate`, and `actions:propose`. External clients can read, explain, simulate, and propose supported actions; a proposed mutation still requires an ORCA review. Connection registration and authorization codes remain process-local, so clients may need to reconnect after an API restart. Client compatibility has not been certified beyond the official SDK test client.

Team access is managed separately under **Settings → Team members**. The teammate first creates and verifies an ORCA account. A workspace admin can then add that account by its verified email. This action creates an active Staff membership and does not send an invitation email.

Before connecting a production MCP client, configure an HTTPS public URL and verify that its OAuth discovery, PKCE, and Streamable HTTP behavior works against ORCA. Automated tests cover registration, discovery, redirect matching, PKCE, scope denial, client/tool round-trip, tenant ownership, expiry, membership revocation, and bearer revocation. No deployment or real third-party client verification is included in this build.
