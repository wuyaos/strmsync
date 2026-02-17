<template>
  <div class="dashboard-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">仪表盘</h1>
      <div class="page-actions">
        <el-button :icon="Refresh" @click="loadData">刷新</el-button>
      </div>
    </div>

    <!-- KPI 统计卡片 -->
    <el-row :gutter="16" class="kpi-row">
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="数据源" :value="stats.sourceCount">
            <template #suffix>
              <el-text size="small" type="success">{{ stats.activeSourceCount }} 个正常</el-text>
            </template>
          </el-statistic>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="总文件数" :value="stats.totalFiles">
            <template #suffix>
              <el-text size="small" type="primary">
                <el-icon><TrendCharts /></el-icon>
                +{{ stats.newFilesToday }}
              </el-text>
            </template>
          </el-statistic>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="运行任务" :value="stats.runningTasks">
            <template #prefix>
              <el-icon :class="{ 'is-loading': stats.runningTasks > 0 }">
                <Loading />
              </el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="失败任务" :value="stats.failedTasks">
            <template #prefix>
              <el-icon v-if="stats.failedTasks > 0" color="#F56C6C">
                <WarningFilled />
              </el-icon>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
    </el-row>

    <!-- 数据源状态 -->
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :xs="24" :md="16">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>数据源状态</span>
              <el-button text @click="$router.push('/config')">
                查看全部 <el-icon><ArrowRight /></el-icon>
              </el-button>
            </div>
          </template>

          <el-empty v-if="sourceList.length === 0" description="暂无数据源" />

          <div v-else class="source-list">
            <div
              v-for="source in sourceList"
              :key="source.id"
              class="source-item"
              @click="$router.push(`/config?id=${source.id}`)"
            >
              <div class="source-info">
                <el-icon :size="20"><FolderOpened /></el-icon>
                <div class="source-details">
                  <div class="source-name">{{ source.name }}</div>
                  <div class="source-meta">
                    {{ source.type }} · {{ source.file_count || 0 }} 个文件
                  </div>
                </div>
              </div>

              <div class="source-status">
                <el-tag
                  :type="getStatusType(source.status)"
                  size="small"
                  effect="dark"
                >
                  {{ getStatusText(source.status) }}
                </el-tag>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :md="8">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>最近任务</span>
              <el-button text @click="$router.push('/tasks')">
                查看全部 <el-icon><ArrowRight /></el-icon>
              </el-button>
            </div>
          </template>

          <el-empty v-if="taskList.length === 0" description="暂无任务" />

          <el-timeline v-else>
            <el-timeline-item
              v-for="task in taskList"
              :key="task.id"
              :type="getTaskType(task.status)"
              :icon="getTaskIcon(task.status)"
            >
              <div class="task-item">
                <div class="task-name">{{ task.name }}</div>
                <div class="task-meta">
                  {{ formatTime(task.created_at) }}
                </div>
              </div>
            </el-timeline-item>
          </el-timeline>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getSourceList } from '@/api/source'
import { ElMessage } from 'element-plus'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

// 统计数据
const stats = ref({
  sourceCount: 0,
  activeSourceCount: 0,
  totalFiles: 0,
  newFilesToday: 0,
  runningTasks: 0,
  failedTasks: 0
})

// 数据源列表
const sourceList = ref([])

// 任务列表
const taskList = ref([])

// 加载数据
const loadData = async () => {
  try {
    // 获取数据源列表
    const data = await getSourceList()
    const sources = data.sources || []
    sourceList.value = sources

    // 计算统计数据
    stats.value.sourceCount = sources.length
    stats.value.activeSourceCount = sources.filter(s => s.status !== 'error').length
    stats.value.totalFiles = sources.reduce((sum, s) => sum + (s.file_count || 0), 0)

    // 模拟其他统计数据（实际应从后端获取）
    stats.value.newFilesToday = 125
    stats.value.runningTasks = 0
    stats.value.failedTasks = 0
  } catch (error) {
    console.error('加载数据失败:', error)
  }
}

// 获取状态类型
const getStatusType = (status) => {
  const typeMap = {
    idle: 'info',
    scanning: 'primary',
    watching: 'success',
    error: 'danger'
  }
  return typeMap[status] || 'info'
}

// 获取状态文本
const getStatusText = (status) => {
  const textMap = {
    idle: '空闲',
    scanning: '扫描中',
    watching: '监控中',
    error: '错误'
  }
  return textMap[status] || status
}

// 获取任务类型
const getTaskType = (status) => {
  const typeMap = {
    running: 'primary',
    completed: 'success',
    failed: 'danger'
  }
  return typeMap[status] || 'info'
}

// 获取任务图标
const getTaskIcon = (status) => {
  const iconMap = {
    running: 'Loading',
    completed: 'SuccessFilled',
    failed: 'CircleCloseFilled'
  }
  return iconMap[status] || 'InfoFilled'
}

// 格式化时间
const formatTime = (time) => {
  return dayjs(time).fromNow()
}

// 组件挂载时加载数据
onMounted(() => {
  loadData()

  // 每30秒自动刷新
  setInterval(loadData, 30000)
})
</script>

<style scoped lang="scss">
.dashboard-page {
  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
  }

  .kpi-row {
    margin-bottom: 16px;
  }

  .stat-card {
    :deep(.el-card__body) {
      padding: 20px;
    }

    .el-statistic__content {
      font-size: 28px;
      font-weight: 600;
    }
  }

  .source-list {
    .source-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 12px;
      border-radius: 4px;
      cursor: pointer;
      transition: background-color 0.2s;

      &:hover {
        background-color: var(--el-fill-color-light);
      }

      .source-info {
        display: flex;
        align-items: center;
        gap: 12px;

        .source-details {
          .source-name {
            font-weight: 500;
            margin-bottom: 4px;
          }

          .source-meta {
            font-size: 12px;
            color: var(--el-text-color-secondary);
          }
        }
      }
    }
  }

  .task-item {
    .task-name {
      font-weight: 500;
      margin-bottom: 4px;
    }

    .task-meta {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }
}

.is-loading {
  animation: rotating 2s linear infinite;
}

@keyframes rotating {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
