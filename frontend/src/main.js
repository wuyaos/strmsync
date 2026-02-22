import { createApp } from 'vue'
import 'element-plus/theme-chalk/dark/css-vars.css'
import router from './router'
import App from './App.vue'
import './assets/styles/main.scss'

const app = createApp(App)

app.use(router)

const appVersion = import.meta.env.VITE_APP_VERSION || 'unknown'
console.info(`[STRMSync] 前端版本: ${appVersion}`)

app.mount('#app')
