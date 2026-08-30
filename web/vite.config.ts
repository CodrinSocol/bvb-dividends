// The vitest config re-export is what allows the test block below to sit in the
// same file as the build config.
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';

export default defineConfig({
  root: __dirname,
  plugins: [
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: !process.env.VITEST,
      // Tests live beside the route they cover and must not become routes.
      routeFileIgnorePattern: String.raw`\.(test|spec)\.[jt]sx?$`,
    }),
    react(),
    tailwindcss(),
  ],
  server: {
    port: 4200,
    // The dev server proxies the API prefix, so the client uses the same
    // origin-relative URLs in development that it uses once the build is
    // embedded in the binary and served from the same host.
    proxy: {
      '/api': { target: process.env.VITE_API_URL ?? 'http://localhost:8080', changeOrigin: true },
    },
  },
  build: {
    // Written where web/embed.go embeds it from, so `go build` carries the
    // application into the binary.
    outDir: 'dist',
    emptyOutDir: false,
    sourcemap: true,
  },
  test: {
    name: 'web',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
});
