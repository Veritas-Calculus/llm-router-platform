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
  build: {
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // React core — rarely changes, cached long-term
            if (/[\\/](react|react-dom|react-router-dom)[\\/]/.test(id)) {
              return 'vendor-react';
            }
            // Charting — heavy, loaded only on dashboard
            if (/[\\/]recharts[\\/]/.test(id)) {
              return 'vendor-charts';
            }
            // Animation + UI utilities
            if (/[\\/](framer-motion|react-hot-toast)[\\/]/.test(id)) {
              return 'vendor-ui';
            }
            // State management + HTTP
            if (/[\\/](zustand|axios)[\\/]/.test(id)) {
              return 'vendor-state';
            }
          }
        },
      },
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
});
