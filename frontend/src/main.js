import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import router from './router'
import App from './App.vue'
import './assets/styles/main.scss'

const app = createApp(App)
const pinia = createPinia()

// 注册所有 Element Plus 图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(pinia)
app.use(router)

// Element Plus 全局配置
app.use(ElementPlus, {
  size: 'default',
  zIndex: 3000
})

const appVersion = import.meta.env.VITE_APP_VERSION || 'unknown'
console.info(`[STRMSync] 前端版本: ${appVersion}`)

app.mount('#app')
