# ORCA documentation

These documents define the product and its implementation contracts. They do not prove that a feature is implemented; use the status/evidence column in [requirements.md](requirements.md) for that.

| Document | Purpose |
|---|---|
| [requirements.md](requirements.md) | Source requirements mapped to acceptance evidence and current status |
| [architecture.md](architecture.md) | Services, boundaries, persistence, and execution model |
| [billing.md](billing.md) | Exact arithmetic, periods, invoice snapshots, ledger, and retries |
| [security.md](security.md) | Identity, tenant isolation, roles, sessions, and secrets |
| [design.md](design.md) | Interaction and visual design contract |
| [development.md](development.md) | Local setup, module ownership, commands, and contribution flow |
| [testing.md](testing.md) | Required unit, integration, scale, security, and end-to-end evidence |
| [deployment.md](deployment.md) | Local release readiness and owner-managed hosting handoff |
| [integrations.md](integrations.md) | Stripe, email, and Groq provider contracts and failure handling |
| [exports.md](exports.md) | Business and filtered export formats, privacy, and lifecycle |
| [agents.md](agents.md) | Built-in Copilot tools, run persistence, approvals, and recovery |
| [mcp.md](mcp.md) | External Streamable HTTP MCP, authorization, scopes, and proposals |
| [demo.md](demo.md) | Local account-backed end-to-end test journey and expected outcomes |
| [screenshots.md](screenshots.md) | Every workspace, portal, and mobile screen with current UI context |
| [history.md](history.md) | Dated change and verification log |
| [mistakes.md](mistakes.md) | Verified mistakes, corrections, and prevention |
| [api/openapi.yaml](api/openapi.yaml) | Versioned REST contract |
| [decisions/0001-stack.md](decisions/0001-stack.md) | Initial stack and boundary decisions |
| [references/requirements-source.md](references/requirements-source.md) | Source document provenance and interpreted constraints |

Contributor instructions live in [AGENTS.md](../AGENTS.md); the product agent's runtime policy lives in [agents.md](agents.md). Update the owning contract when behavior changes.
