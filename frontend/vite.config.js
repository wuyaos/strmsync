import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import { readFileSync } from 'fs'

const versionFile = new URL('../VERSION', import.meta.url)
const appVersion = readFileSync(versionFile, 'utf-8').trim() || 'unknown'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  define: {
    'import.meta.env.VITE_APP_VERSION': JSON.stringify(appVersion)
  },
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
  build: {
    // 输出到 dist/web_statics（与后端可执行文件同级）
    outDir: '../dist/web_statics',
    emptyOutDir: true,
    // 资源路径使用相对路径
    assetsDir: 'assets',
    // 生成 manifest.json 用于资源映射
    manifest: false
  },
  server: {
    port: 5678,
    proxy: {
      '/api': {
        // 支持环境变量配置后端端口（用于开发/测试环境切换）
        target: process.env.VITE_BACKEND_PORT
          ? `http://localhost:${process.env.VITE_BACKEND_PORT}`
          : 'http://localhost:5677',
        changeOrigin: true
      }
    }
  }
})
