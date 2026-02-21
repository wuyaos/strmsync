<template>
  <el-container class="main-layout">
    <!-- 侧边栏 -->
    <el-aside :width="isCollapse ? '64px' : '200px'" class="sidebar">
      <div class="logo" :class="{ collapse: isCollapse }">
        <img :src="logoSvg" alt="STRMSync Logo" class="logo-icon" />
        <span v-if="!isCollapse">STRMSync</span>
      </div>

      <el-menu
        :default-active="activeMenu"
        :collapse="isCollapse"
        :router="true"
        class="sidebar-menu"
      >
        <el-menu-item
          v-for="route in routes"
          :key="route.path"
          :index="route.path"
        >
          <el-icon><component :is="route.meta.icon" /></el-icon>
          <template #title>{{ route.meta.title }}</template>
        </el-menu-item>
      </el-menu>

    </el-aside>

    <!-- 主内容区 -->
    <el-container>
      <!-- 顶部状态栏 -->
      <el-header class="header">
        <div class="header-left">
          <el-icon class="collapse-icon" @click="toggleCollapse">
            <Expand v-if="isCollapse" />
            <Fold v-else />
          </el-icon>
        </div>

        <div class="header-right">
          <a
            class="header-link"
            href="https://github.com/wuyaos/strmsync"
            target="_blank"
            rel="noopener noreferrer"
          >
            主页
          </a>
          <div class="version-info">{{ displayVersion }}</div>
          <!-- 暗色模式切换 -->
          <el-tooltip :content="isDark ? '切换到亮色模式' : '切换到暗色模式'">
            <el-icon :size="20" @click="toggleTheme" class="theme-toggle">
              <Moon v-if="!isDark" />
              <Sunny v-else />
            </el-icon>
          </el-tooltip>

          <!-- 刷新 -->
          <el-tooltip content="刷新数据">
            <el-icon :size="20" @click="handleRefresh" class="refresh-icon">
              <Refresh />
            </el-icon>
          </el-tooltip>
        </div>
      </el-header>

      <!-- 页面内容 -->
      <el-main class="main-content">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSystemInfo } from '@/composables/useSystemInfo'
import logoSvg from '@/assets/icons/logo.svg'

const route = useRoute()
const router = useRouter()
const { frontendVersion, loadSystemInfo } = useSystemInfo()
const displayVersion = computed(() => {
  const version = frontendVersion.value || 'unknown'
  return `STRMSync v${version}`
})

// 侧边栏折叠状态
const isCollapse = ref(false)
// 暗色模式
const isDark = ref(false)

// 菜单路由
const routes = computed(() => {
  const allRoutes = router.options.routes[0].children || []
  // 过滤掉重定向路由，只显示有meta.title的路由
  return allRoutes.filter(route => route.meta && route.meta.title)
})

// 当前激活的菜单
const activeMenu = computed(() => route.path)

// 切换侧边栏
const toggleCollapse = () => {
  isCollapse.value = !isCollapse.value
}

// 切换主题
const toggleTheme = () => {
  isDark.value = !isDark.value
  if (isDark.value) {
    document.documentElement.classList.add('dark')
    localStorage.setItem('theme', 'dark')
  } else {
    document.documentElement.classList.remove('dark')
    localStorage.setItem('theme', 'light')
  }
}

// 刷新数据
const handleRefresh = () => {
  window.location.reload()
}

// 初始化主题
const initTheme = () => {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark') {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

initTheme()

onMounted(() => {
  loadSystemInfo()
})
</script>

<style scoped lang="scss">
.main-layout {
  height: 100vh;
}

.sidebar {
  position: relative;
  background-color: var(--el-bg-color);
  border-right: 1px solid var(--el-border-color);
  transition: width 0.3s;

  .logo {
    height: 60px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 18px;
    font-weight: 600;
    color: var(--el-color-primary);
    border-bottom: 1px solid var(--el-border-color);
    gap: 8px;

    .logo-icon {
      width: 32px;
      height: 32px;
      object-fit: contain;
    }

    &.collapse {
      justify-content: center;

      .logo-icon {
        width: 28px;
        height: 28px;
      }
    }
  }

  .sidebar-menu {
    border-right: none;
  }

}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background-color: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color);

  .header-left {
    .collapse-icon {
      font-size: 20px;
      cursor: pointer;
      color: var(--el-text-color-primary);

      &:hover {
        color: var(--el-color-primary);
      }
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 20px;

    .header-link {
      color: var(--el-text-color-secondary);
      text-decoration: none;

      &:hover {
        color: var(--el-color-primary);
      }
    }

    .version-info {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    .el-icon {
      cursor: pointer;
      color: var(--el-text-color-regular);

      &:hover {
        color: var(--el-color-primary);
      }
    }
  }
}

.main-content {
  padding: 20px;
  background-color: var(--el-bg-color-page);
  overflow-y: auto;
}

// 过渡动画
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
