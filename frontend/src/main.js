import { createApp } from 'vue'
import 'element-plus/theme-chalk/dark/css-vars.css'
import { ElMessage } from 'element-plus'
import router from './router'
import App from './App.vue'
import './assets/styles/main.scss'
import { installGlobalErrorHandlers } from './utils/error'

const app = createApp(App)

app.use(router)
installGlobalErrorHandlers(app, {
  notify: (message) => ElMessage.error({ message, grouping: true })
})

const appVersion = import.meta.env.VITE_APP_VERSION || 'unknown'
console.info(`[STRMSync] 前端版本: ${appVersion}`)

app.mount('#app')
