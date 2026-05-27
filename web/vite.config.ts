import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/graphql': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/auth/oauth2': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    // Without manualChunks the default Rollup heuristic dumps the heavy
    // visualization libs (recharts ~600KB, framer-motion ~150KB) into the
    // first route chunk that imports them — usually the dashboard — so the
    // first paint pulls a 1MB+ JS bundle. Splitting them into named vendor
    // chunks lets the browser cache them across route navigations.
    rollupOptions: {
      output: {
        // Rolldown (Vite 8) requires the function form. Map node_modules
        // paths into named vendor chunks; anything not matched stays in
        // its natural route chunk.
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined;
          if (id.includes('node_modules/recharts/')) return 'vendor-charts';
          if (id.includes('node_modules/framer-motion/')) return 'vendor-motion';
          if (id.includes('node_modules/react-markdown/') || id.includes('node_modules/remark-gfm/')) return 'vendor-markdown';
          if (id.includes('node_modules/@heroicons/')) return 'vendor-icons';
          if (id.includes('node_modules/@sentry/')) return 'vendor-sentry';
          if (id.includes('node_modules/@apollo/') || id.includes('node_modules/graphql/')) return 'vendor-apollo';
          if (
            id.includes('node_modules/react-dom/') ||
            id.includes('node_modules/react-router-dom/') ||
            id.includes('node_modules/react-router/') ||
            id.match(/node_modules\/react\/(?!.*node_modules)/)
          ) {
            return 'vendor-react';
          }
          return undefined;
        },
      },
    },
    // Surface oversized chunks during build so regressions get caught.
    chunkSizeWarningLimit: 600,
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    // Vitest greedily globs **/*.spec.ts which picks up Playwright e2e
    // specs (web/e2e/*.spec.ts). Playwright has its own runner — keep them
    // out of the vitest run.
    exclude: ['**/node_modules/**', '**/dist/**', 'e2e/**', 'playwright-report/**', 'test-results/**'],
  },
});
