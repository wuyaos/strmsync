<template>
  <div class="dashboard-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">仪表盘</h1>
    </div>

    <!-- KPI 统计卡片 -->
    <div class="dashboard-section">
      <DashboardKpiRow :stats="stats" />
    </div>

    <!-- 图表区 -->
    <div class="dashboard-section">
      <el-row :gutter="32" class="dashboard-row">
        <el-col :xs="24" :md="12">
        <el-card shadow="hover" class="chart-card">
          <template #header>
            <div class="card-header">
              <span>近7日运行结果</span>
            </div>
          </template>
          <div ref="runTrendRef" class="chart-container"></div>
        </el-card>
      </el-col>
        <el-col :xs="24" :md="12">
        <el-card shadow="hover" class="chart-card">
          <template #header>
            <div class="card-header">
              <span>运行耗时分布</span>
            </div>
          </template>
          <div ref="durationDistRef" class="chart-container"></div>
        </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- 最近运行记录 -->
    <div class="dashboard-section">
      <DashboardRecentRuns
        :run-list="runList"
        :get-run-type="getRunType"
        :format-time="formatTime"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import DashboardKpiRow from '@/components/dashboard/DashboardKpiRow.vue'
import DashboardRecentRuns from '@/components/dashboard/DashboardRecentRuns.vue'
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
  serverTotal: 0,
  serverEnabled: 0,
  serverDisabled: 0,
  jobTotal: 0,
  jobEnabled: 0,
  jobDisabled: 0,
  recentRuns: 0,
  failedRuns: 0
})

// 运行记录列表
const runList = ref([])
const runTrendRef = ref(null)
const durationDistRef = ref(null)
let runTrendChart = null
let durationDistChart = null
let echartsModule = null

let refreshTimer = null
let resizeHandler = null

// 加载数据
const loadData = async () => {
  try {
    const now = dayjs()
    const lastHour = now.subtract(1, 'hour').toISOString()
    const last24h = now.subtract(24, 'hour').toISOString()

    const [
      dataServersTotalResp,
      dataServersEnabledResp,
      dataServersDisabledResp,
      mediaServersTotalResp,
      mediaServersEnabledResp,
      mediaServersDisabledResp,
      jobsTotalResp,
      jobsEnabledResp,
      jobsDisabledResp,
      runsResp,
      recentRunsResp,
      failedRunsResp
    ] = await Promise.all([
      getServerList({ type: 'data', page: 1, pageSize: 1 }),
      getServerList({ type: 'data', enabled: 'true', page: 1, pageSize: 1 }),
      getServerList({ type: 'data', enabled: 'false', page: 1, pageSize: 1 }),
      getServerList({ type: 'media', page: 1, pageSize: 1 }),
      getServerList({ type: 'media', enabled: 'true', page: 1, pageSize: 1 }),
      getServerList({ type: 'media', enabled: 'false', page: 1, pageSize: 1 }),
      getJobList({ page: 1, pageSize: 1 }),
      getJobList({ enabled: 'true', page: 1, pageSize: 1 }),
      getJobList({ enabled: 'false', page: 1, pageSize: 1 }),
      getRunList({ page: 1, pageSize: 200 }),
      getRunList({ from: lastHour, to: now.toISOString(), page: 1, pageSize: 1 }),
      getRunList({ status: 'failed', from: last24h, to: now.toISOString(), page: 1, pageSize: 1 })
    ])

    const dataServersTotal = normalizeListResponse(dataServersTotalResp)
    const dataServersEnabled = normalizeListResponse(dataServersEnabledResp)
    const dataServersDisabled = normalizeListResponse(dataServersDisabledResp)
    const mediaServersTotal = normalizeListResponse(mediaServersTotalResp)
    const mediaServersEnabled = normalizeListResponse(mediaServersEnabledResp)
    const mediaServersDisabled = normalizeListResponse(mediaServersDisabledResp)
    const jobsTotal = normalizeListResponse(jobsTotalResp)
    const jobsEnabled = normalizeListResponse(jobsEnabledResp)
    const jobsDisabled = normalizeListResponse(jobsDisabledResp)
    const runsResult = normalizeListResponse(runsResp)
    const recentRunsResult = normalizeListResponse(recentRunsResp)
    const failedRunsResult = normalizeListResponse(failedRunsResp)

    runList.value = runsResult.list.slice(0, 10)

    stats.value.serverTotal = dataServersTotal.total || dataServersTotal.list.length
    stats.value.serverEnabled = dataServersEnabled.total || dataServersEnabled.list.length
    stats.value.serverDisabled = dataServersDisabled.total || dataServersDisabled.list.length
    stats.value.jobTotal = jobsTotal.total || jobsTotal.list.length
    stats.value.jobEnabled = jobsEnabled.total || jobsEnabled.list.length
    stats.value.jobDisabled = jobsDisabled.total || jobsDisabled.list.length
    stats.value.recentRuns = recentRunsResult.total || recentRunsResult.list.length
    stats.value.failedRuns = failedRunsResult.total || failedRunsResult.list.length

    await nextTick()
    await renderCharts(runsResult.list)
  } catch (error) {
    console.error('加载数据失败:', error)
  }
}

const buildRunTrendData = (runs) => {
  const days = []
  const dayKeys = []
  for (let i = 6; i >= 0; i -= 1) {
    const day = dayjs().subtract(i, 'day')
    days.push(day.format('MM-DD'))
    dayKeys.push(day.format('YYYY-MM-DD'))
  }
  const success = Array(7).fill(0)
  const failed = Array(7).fill(0)
  const keyIndex = new Map(dayKeys.map((key, idx) => [key, idx]))

  for (const run of runs || []) {
    if (!run?.started_at) continue
    const key = dayjs(run.started_at).format('YYYY-MM-DD')
    const idx = keyIndex.get(key)
    if (idx === undefined) continue
    if (run.status === 'completed') success[idx] += 1
    if (run.status === 'failed') failed[idx] += 1
  }
  return { days, success, failed }
}

const buildDurationBuckets = (runs) => {
  const buckets = [
    { label: '0-5m', max: 300 },
    { label: '5-15m', max: 900 },
    { label: '15-30m', max: 1800 },
    { label: '30-60m', max: 3600 },
    { label: '60m+', max: Infinity }
  ]
  const counts = Array(buckets.length).fill(0)
  for (const run of runs || []) {
    const sec = Number(run?.duration || 0)
    if (!Number.isFinite(sec) || sec <= 0) continue
    const idx = buckets.findIndex(bucket => sec <= bucket.max)
    if (idx >= 0) counts[idx] += 1
  }
  return { labels: buckets.map(b => b.label), counts }
}

const ensureEcharts = async () => {
  if (echartsModule) return echartsModule
  const module = await import('@/utils/echarts')
  echartsModule = module.default
  return module.default
}

const renderCharts = async (runs) => {
  const echarts = await ensureEcharts()
  if (runTrendRef.value && !runTrendChart) {
    runTrendChart = echarts.init(runTrendRef.value)
  }
  if (durationDistRef.value && !durationDistChart) {
    durationDistChart = echarts.init(durationDistRef.value)
  }
  if (!runTrendChart || !durationDistChart) return

  const trend = buildRunTrendData(runs)
  runTrendChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['成功', '失败'] },
    grid: { left: 32, right: 16, top: 32, bottom: 24, containLabel: true },
    xAxis: { type: 'category', data: trend.days },
    yAxis: { type: 'value' },
    series: [
      { name: '成功', type: 'line', data: trend.success, smooth: true },
      { name: '失败', type: 'line', data: trend.failed, smooth: true }
    ]
  }, true)

  const duration = buildDurationBuckets(runs)
  durationDistChart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 32, right: 16, top: 32, bottom: 24, containLabel: true },
    xAxis: { type: 'category', data: duration.labels },
    yAxis: { type: 'value' },
    series: [
      { name: '数量', type: 'bar', data: duration.counts, barMaxWidth: 32 }
    ]
  }, true)
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

  resizeHandler = () => {
    runTrendChart?.resize()
    durationDistChart?.resize()
  }
  window.addEventListener('resize', resizeHandler)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  runTrendChart?.dispose()
  durationDistChart?.dispose()
  runTrendChart = null
  durationDistChart = null
  echartsModule = null
  if (resizeHandler) {
    window.removeEventListener('resize', resizeHandler)
    resizeHandler = null
  }
})
</script>

<style scoped lang="scss">
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 12px;

  .dashboard-section {
    width: 100%;
  }

  .dashboard-row {
    row-gap: 12px;
  }


  .chart-card {
    :deep(.el-card__body) {
      overflow: hidden;
    }
  }

  .chart-container {
    height: 210px;
    width: 100%;
    overflow: hidden;
  }

}
</style>
