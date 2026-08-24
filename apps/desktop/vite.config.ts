import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  server: { port: 5173, strictPort: true },
  build: {
    rollupOptions: {
      input: {
        popover: resolve(__dirname, 'index.html'),
        settings: resolve(__dirname, 'settings.html'),
      },
    },
  },
  test: { environment: 'jsdom' },
})
