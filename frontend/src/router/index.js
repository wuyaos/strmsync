import { createRouter, createWebHistory } from 'vue-router'
import MainLayout from '@/layouts/MainLayout.vue'

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
        name: 'Servers',
        component: () => import('@/views/Servers.vue'),
        meta: { title: '服务器管理', icon: 'Connection' }
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
        component: () => import('@/views/Logs.vue'),
        meta: { title: '日志', icon: 'Document' }
      },
      {
        path: '/settings',
        name: 'Settings',
        component: () => import('@/views/Settings.vue'),
        meta: { title: '系统设置', icon: 'Setting' }
      },
      // 兼容旧路由 - 重定向
      {
        path: '/config',
        redirect: '/servers'
      },
      {
        path: '/server-settings',
        redirect: '/servers'
      },
      {
        path: '/sources',
        redirect: '/servers'
      },
      {
        path: '/tasks',
        redirect: '/jobs'
      },
      {
        path: '/files',
        redirect: '/tools'
      },
      {
        path: '/notifications',
        redirect: '/settings'
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

export default router
