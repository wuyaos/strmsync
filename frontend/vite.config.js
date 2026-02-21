import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import Icons from 'unplugin-icons/vite'
import IconsResolver from 'unplugin-icons/resolver'
import { resolve } from 'path'
import { readFileSync } from 'fs'

const versionFile = new URL('../VERSION', import.meta.url)
const appVersion = readFileSync(versionFile, 'utf-8').trim() || 'unknown'
const buildRoot = resolve(__dirname, '../build/vue')
const frontendPort = Number(process.env.FRONTEND_PORT) || 7786
const backendPort = process.env.VITE_BACKEND_PORT || '6786'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      resolvers: [ElementPlusResolver()],
      dts: false
    }),
    Components({
      resolvers: [
        ElementPlusResolver({ importStyle: 'css' }),
        IconsResolver({ prefix: 'Icon' })
      ],
      dts: false
    }),
    Icons({ autoInstall: true })
  ],
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
  cacheDir: resolve(buildRoot, '.vite'),
  build: {
    // 统一产物到 build/vue/dist
    outDir: resolve(buildRoot, 'dist'),
    emptyOutDir: true,
    // 资源路径使用相对路径
    assetsDir: 'assets',
    // 生成 manifest.json 用于资源映射
    manifest: false,
    // 开启代码压缩
    minify: 'esbuild'
  },
  server: {
    port: frontendPort,
    proxy: {
      '/api': {
        // 支持环境变量配置后端端口（用于开发/测试环境切换）
        target: `http://localhost:${backendPort}`,
        changeOrigin: true,
        timeout: 60000,
        proxyTimeout: 60000
      }
    }
  }
})
