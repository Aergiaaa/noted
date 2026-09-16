import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      // Dev only: forward API to Go. Prod stays same-origin via nginx.
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    coverage: {
      // istanbul, not v8: v8 coverage relies on the Node inspector and
      // reports 0% when vitest runs under the Bun runtime.
      provider: 'istanbul',
      reporter: ['text', 'lcov'],
      include: ['src/**/*.{ts,tsx}'],
      // src/main.tsx is the createRoot entrypoint: it can only run by
      // booting the real app, so it is excluded from the 100% gate the
      // same way func main is excluded on the backend.
      exclude: ['src/main.tsx', 'src/**/*.test.{ts,tsx}', 'src/**/*.d.ts'],
      thresholds: {
        lines: 100,
        functions: 100,
        branches: 100,
        statements: 100,
      },
    },
  },
})
