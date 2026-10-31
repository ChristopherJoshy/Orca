# 0033 — Default Copilot to Groq GPT-OSS 20B

## Result

- Set `openai/gpt-oss-20b` as the backend fallback and configured default across environment examples, README, and the existing ignored local `.env` model setting.
- Kept `GROQ_MODEL` configurable, including `openai/gpt-oss-120b` for workflows that prefer its higher reasoning capability.
- Documented the model decision against Groq's current published production catalog and tool-use support: GPT-OSS 20B is listed at approximately 1,000 tokens/second with a 131,072-token context and tool calling. ORCA's current serial tool loop does not need parallel tool calls.
- Added a Go test that verifies the fallback model sent by the tool loop.

## Verification

- `go test ./...` and `go vet ./...` — passed.
- Groq authenticated probes from the workstation environment and VPS received Cloudflare HTTP 403 error 1010. The model was not live-verified; the error does not establish that the configured key is invalid.
- Groq model catalog and tool-use support were checked against official documentation on 2026-10-31.

## Limits

- The Copilot provider cannot be confirmed end to end until credentialed calls can pass the current Cloudflare block.
