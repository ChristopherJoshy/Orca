import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const configuredOrigin = process.env.ORCA_PUBLIC_URL?.trim();
const systemDomain = process.env.VERCEL_PROJECT_PRODUCTION_URL?.trim()
  || process.env.VERCEL_URL?.trim();
const originInput = configuredOrigin || (systemDomain ? `https://${systemDomain}` : 'http://localhost:5173');
const originUrl = new URL(originInput);

if (originUrl.pathname !== '/' || originUrl.search || originUrl.hash) {
  throw new Error('ORCA_PUBLIC_URL must be an origin without a path, query, or fragment.');
}
if (process.env.VERCEL_ENV === 'production' && originUrl.protocol !== 'https:') {
  throw new Error('Production MCP discovery requires an HTTPS ORCA public origin.');
}

const origin = originUrl.origin;
const scopes = ['records:read', 'reports:read', 'billing:simulate', 'actions:propose'];
const files = new Map([
  ['.well-known/oauth-protected-resource/mcp', {
    resource: `${origin}/mcp`,
    authorization_servers: [origin],
    scopes_supported: scopes,
    bearer_methods_supported: ['header'],
    resource_name: 'ORCA MCP',
  }],
  ['.well-known/oauth-authorization-server', {
    issuer: origin,
    authorization_endpoint: `${origin}/mcp/oauth/authorize`,
    token_endpoint: `${origin}/mcp/oauth/token`,
    revocation_endpoint: `${origin}/mcp/oauth/revoke`,
    registration_endpoint: `${origin}/mcp/register`,
    response_types_supported: ['code'],
    grant_types_supported: ['authorization_code'],
    token_endpoint_auth_methods_supported: ['none'],
    scopes_supported: scopes,
    code_challenge_methods_supported: ['S256'],
    authorization_response_iss_parameter_supported: true,
  }],
]);

for (const [relativePath, metadata] of files) {
  const path = resolve(root, 'public', relativePath);
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(metadata, null, 2)}\n`, { mode: 0o644 });
}

console.log(`Generated MCP OAuth discovery for ${origin}`);
