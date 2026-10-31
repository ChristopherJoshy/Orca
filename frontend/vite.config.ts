import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', 'ORCA_');
  const apiTarget = env.ORCA_API_TARGET || 'http://127.0.0.1:8080';
  const apiProxy = { target: apiTarget, changeOrigin: false };
  return {
    server: {
      port: 5173,
      strictPort: true,
      proxy: {
        '/api': apiProxy,
        '/mcp': apiProxy,
        '/.well-known': apiProxy,
      },
    },
  };
});
