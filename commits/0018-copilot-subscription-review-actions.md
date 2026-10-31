# 0018 — Add ORCA-reviewed Copilot subscription lifecycle proposals

## Result

- Copilot and external MCP can propose plan-permitted subscription pause, resume, and renewal actions.
- ORCA approval rechecks proposer membership, current subscription state, plan permission, exact stored inputs, and resource version before execution.
- Renewal binds the business-local start date into the approval. External MCP remains read-and-propose only; the mutation executes only inside ORCA approval.
- Updated tool schemas, action descriptions, tests, and the product-agent action matrix.

## Verification

- `make verify` — passed against the integrated working tree.
- `git diff --check` — passed.

## Known limits

- Full Copilot page-action coverage, restart-safe run recovery, and live Groq tool execution remain open.
