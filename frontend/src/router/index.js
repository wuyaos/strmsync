import { createRouter, createWebHistory } from 'vue-router'
import MainLayout from '@/layouts/MainLayout.vue'
import LogsView from '@/views/Logs.vue'

const routes = [
  {
    path: '/',
    component: MainLayout,
    redirect: '/dashboard',
    children: [
      {
        path: '/dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
        meta: { title: '仪表盘', icon: 'DataAnalysis' }
      },
      {
        path: '/servers',
        redirect: '/data-servers'
      },
      {
        path: '/data-servers',
        name: 'DataServers',
        component: () => import('@/views/DataServers.vue'),
        meta: { title: '网盘管理', icon: 'FolderOpened' }
      },
      {
        path: '/media-servers',
        name: 'MediaServers',
        component: () => import('@/views/MediaServers.vue'),
        meta: { title: '媒体服务器', icon: 'Film' }
      },
      {
        path: '/jobs',
        name: 'Jobs',
        component: () => import('@/views/Jobs.vue'),
        meta: { title: '任务配置', icon: 'List' }
      },
      {
        path: '/runs',
        name: 'TaskRuns',
        component: () => import('@/views/TaskRuns.vue'),
        meta: { title: '执行历史', icon: 'Clock' }
      },
      {
        path: '/tools',
        name: 'Tools',
        component: () => import('@/views/Tools.vue'),
        meta: { title: '小工具', icon: 'Tools' }
      },
      {
        path: '/logs',
        name: 'Logs',
        component: LogsView,
        meta: { title: '系统日志', icon: 'Document' }
      },
      {
        path: '/settings',
        name: 'Settings',
        component: () => import('@/views/Settings.vue'),
        meta: { title: '系统设置', icon: 'Setting' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  // 设置页面标题
  if (to.meta.title) {
    document.title = `${to.meta.title} - STRMSync`
  }
  next()
})

router.afterEach(() => {
  if (typeof window === 'undefined') return
  try {
    sessionStorage.removeItem('route_chunk_reload_once')
  } catch (error) {}
  try {
    delete window.__route_chunk_reload_once__
  } catch (error) {}
})

router.onError((error) => {
  const message = String(error?.message || '')
  const isChunkError = message.includes('Loading chunk')
    || message.includes('Failed to fetch dynamically imported module')
    || message.includes('Importing a module script failed')
  if (isChunkError && typeof window !== 'undefined') {
    const cacheKey = 'route_chunk_reload_once'
    let storageReady = true
    try {
      if (!sessionStorage.getItem(cacheKey)) {
        sessionStorage.setItem(cacheKey, '1')
        window.location.reload()
      }
    } catch (error) {
      storageReady = false
    }
    if (!storageReady && !window.__route_chunk_reload_once__) {
      window.__route_chunk_reload_once__ = true
      window.location.reload()
    }
  }
})

export default router
