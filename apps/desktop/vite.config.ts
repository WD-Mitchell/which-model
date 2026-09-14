import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  server: { port: 5173, strictPort: true },
  build: {
    manifest: true,
    rollupOptions: {
      input: {
        popover: resolve(__dirname, 'index.html'),
        settings: resolve(__dirname, 'settings.html'),
        offline: resolve(__dirname, 'offline.html'),
      },
    },
  },
  test: { environment: 'jsdom', setupFiles: ['./src/testSetup.ts'] },
})
