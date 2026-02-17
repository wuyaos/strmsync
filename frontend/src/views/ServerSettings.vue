<template>
  <div class="server-settings-page">
    <el-tabs v-model="activeTab" type="border-card">
      <!-- 数据服务器配置 -->
      <el-tab-pane label="数据服务器" name="data">
        <div class="tab-content">
          <h2>数据服务器配置</h2>
          <p class="description">配置各种数据源适配器（Local、CloudDrive2、OpenList等）</p>

          <div class="placeholder">
            <el-empty description="数据服务器配置功能开发中...">
              <el-button type="primary">添加数据服务器</el-button>
            </el-empty>
          </div>
        </div>
      </el-tab-pane>

      <!-- 媒体服务器配置 -->
      <el-tab-pane label="媒体服务器" name="media">
        <div class="tab-content">
          <h2>媒体服务器配置</h2>
          <p class="description">配置Emby、Jellyfin等媒体服务器用于通知刷新</p>

          <div class="placeholder">
            <el-empty description="媒体服务器配置功能开发中...">
              <el-button type="primary">添加媒体服务器</el-button>
            </el-empty>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const activeTab = ref('data')
</script>

<style scoped lang="scss">
.server-settings-page {
  padding: 20px;

  .tab-content {
    padding: 20px;

    h2 {
      margin: 0 0 8px 0;
      font-size: 18px;
      font-weight: 600;
    }

    .description {
      margin: 0 0 24px 0;
      color: var(--el-text-color-secondary);
      font-size: 14px;
    }

    .placeholder {
      margin-top: 60px;
    }
  }
}
</style>
