<template>
  <div class="tools-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">小工具</h1>
        <p class="page-description">实用工具集合，帮助您更好地管理媒体文件</p>
      </div>
    </div>

    <el-row :gutter="20" class="tools-grid">
      <el-col :xs="24" :sm="12" :md="8" :lg="6">
        <el-card shadow="hover" class="tool-card" @click="handleToolClick('rename')">
          <template #header>
            <div class="card-header">
              <el-icon :size="24"><EditPen /></el-icon>
              <span>批量改名</span>
            </div>
          </template>
          <div class="card-body">
            <p>批量修改文件名，支持正则表达式替换</p>
            <el-tag type="info" size="small">开发中</el-tag>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="8" :lg="6">
        <el-card shadow="hover" class="tool-card" @click="handleToolClick('crop')">
          <template #header>
            <div class="card-header">
              <el-icon :size="24"><Picture /></el-icon>
              <span>海报裁剪</span>
            </div>
          </template>
          <div class="card-body">
            <p>批量裁剪和调整海报尺寸</p>
            <el-tag type="info" size="small">开发中</el-tag>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="8" :lg="6">
        <el-card shadow="hover" class="tool-card disabled">
          <template #header>
            <div class="card-header">
              <el-icon :size="24"><More /></el-icon>
              <span>更多工具</span>
            </div>
          </template>
          <div class="card-body">
            <p>更多实用工具即将推出...</p>
            <el-tag type="info" size="small">敬请期待</el-tag>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ElMessage } from 'element-plus'
import EditPen from '~icons/ep/edit-pen'
import Picture from '~icons/ep/picture'
import More from '~icons/ep/more'

const handleToolClick = (tool) => {
  ElMessage.info(`${tool === 'rename' ? '批量改名' : '海报裁剪'}功能开发中...`)
}
</script>

<style scoped lang="scss">
.tools-page {
  padding: 20px;

  .tools-grid {
    :deep(.el-col) {
      display: flex;
      margin-bottom: 20px;
    }
  }

  .tool-card {
    cursor: pointer;
    transition: all 0.3s;
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 180px;

    :deep(.el-card__header) {
      padding: 16px 20px;
    }

    :deep(.el-card__body) {
      flex: 1;
      display: flex;
      flex-direction: column;
      padding: 20px;
    }

    &:not(.disabled):hover {
      transform: translateY(-4px);
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    }

    &.disabled {
      cursor: not-allowed;
      opacity: 0.6;
    }

    .card-body {
      flex: 1;
      display: flex;
      flex-direction: column;
      justify-content: space-between;

      p {
        margin: 0 0 12px 0;
        color: var(--el-text-color-secondary);
        font-size: 14px;
        flex: 1;
      }
    }
  }
}
</style>
