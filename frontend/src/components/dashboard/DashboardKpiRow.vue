<template>
  <el-row :gutter="32" class="dashboard-row">
    <el-col :xs="24" :sm="12" :md="6">
      <el-card shadow="hover" class="stat-card stat-card--icon">
        <div class="stat-icon">
          <img :src="folderSvg" alt="网盘数量" />
        </div>
        <div class="stat-content">
          <div class="stat-title">网盘数量</div>
          <div class="stat-number-row">
            <span class="stat-number">{{ stats.serverTotal }}</span>
            <span class="stat-inline compact">
              启用 <span class="stat-pill-number">{{ stats.serverEnabled }}</span>
              / 禁用 <span class="stat-pill-number">{{ stats.serverDisabled }}</span>
            </span>
          </div>
        </div>
      </el-card>
    </el-col>

    <el-col :xs="24" :sm="12" :md="6">
      <el-card shadow="hover" class="stat-card stat-card--icon">
        <div class="stat-icon">
          <img :src="listSvg" alt="任务配置数量" />
        </div>
        <div class="stat-content">
          <div class="stat-title">任务配置数量</div>
          <div class="stat-number-row">
            <span class="stat-number">{{ stats.jobTotal }}</span>
            <span class="stat-inline compact">
              启用 <span class="stat-pill-number">{{ stats.jobEnabled }}</span>
              / 禁用 <span class="stat-pill-number">{{ stats.jobDisabled }}</span>
            </span>
          </div>
        </div>
      </el-card>
    </el-col>

    <el-col :xs="24" :sm="12" :md="6">
      <el-card shadow="hover" class="stat-card stat-card--icon">
        <div class="stat-icon">
          <img :src="clockSvg" alt="最近1小时运行" />
        </div>
        <div class="stat-content">
          <div class="stat-title">最近1小时运行</div>
          <div class="stat-number-row">
            <span class="stat-number">{{ stats.recentRuns }}</span>
          </div>
        </div>
      </el-card>
    </el-col>

    <el-col :xs="24" :sm="12" :md="6">
      <el-card shadow="hover" class="stat-card stat-card--icon">
        <div class="stat-icon">
          <img :src="warningSvg" alt="失败任务" />
        </div>
        <div class="stat-content">
          <div class="stat-title">失败任务(24h)</div>
          <div class="stat-number-row">
            <span class="stat-number">{{ stats.failedRuns }}</span>
          </div>
        </div>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup>
import clockSvg from '@/assets/icons/Clock.svg'
import folderSvg from '@/assets/icons/FolderOpened.svg'
import listSvg from '@/assets/icons/List.svg'
import warningSvg from '@/assets/icons/WarningFilled.svg'

defineProps({
  stats: {
    type: Object,
    required: true
  }
})
</script>

<style scoped lang="scss">
.dashboard-row {
  row-gap: 12px;
}

.stat-card--icon {
  :deep(.el-card__body) {
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 16px;
    padding: 24px;
  }

  .stat-icon {
    width: 56px;
    height: 56px;
    border-radius: 12px;
    background: var(--el-color-primary-light-9);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;

    img {
      width: 36px;
      height: 36px;
      object-fit: contain;
    }
  }

  .stat-content {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    flex: 1;
  }

  .stat-title {
    font-size: 18px;
    font-weight: 500;
    color: var(--el-text-color-secondary);
    line-height: 1.2;
  }

  .stat-number-row {
    display: flex;
    align-items: baseline;
    gap: 8px;
    flex-wrap: nowrap;
  }

  .stat-number {
    font-size: 32px;
    font-weight: 600;
    color: var(--el-text-color-primary);
    line-height: 1;
  }

  .stat-inline {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--el-text-color-secondary);
    white-space: nowrap;

    &.compact {
      font-size: 12px;
      gap: 4px;
    }
  }

  .stat-pill-number {
    font-size: 14px;
    font-weight: 600;
  }
}
</style>
