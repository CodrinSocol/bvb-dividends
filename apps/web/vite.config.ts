import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  root: __dirname,
  plugins: [react(), tailwindcss()],
  server: {
    port: 4200,
    // The API's CORS allow-list names this origin, so requests from the dev
    // server reach it directly; the proxy is here so a build served from the
    // same host as the API works without changing the client's base URL.
    proxy: {
      '/v1': { target: process.env.VITE_API_URL ?? 'http://localhost:8080', changeOrigin: true },
    },
  },
  build: {
    outDir: '../../dist/apps/web',
    emptyOutDir: true,
    sourcemap: true,
  },
});
