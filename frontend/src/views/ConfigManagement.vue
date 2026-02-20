<template>
  <div class="config-management-page">
    <div class="page-header">
      <h1 class="page-title">配置管理</h1>
      <div class="page-description">
        管理数据源和服务器配置，包括媒体库通知设置
      </div>
    </div>

    <el-tabs v-model="activeTab" type="border-card" class="config-tabs">
      <!-- 数据源配置 -->
      <el-tab-pane label="数据源配置" name="sources">
        <template #label>
          <span class="tab-label">
            <el-icon><FolderOpened /></el-icon>
            数据源配置
          </span>
        </template>
        <Servers />
      </el-tab-pane>

      <!-- 服务器配置 -->
      <el-tab-pane label="服务器配置" name="servers">
        <template #label>
          <span class="tab-label">
            <el-icon><Monitor /></el-icon>
            服务器配置
          </span>
        </template>
        <ServerConfig />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { FolderOpened, Monitor } from '@element-plus/icons-vue'
import Servers from '@/views/Servers.vue'
import ServerConfig from '@/views/ServerConfig.vue'

// 当前激活的标签页
const activeTab = ref('sources')
</script>

<style scoped lang="scss">
.config-management-page {
  .page-header {
    margin-bottom: 20px;

    .page-title {
      font-size: 24px;
      font-weight: 600;
      margin: 0 0 8px 0;
      color: var(--el-text-color-primary);
    }

    .page-description {
      font-size: 14px;
      color: var(--el-text-color-secondary);
    }
  }

  .config-tabs {
    :deep(.el-tabs__header) {
      margin-bottom: 0;
    }

    .tab-label {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 14px;
    }
  }
}
</style>
