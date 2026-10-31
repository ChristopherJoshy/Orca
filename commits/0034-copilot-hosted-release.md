# 0034 — Record Copilot and Invoice Truth hosted release

## Result

- Recorded production release of the Copilot chat/progress and Invoice Truth layout changes, plus the faster Groq model default.
- Updated VPS API binary from commit `3b1455a`, set its model configuration to `openai/gpt-oss-20b`, and restarted the systemd service.

## Verification

- Vercel production deployment for the pushed branch reported Ready.
- VPS `orca-api.service` is active; loopback health and the Vercel same-origin API health route returned HTTP 200 and PostgreSQL `ok`.
- Authenticated Groq requests from the VPS returned Cloudflare HTTP 403 error 1010; no live Copilot completion is claimed. Browser inspection reached sign-in, so authenticated layout review was not completed.

## Limits

- Resolve the Groq request block and complete an account-backed read-only Copilot run before describing AI responses as live in a demo.
