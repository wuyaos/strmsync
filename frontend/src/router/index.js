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
        path: '/config',
        name: 'ConfigManagement',
        component: () => import('@/views/ConfigManagement.vue'),
        meta: { title: '配置管理', icon: 'Setting' }
      },
      {
        path: '/tasks',
        name: 'Tasks',
        component: () => import('@/views/Tasks.vue'),
        meta: { title: '任务管理', icon: 'List' }
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
        path: '/server-settings',
        redirect: '/config'
      },
      {
        path: '/sources',
        redirect: '/config'
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
