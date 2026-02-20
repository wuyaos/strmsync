import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  css: {
    preprocessorOptions: {
      // 使用 modern API 避免 Sass legacy-js-api 弃用警告
      scss: {
        api: 'modern-compiler',
        silenceDeprecations: ['legacy-js-api']
      }
    }
  },
  server: {
    port: 5676,
    proxy: {
      '/api': {
        // 支持环境变量配置后端端口（用于开发/测试环境切换）
        target: process.env.VITE_BACKEND_PORT
          ? `http://localhost:${process.env.VITE_BACKEND_PORT}`
          : 'http://localhost:6754',
        changeOrigin: true
      }
    }
  }
})
