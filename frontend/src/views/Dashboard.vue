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
          <el-statistic title="服务器数量" :value="stats.serverCount" />
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="任务配置数量" :value="stats.jobCount" />
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="最近1小时运行" :value="stats.recentRuns" />
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="失败任务(24h)" :value="stats.failedRuns" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 服务器概览 / 最近运行记录 -->
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :xs="24" :md="16">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>服务器概览</span>
              <el-button text @click="$router.push('/servers')">
                查看全部 <el-icon><ArrowRight /></el-icon>
              </el-button>
            </div>
          </template>

          <el-empty
            v-if="dataServers.length === 0 && mediaServers.length === 0"
            description="暂无服务器"
          />

          <div v-else class="server-list">
            <div class="server-group">
              <div class="group-title">数据服务器</div>
              <el-empty v-if="dataServers.length === 0" description="暂无数据服务器" />
              <div
                v-for="server in dataServers"
                :key="server.id"
                class="server-item"
                @click="$router.push('/servers')"
              >
                <div class="server-info">
                  <div class="server-name">{{ server.name }}</div>
                  <div class="server-meta">
                    {{ server.type }} · {{ server.host }}:{{ server.port }}
                  </div>
                </div>
                <el-tag :type="server.enabled ? 'success' : 'info'" size="small">
                  {{ server.enabled ? '启用' : '禁用' }}
                </el-tag>
              </div>
            </div>

            <el-divider />

            <div class="server-group">
              <div class="group-title">媒体服务器</div>
              <el-empty v-if="mediaServers.length === 0" description="暂无媒体服务器" />
              <div
                v-for="server in mediaServers"
                :key="server.id"
                class="server-item"
                @click="$router.push('/servers')"
              >
                <div class="server-info">
                  <div class="server-name">{{ server.name }}</div>
                  <div class="server-meta">
                    {{ server.type }} · {{ server.host }}:{{ server.port }}
                  </div>
                </div>
                <el-tag :type="server.enabled ? 'success' : 'info'" size="small">
                  {{ server.enabled ? '启用' : '禁用' }}
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
              <span>最近运行记录</span>
              <el-button text @click="$router.push('/runs')">
                查看全部 <el-icon><ArrowRight /></el-icon>
              </el-button>
            </div>
          </template>

          <el-empty v-if="runList.length === 0" description="暂无运行记录" />

          <el-timeline v-else>
            <el-timeline-item
              v-for="run in runList"
              :key="run.id"
              :type="getRunType(run.status)"
            >
              <div class="task-item">
                <div class="task-name">{{ run.job_name || run.job?.name || '未命名任务' }}</div>
                <div class="task-meta">
                  {{ formatTime(run.started_at || run.created_at) }}
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
import { ref, onMounted, onUnmounted } from 'vue'
import { Refresh, ArrowRight } from '@element-plus/icons-vue'
import { getServerList } from '@/api/servers'
import { getJobList } from '@/api/jobs'
import { getRunList } from '@/api/runs'
import { normalizeListResponse } from '@/api/normalize'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

// 统计数据
const stats = ref({
  serverCount: 0,
  jobCount: 0,
  recentRuns: 0,
  failedRuns: 0
})

// 服务器列表
const dataServers = ref([])
const mediaServers = ref([])

// 运行记录列表
const runList = ref([])

let refreshTimer = null

// 加载数据
const loadData = async () => {
  try {
    const now = dayjs()
    const lastHour = now.subtract(1, 'hour').toISOString()
    const last24h = now.subtract(24, 'hour').toISOString()

    const [
      dataServersResp,
      mediaServersResp,
      jobsResp,
      runsResp,
      recentRunsResp,
      failedRunsResp
    ] = await Promise.all([
      getServerList({ type: 'data', page: 1, pageSize: 5 }),
      getServerList({ type: 'media', page: 1, pageSize: 5 }),
      getJobList({ page: 1, pageSize: 1 }),
      getRunList({ page: 1, pageSize: 10 }),
      getRunList({ from: lastHour, to: now.toISOString(), page: 1, pageSize: 1 }),
      getRunList({ status: 'failed', from: last24h, to: now.toISOString(), page: 1, pageSize: 1 })
    ])

    const dataServersResult = normalizeListResponse(dataServersResp)
    const mediaServersResult = normalizeListResponse(mediaServersResp)
    const jobsResult = normalizeListResponse(jobsResp)
    const runsResult = normalizeListResponse(runsResp)
    const recentRunsResult = normalizeListResponse(recentRunsResp)
    const failedRunsResult = normalizeListResponse(failedRunsResp)

    dataServers.value = dataServersResult.list
    mediaServers.value = mediaServersResult.list
    runList.value = runsResult.list

    stats.value.serverCount = (dataServersResult.total || dataServersResult.list.length)
      + (mediaServersResult.total || mediaServersResult.list.length)
    stats.value.jobCount = jobsResult.total || jobsResult.list.length
    stats.value.recentRuns = recentRunsResult.total || recentRunsResult.list.length
    stats.value.failedRuns = failedRunsResult.total || failedRunsResult.list.length
  } catch (error) {
    console.error('加载数据失败:', error)
  }
}

// 获取运行记录类型
const getRunType = (status) => {
  const typeMap = {
    running: 'primary',
    completed: 'success',
    failed: 'danger',
    pending: 'info',
    cancelled: 'warning'
  }
  return typeMap[status] || 'info'
}

// 格式化时间
const formatTime = (time) => {
  return dayjs(time).fromNow()
}

// 组件挂载时加载数据
onMounted(() => {
  loadData()

  // 每30秒自动刷新
  refreshTimer = setInterval(loadData, 30000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<style scoped lang="scss">
.dashboard-page {
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

  .server-list {
    .server-group {
      .group-title {
        font-weight: 600;
        margin-bottom: 8px;
      }
    }

    .server-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 10px 12px;
      border-radius: 4px;
      cursor: pointer;
      transition: background-color 0.2s;

      &:hover {
        background-color: var(--el-fill-color-light);
      }

      .server-info {
        .server-name {
          font-weight: 500;
          margin-bottom: 4px;
        }

        .server-meta {
          font-size: 12px;
          color: var(--el-text-color-secondary);
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
</style>
