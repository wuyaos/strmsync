<template>
  <div class="settings-page flex flex-col gap-16 p-20">
    <div class="page-header">
      <div>
        <h1 class="page-title">系统设置</h1>
        <p class="page-description">配置全局参数与日志参数</p>
      </div>
      <div class="page-actions">
        <el-button type="primary" @click="handleSave">保存设置</el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" type="border-card" class="settings-tabs">
      <!-- 全局参数 -->
      <el-tab-pane label="全局参数" name="global">
        <div class="settings-pane">
          <el-form :model="settings.global" label-width="160px" class="global-form">
            <el-form-item label="并发数">
              <el-input
                v-model.number="settings.global.concurrency"
                type="number"
                :min="1"
                :max="100"
                class="input-short"
              />
              <span class="form-help">同时扫描的文件数量，过高可能影响性能</span>
            </el-form-item>

            <el-form-item label="批量大小">
              <el-input
                v-model.number="settings.global.batchSize"
                type="number"
                :min="10"
                :max="1000"
                class="input-short"
              />
              <span class="form-help">批量写入数据库的记录数</span>
            </el-form-item>
            <el-form-item label="下载队列每秒处理数量">
              <el-input
                v-model.number="settings.global.download_rate_per_sec"
                type="number"
                :min="1"
                :max="1000"
                class="input-short"
              />
              <span class="form-help">控制下载队列的处理速度，过高可能影响稳定性</span>
            </el-form-item>

            <el-form-item label="接口速率(每秒请求数)">
              <el-input
                v-model.number="settings.global.api_rate"
                type="number"
                :min="1"
                :max="1000"
                class="input-short"
              />
              <span class="form-help">限制接口请求频率，避免触发服务端限流</span>
            </el-form-item>

            <el-form-item label="接口重试次数">
              <el-input
                v-model.number="settings.global.api_retry_max"
                type="number"
                :min="0"
                :max="10"
                class="input-short"
              />
              <span class="form-help">接口调用失败时的最大重试次数</span>
            </el-form-item>

            <el-form-item label="接口重试间隔秒数">
              <el-input
                v-model.number="settings.global.api_retry_interval_sec"
                type="number"
                :min="1"
                :max="60"
                class="input-short"
              />
              <span class="form-help">重试之间的等待时间（秒）</span>
            </el-form-item>
          </el-form>
        </div>
      </el-tab-pane>

      <!-- 通知样式 -->
      <el-tab-pane label="系统通知" name="notification">
        <div class="settings-pane notification-section">
          <el-form :model="settings.notification" label-width="140px">
            <el-form-item label="通知位置">
              <el-select v-model="settings.notification.position" class="input-short">
                <el-option label="右上角" value="top-right" />
                <el-option label="右下角" value="bottom-right" />
                <el-option label="左上角" value="top-left" />
                <el-option label="左下角" value="bottom-left" />
              </el-select>
            </el-form-item>

            <el-form-item label="显示时长(秒)">
              <el-input
                v-model.number="settings.notification.duration"
                type="number"
                :min="1"
                :max="10"
                class="input-short"
              />
              <span class="form-help">消息提示的默认显示时长</span>
            </el-form-item>

            <el-form-item label="显示图标">
              <el-switch v-model="settings.notification.showIcon" />
              <span class="form-help">是否在消息中显示图标</span>
            </el-form-item>

            <el-form-item label="声音提示">
              <el-switch v-model="settings.notification.sound" />
              <span class="form-help">是否启用声音提示（仅重要消息）</span>
            </el-form-item>
          </el-form>
        </div>
      </el-tab-pane>

      <!-- 关于 -->
      <el-tab-pane label="关于" name="about">
        <div class="settings-pane about-section">
          <div class="about-header">
            <img :src="logoSvg" alt="STRMSync Logo" class="about-logo" />
            <h2>STRMSync</h2>
          </div>
          <p>版本：{{ frontendVersion }}</p>
          <p>自动化STRM媒体文件管理系统</p>
          <el-divider />
          <h3>功能特性</h3>
          <ul>
            <li>支持多种数据源适配器（Local、CloudDrive2、OpenList）</li>
            <li>自动扫描和生成STRM文件</li>
            <li>实时文件监控</li>
            <li>元数据同步</li>
            <li>媒体库刷新通知</li>
            <li>任务调度管理</li>
          </ul>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import logoSvg from '@/assets/icons/logo.svg'
import { useSettingsPage } from '@/composables/useSettingsPage'

const {
  activeTab,
  settings,
  frontendVersion,
  handleSave
} = useSettingsPage()
</script>

<style scoped lang="scss">
.settings-page {
  .settings-tabs {
    :deep(.el-tabs__header) {
      margin-bottom: 0;
    }

    :deep(.el-tabs__content) {
      padding: 0;
    }
  }

  .settings-pane {
    padding: 20px;
  }

  .global-form {
    :deep(.el-form-item__content) {
      align-items: flex-start;
      justify-content: flex-start;
      text-align: left;
      flex-wrap: nowrap;
    }

    :deep(.el-form-item__content .el-input) {
      margin-right: 12px;
    }

    :deep(.el-form-item__content .form-help) {
      margin-top: 0;
      white-space: nowrap;
    }
  }

  .form-help-inline {
    margin-left: 12px;
  }

  .theme-section,
  .notification-section {
    h3 {
      font-size: 18px;
      margin: 0 0 8px 0;
      font-weight: 600;
    }

    .description {
      margin: 0 0 24px 0;
      color: var(--el-text-color-secondary);
      font-size: 14px;
    }
  }

  .about-section {
    padding: 20px;

    .about-header {
      display: flex;
      align-items: center;
      gap: 16px;
      margin-bottom: 16px;

      .about-logo {
        width: 64px;
        height: 64px;
        object-fit: contain;
      }

      h2 {
        font-size: 28px;
        margin: 0;
      }
    }

    h3 {
      font-size: 18px;
      margin-top: 20px;
      margin-bottom: 12px;
    }

    p {
      margin: 8px 0;
      color: var(--el-text-color-regular);
    }

    ul {
      list-style: disc;
      padding-left: 24px;

      li {
        margin: 8px 0;
        color: var(--el-text-color-regular);
      }
    }
  }
}
</style>
