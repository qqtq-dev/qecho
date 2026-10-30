import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  base: './',
  server: {
    port: 54311,
    host: '127.0.0.1',
    watch: {
      usePolling: true,
      interval: 300,
      ignored: (path: string) => {
        const p = path.replace(/\\/g, '/')
        return p.includes('/src-tauri') || p.includes('/engine/') || p.includes('/dist/')
      },
    },
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:54310',
        changeOrigin: true,
      },
    },
  },
  build: {
    target: 'chrome105',
    outDir: 'dist',
    emptyOutDir: true,
  },
})
