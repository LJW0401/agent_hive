import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test-setup.ts',
  },
  server: {
    host: true,
    proxy: {
      '/ws': {
        target: 'http://localhost:8090',
        ws: true,
      },
      '/api': {
        target: 'http://localhost:8090',
      },
    },
  },
})
