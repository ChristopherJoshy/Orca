# 0020 — Move MCP cache policy into Vercel headers config

## Result

- Removed a `headers` property from the `/mcp` rewrite, which Vercel rejects.
- Added the `/mcp` root cache policy under the top-level `headers` array alongside `/mcp/:path*`, preserving private uncached OAuth/MCP responses.

## Verification

- `node` JSON parse of `vercel.json` — passed.
- `git diff --check` — passed.
- `vercel deploy --dry` — passed; Vercel detected Vite and inspected the project without uploading or creating a deployment.

## Known limits

- No Vercel deployment was created; the owner will deploy after the VPS API is ready.
