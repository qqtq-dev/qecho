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
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      output: {
        // 拆出大依赖：首屏可并行下载，二次访问时这些 chunk 命中长缓存
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('ant-design-vue') || id.includes('@ant-design')) return 'vendor-antd'
          if (id.includes('/echarts/') || id.includes('zrender')) return 'vendor-echarts'
          if (id.includes('/vue/') || id.includes('vue-router') || id.includes('/pinia/')) return 'vendor-vue'
          if (id.includes('marked') || id.includes('dompurify')) return 'vendor-markdown'
          if (id.includes('lucide-vue-next')) return 'vendor-icons'
          return 'vendor'
        },
      },
    },
  },
})
