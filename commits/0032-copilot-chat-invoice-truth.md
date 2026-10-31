# 0032 — Restore Copilot run progress and clarify Invoice Truth

## Result

- Kept Copilot runs visibly in progress until the API records a terminal result. The chat restores recorded tool calls/results after refresh, displays colored tool-specific progress, offers cancellation, and uses the actual Groq configuration state from the run-list API.
- Reworked the conversation surface to use more of the available desktop width and keep a larger chat composer. The approval/tool rail stacks below the chat on narrow screens.
- Separated invoice identity, customer, date, balance, allocation, and calculation metadata into readable layouts; long canonical hashes wrap without merging into labels.
- Updated the Copilot, design, API, requirements, history, and mistakes documentation. Added a handler test for provider readiness and secret omission.

## Verification

- `go test ./...`, `go test -race ./...`, and `go vet ./...` — passed.
- Frontend `npm run typecheck`, `npm run lint`, and production `npm run build` with the production ORCA origin — passed.
- OpenAPI YAML parsing and `git diff --check` — passed.

## Limits

- The deployed Groq tool loop and changed layouts still need live browser/account acceptance. In-flight runs remain process-bound and are not recovered after an API restart.
