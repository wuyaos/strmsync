<template>
  <div class="runs-page page-with-pagination">
    <div class="toolbar">
      <el-select
        v-model="filters.jobId"
        placeholder="任务"
        clearable
        filterable
        style="width: 200px"
        @change="handleSearch"
      >
        <el-option
          v-for="job in jobOptions"
          :key="job.id"
          :label="job.name"
          :value="job.id"
        />
      </el-select>
      <el-date-picker
        v-model="filters.timeRange"
        type="datetimerange"
        range-separator="至"
        start-placeholder="开始时间"
        end-placeholder="结束时间"
        value-format="YYYY-MM-DDTHH:mm:ss"
        @change="handleSearch"
      />
      <el-select
        v-model="filters.status"
        placeholder="状态"
        clearable
        style="width: 140px"
        @change="handleSearch"
      >
        <el-option label="成功" value="completed" />
        <el-option label="失败" value="failed" />
        <el-option label="运行中" value="running" />
        <el-option label="已取消" value="cancelled" />
      </el-select>
      <div style="flex: 1"></div>
      <el-switch v-model="autoRefresh" active-text="自动刷新" />
    </div>

    <el-table v-loading="loading" :data="runList" stripe style="width: 100%">
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-content">
            <div class="expand-title">任务配置</div>
            <div class="expand-body">
              <div class="kv-list">
                <div
                  v-for="item in getJobConfigRows(row)"
                  :key="item.label"
                  class="kv-row"
                >
                  <span class="kv-label">{{ item.label }}</span>
                  <span class="kv-value" :class="{ mono: item.mono }">
                    <pre v-if="item.pre" class="kv-pre">{{ item.value }}</pre>
                    <template v-else>{{ item.value }}</template>
                  </span>
                </div>
              </div>
            </div>
            <div class="expand-title">执行情况</div>
            <div class="expand-body">
              <div class="kv-list">
                <div
                  v-for="item in getExecutionRows(row)"
                  :key="item.label"
                  class="kv-row"
                >
                  <span class="kv-label">{{ item.label }}</span>
                  <span class="kv-value">{{ item.value }}</span>
                </div>
              </div>
            </div>
            <div class="expand-title">总结</div>
            <div class="expand-body">
              <div class="kv-list">
                <div
                  v-for="item in getSummaryRows(row)"
                  :key="item.label"
                  class="kv-row"
                >
                  <span class="kv-label">{{ item.label }}</span>
                  <span class="kv-value">{{ item.value }}</span>
                </div>
              </div>
            </div>
            <div class="expand-title">错误信息</div>
            <div class="expand-body">
              <div class="kv-list">
                <div class="kv-row">
                  <span class="kv-label">错误信息</span>
                  <span class="kv-value">
                    {{ row.error_message || row.error || '无' }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="job_name" label="任务名称" min-width="160">
        <template #default="{ row }">
          {{ row.job_name || row.job?.name || '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="started_at" label="开始时间" width="160">
        <template #default="{ row }">
          {{ row.started_at ? formatTime(row.started_at) : '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="ended_at" label="结束时间" width="160">
        <template #default="{ row }">
          {{ row.ended_at ? formatTime(row.ended_at) : '-' }}
        </template>
      </el-table-column>
      <el-table-column label="耗时" width="110">
        <template #default="{ row }">
          {{ getDuration(row) }}
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ getStatusText(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="STRM" min-width="160">
        <template #default="{ row }">
          {{ getStrmStatsText(row) }}
        </template>
      </el-table-column>
      <el-table-column label="元数据" min-width="180">
        <template #default="{ row }">
          {{ getMetaStatsText(row) }}
        </template>
      </el-table-column>
      <el-table-column label="总数" min-width="160">
        <template #default="{ row }">
          {{ getTotalStatsText(row) }}
        </template>
      </el-table-column>
    </el-table>

    <div class="page-pagination">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="pagination.total"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
      />
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'
import { getRunList } from '@/api/runs'
import { getJobList } from '@/api/jobs'
import { normalizeListResponse } from '@/api/normalize'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

const loading = ref(false)
const runList = ref([])
const jobOptions = ref([])
const autoRefresh = ref(false)
let refreshTimer = null

const filters = reactive({
  jobId: '',
  timeRange: [],
  status: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

const formatTime = (time) => {
  return dayjs(time).fromNow()
}

const getDuration = (row) => {
  if (!row.started_at) return '-'
  const end = row.ended_at ? dayjs(row.ended_at) : dayjs()
  const diff = end.diff(dayjs(row.started_at), 'second')
  if (diff < 60) return `${diff}s`
  if (diff < 3600) return `${Math.floor(diff / 60)}m`
  return `${Math.floor(diff / 3600)}h`
}

const getStatusType = (status) => {
  const map = {
    completed: 'success',
    failed: 'danger',
    running: 'primary',
    pending: 'info',
    cancelled: 'warning'
  }
  return map[status] || 'info'
}

const getStatusText = (status) => {
  const map = {
    completed: '成功',
    failed: '失败',
    running: '运行中',
    pending: '待执行',
    cancelled: '已取消'
  }
  return map[status] || status || '-'
}

const getStrmStatsText = (row) => {
  const created = (row.created_files ?? row.created) || 0
  const updated = (row.updated_files ?? row.updated) || 0
  const failed = (row.failed_files ?? row.failed) || 0
  return `生成 ${created}，更新 ${updated}，失败 ${failed}`
}

const getMetaStatsText = (row) => {
  const created = row.meta_created_files ?? 0
  const updated = row.meta_updated_files ?? 0
  const failed = row.meta_failed_files ?? 0
  return `复制 ${created}，更新 ${updated}，失败 ${failed}`
}

const getTotalStatsText = (row) => {
  const total = (row.total_files ?? row.total) || 0
  const failed = (row.failed_files ?? row.failed) || 0
  const skipped = row.skipped_files ?? 0
  return `总 ${total}，失败 ${failed}，跳过 ${skipped}`
}

const getExecutionRows = (row) => {
  const created = row.created_files ?? 0
  const updated = row.updated_files ?? 0
  const failed = row.failed_files ?? 0
  const metaCreated = row.meta_created_files ?? 0
  const metaUpdated = row.meta_updated_files ?? 0
  const metaFailed = row.meta_failed_files ?? 0
  return [
    { label: 'STRM', value: `生成 ${created}，更新 ${updated}，失败 ${failed}` },
    { label: '元数据', value: `复制 ${metaCreated}，更新 ${metaUpdated}，失败 ${metaFailed}` }
  ]
}

const getSummaryRows = (row) => {
  const total = row.total_files ?? 0
  const filtered = row.filtered_files ?? 0
  const processed = row.processed_files ?? 0
  const created = row.created_files ?? 0
  const updated = row.updated_files ?? 0
  const skipped = row.skipped_files ?? 0
  const failed = row.failed_files ?? 0
  const metaTotal = row.meta_total_files ?? 0
  const metaCreated = row.meta_created_files ?? 0
  const metaUpdated = row.meta_updated_files ?? 0
  const metaProcessed = row.meta_processed_files ?? 0
  const metaFailed = row.meta_failed_files ?? 0
  return [
    {
      label: '统计',
      value: `扫描 ${total}，过滤 ${filtered}，处理 ${processed}，生成 ${created}，更新 ${updated}，跳过 ${skipped}，失败 ${failed}`
    },
    {
      label: '元数据',
      value: `复制 ${metaCreated} / 更新 ${metaUpdated} / 失败 ${metaFailed}（总 ${metaTotal}，已处理 ${metaProcessed}）`
    }
  ]
}

const getJobConfigRows = (row) => {
  const job = row.job || {}
  const dataServer = job.data_server || {}
  const mediaServer = job.media_server || {}
  const dataServerType = dataServer.type || job.data_server_type || '-'
  const mediaServerType = mediaServer.type || job.media_server_type || '-'
  return [
    { label: '任务ID', value: job.id ?? row.job_id ?? '-' },
    { label: '任务名', value: job.name || row.job_name || '-' },
    { label: '数据服务器ID', value: job.data_server_id ?? '-' },
    { label: '数据服务系统', value: dataServerType || '-' },
    { label: '媒体服务器ID', value: job.media_server_id ?? '-' },
    { label: '媒体服务器类型', value: mediaServerType || '-' },
    { label: '访问目录', value: job.source_path || '-' },
    { label: '输出目录', value: job.target_path || '-' },
    { label: 'STRM路径', value: job.strm_path || '-' },
    {
      label: '选项',
      value: formatOptionsPretty(job.options),
      pre: true,
      mono: true
    }
  ]
}

const formatOptionsPretty = (raw) => {
  if (!raw) return '-'
  if (typeof raw !== 'string') {
    return JSON.stringify(raw, null, 2)
  }
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch (error) {
    return raw
  }
}

const loadRuns = async () => {
  loading.value = true
  try {
    const [from, to] = filters.timeRange || []
    const params = {
      job_id: filters.jobId || undefined,
      status: filters.status,
      from: from || undefined,
      to: to || undefined,
      page: pagination.page,
      pageSize: pagination.pageSize
    }
    const response = await getRunList(params)
    const { list, total } = normalizeListResponse(response)
    runList.value = list
    pagination.total = total
  } catch (error) {
    console.error('加载运行记录失败:', error)
  } finally {
    loading.value = false
  }
}

const loadJobs = async () => {
  try {
    const response = await getJobList({ page: 1, pageSize: 200 })
    const { list } = normalizeListResponse(response)
    jobOptions.value = list
  } catch (error) {
    console.error('加载任务列表失败:', error)
  }
}

const handleSearch = () => {
  pagination.page = 1
  loadRuns()
}

const handlePageChange = () => {
  loadRuns()
}

const handleSizeChange = () => {
  pagination.page = 1
  loadRuns()
}

const startAutoRefresh = () => {
  if (refreshTimer) return
  refreshTimer = setInterval(loadRuns, 30000)
}

const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

watch(autoRefresh, (enabled) => {
  if (enabled) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
})

onMounted(() => {
  loadJobs()
  loadRuns()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped lang="scss">
.runs-page {
  .expand-content {
    padding: 8px 12px;
    background: var(--el-fill-color-light);
    border-radius: 4px;
  }

  .expand-title {
    font-weight: 600;
    margin-bottom: 6px;
  }

  .expand-body {
    color: var(--el-text-color-regular);
  }

  .kv-list {
    display: grid;
    row-gap: 6px;
  }

  .kv-row {
    display: grid;
    grid-template-columns: 110px 1fr;
    column-gap: 8px;
    align-items: start;
  }

  .kv-label {
    color: var(--el-text-color-secondary);
  }

  .kv-value {
    word-break: break-all;
  }

  .kv-value.mono {
    font-family: var(--el-font-family-monospace, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace);
  }

  .kv-pre {
    margin: 0;
    white-space: pre-wrap;
  }
}
</style>
