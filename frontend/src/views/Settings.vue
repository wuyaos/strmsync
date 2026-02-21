<template>
  <div class="settings-page">
    <div class="page-header">
      <h1 class="page-title">系统设置</h1>
      <div class="page-actions">
        <el-button type="primary" @click="handleSave">保存设置</el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" type="border-card">
      <!-- 扫描设置 -->
      <el-tab-pane label="扫描配置" name="scanner">
        <el-form :model="settings.scanner" label-width="120px">
          <el-form-item label="并发数">
            <el-input
              v-model.number="settings.scanner.concurrency"
              type="number"
              :min="1"
              :max="100"
              class="input-short"
            />
            <span class="form-help">同时扫描的文件数量，过高可能影响性能</span>
          </el-form-item>

          <el-form-item label="批量大小">
            <el-input
              v-model.number="settings.scanner.batchSize"
              type="number"
              :min="10"
              :max="1000"
              class="input-short"
            />
            <span class="form-help">批量写入数据库的记录数</span>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 日志设置 -->
      <el-tab-pane label="日志配置" name="log">
        <el-form :model="settings.log" label-width="120px">
          <el-form-item label="日志级别">
            <el-select v-model="settings.log.level">
              <el-option label="DEBUG" value="debug" />
              <el-option label="INFO" value="info" />
              <el-option label="WARN" value="warn" />
              <el-option label="ERROR" value="error" />
            </el-select>
          </el-form-item>

          <el-form-item label="写入数据库">
            <el-switch v-model="settings.log.toDB" />
            <span class="form-help">是否将日志写入数据库（需要重启）</span>
          </el-form-item>

          <el-form-item label="日志路径">
            <el-input v-model="settings.log.path" placeholder="logs" />
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 主题设置 -->
      <el-tab-pane label="主题设置" name="theme">
        <div class="theme-section">
          <h3>界面主题</h3>
          <p class="description">自定义系统的视觉外观和主题风格</p>

          <el-form :model="settings.theme" label-width="120px">
            <el-form-item label="主题模式">
              <el-radio-group v-model="settings.theme.mode">
                <el-radio-button label="light">浅色</el-radio-button>
                <el-radio-button label="dark">深色</el-radio-button>
                <el-radio-button label="auto">跟随系统</el-radio-button>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="主题色">
              <el-color-picker
                v-model="settings.theme.primaryColor"
                show-alpha
                :predefine="predefineColors"
              />
              <span class="form-help">自定义系统主色调</span>
            </el-form-item>

            <el-form-item label="紧凑模式">
              <el-switch v-model="settings.theme.compact" />
              <span class="form-help">减小组件间距，提高信息密度</span>
            </el-form-item>
          </el-form>
        </div>
      </el-tab-pane>

      <!-- 接口速率 -->
      <el-tab-pane label="接口速率" name="rate">
        <el-form :model="settings.rate" label-width="160px">
          <el-form-item label="下载队列每秒处理数量">
            <el-input
              v-model.number="settings.rate.download_rate_per_sec"
              type="number"
              :min="1"
              :max="1000"
              class="input-short"
            />
            <span class="form-help">控制下载队列的处理速度，过高可能影响稳定性</span>
          </el-form-item>

          <el-form-item label="接口速率(每秒请求数)">
            <el-input
              v-model.number="settings.rate.api_rate"
              type="number"
              :min="1"
              :max="1000"
              class="input-short"
            />
            <span class="form-help">限制接口请求频率，避免触发服务端限流</span>
          </el-form-item>

          <el-form-item label="接口重试次数">
            <el-input
              v-model.number="settings.rate.api_retry_max"
              type="number"
              :min="0"
              :max="10"
              class="input-short"
            />
            <span class="form-help">接口调用失败时的最大重试次数</span>
          </el-form-item>

          <el-form-item label="接口重试间隔秒数">
            <el-input
              v-model.number="settings.rate.api_retry_interval_sec"
              type="number"
              :min="1"
              :max="60"
              class="input-short"
            />
            <span class="form-help">重试之间的等待时间（秒）</span>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 通知样式 -->
      <el-tab-pane label="通知样式" name="notification">
        <div class="notification-section">
          <h3>UI通知配置</h3>
          <p class="description">配置前端界面的消息提示和通知样式（与媒体库刷新通知不同）</p>

          <el-form :model="settings.notification" label-width="140px">
            <el-form-item label="通知位置">
              <el-select v-model="settings.notification.position">
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
        <div class="about-section">
          <h2>STRMSync</h2>
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
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getSettings, updateSettings } from '@/api/settings'
import { useSystemInfo } from '@/composables/useSystemInfo'

const activeTab = ref('scanner')

// 预定义颜色
const predefineColors = ref([
  '#409EFF',
  '#67C23A',
  '#E6A23C',
  '#F56C6C',
  '#909399',
  '#ff4500',
  '#ff8c00',
  '#ffd700',
  '#90ee90',
  '#00ced1',
  '#1e90ff',
  '#c71585'
])

// 默认设置
const defaultSettings = {
  scanner: {
    concurrency: 20,
    batchSize: 500
  },
  log: {
    level: 'info',
    toDB: false,
    path: 'logs'
  },
  theme: {
    mode: 'light',
    primaryColor: '#409EFF',
    compact: false
  },
  rate: {
    download_rate_per_sec: 10,
    api_rate: 10,
    api_retry_max: 3,
    api_retry_interval_sec: 60
  },
  notification: {
    position: 'top-right',
    duration: 3,
    showIcon: true,
    sound: false
  }
}

const settings = ref({ ...defaultSettings })
const { frontendVersion, loadSystemInfo } = useSystemInfo()

const loadSettings = async () => {
  try {
    const data = await getSettings()
    const resolved = data?.settings || data
    if (resolved) {
      // 深度合并分组，避免嵌套配置被整体覆盖
      settings.value = {
        scanner: { ...defaultSettings.scanner, ...(resolved.scanner || {}) },
        log: { ...defaultSettings.log, ...(resolved.log || {}) },
        theme: { ...defaultSettings.theme, ...(resolved.theme || {}) },
        rate: { ...defaultSettings.rate, ...(resolved.rate || {}) },
        notification: { ...defaultSettings.notification, ...(resolved.notification || {}) }
      }
    } else {
      settings.value = { ...defaultSettings }
    }
  } catch (error) {
    console.error('加载设置失败:', error)
    // 使用默认值
    settings.value = { ...defaultSettings }
  }
}

const handleSave = async () => {
  try {
    await updateSettings(settings.value)
    ElMessage.success('设置已保存')
  } catch (error) {
    console.error('保存设置失败:', error)
    ElMessage.error(error.response?.data?.error || '保存失败')
  }
}

onMounted(() => {
  loadSettings()
  loadSystemInfo()
})
</script>

<style scoped lang="scss">
.settings-page {
  padding: 20px;

  .theme-section,
  .notification-section {
    padding: 20px;

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

    h2 {
      font-size: 28px;
      margin-bottom: 12px;
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
