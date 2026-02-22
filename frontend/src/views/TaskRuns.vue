<template>
  <div class="runs-page flex flex-col min-h-[calc(100vh-80px)]">
    <div class="page-header">
      <div>
        <h1 class="page-title">执行历史</h1>
        <p class="page-description">查看任务执行记录与详细结果</p>
      </div>
    </div>
    <TaskRunsToolbar
      :filters="filters"
      :job-options="jobOptions"
      :selected-count="selectedRunIds.length"
      @update:filters="updateFilters"
      @search="handleSearch"
      @batch-delete="handleBatchDelete"
    />

    <el-table
      v-loading="loading"
      :data="runList"
      stripe
      row-key="id"
      :expand-row-keys="expandedRowKeys"
      :reserve-selection="true"
      style="width: 100%"
      @selection-change="handleSelectionChange"
      @expand-change="handleExpandChange"
    >
      <el-table-column type="selection" width="48" />
      <el-table-column type="expand">
        <template #default="{ row }">
          <TaskRunExpandPanel
            :row="row"
            :get-job-config-rows="getJobConfigRows"
            :get-summary-rows="getSummaryRows"
          />
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
      <el-table-column label="操作" width="90" fixed="right">
        <template #default="{ row }">
          <el-tooltip content="删除记录" placement="top">
            <el-button
              size="default"
              type="danger"
              :disabled="row.status === 'running' || row.status === 'pending'"
              :icon="Delete"
              class="action-button-large"
              @click="handleDeleteRun(row)"
            />
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <div class="mt-auto pt-16 pb-24 flex justify-end">
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
import Delete from '~icons/ep/delete'
import { batchDeleteRuns, deleteRun, getRunList } from '@/api/runs'
import { getJobList } from '@/api/jobs'
import { normalizeListResponse } from '@/api/normalize'
import TaskRunExpandPanel from '@/components/runs/TaskRunExpandPanel.vue'
import TaskRunsToolbar from '@/components/runs/TaskRunsToolbar.vue'
import { confirmDialog } from '@/composables/useConfirmDialog'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

const loading = ref(false)
const runList = ref([])
const jobOptions = ref([])
const autoRefresh = ref(true)
let refreshTimer = null
const expandedRowKeys = ref([])
const selectedRunIds = ref([])

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

const updateFilters = (next) => {
  Object.assign(filters, next || {})
}

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
  const options = parseOptions(job.options || row.options)
  const excludeDirs = Array.isArray(options.exclude_dirs) ? options.exclude_dirs : []
  return [
    { label: '任务ID', value: job.id ?? row.job_id ?? '-', compact: true },
    { label: '任务名', value: job.name || row.job_name || '-', compact: true },
    { label: '数据服务器ID', value: job.data_server_id ?? '-', compact: true },
    { label: '数据服务系统', value: dataServerType || '-', compact: true },
    { label: '媒体服务器ID', value: job.media_server_id ?? '-', compact: true },
    { label: '媒体服务器类型', value: mediaServerType || '-', compact: true },
    { label: '访问目录', value: job.source_path || '-' },
    { label: '远程根目录', value: job.remote_root || '-' },
    { label: '输出目录', value: job.target_path || '-' },
    { label: 'STRM路径', value: job.strm_path || '-' },
    { label: '排除目录', value: excludeDirs.length ? excludeDirs.join('、') : '-' },
    { label: 'STRM模式', value: formatStrmMode(options.strm_mode) },
    { label: '元数据模式', value: formatMetadataMode(options.metadata_mode) },
    { label: '线程数', value: options.thread_count ?? '-' , compact: true },
    { label: '清理选项', value: formatCleanupOpts(options.cleanup_opts) },
    { label: '同步策略', value: formatSyncOpts(options.sync_opts) },
    { label: '替换规则', value: formatReplaceRules(options.strm_replace_rules) }
  ]
}

const parseOptions = (raw) => {
  if (!raw) return {}
  if (typeof raw === 'object') return raw
  try {
    return JSON.parse(raw)
  } catch (error) {
    return {}
  }
}

const formatStrmMode = (mode) => {
  if (mode === 'url') return '远程 URL'
  if (mode === 'local') return '本地路径'
  return '-'
}

const formatMetadataMode = (mode) => {
  const map = {
    copy: '复制',
    download: '下载',
    none: '不生成'
  }
  return map[mode] || '-'
}

const formatCleanupOpts = (opts) => {
  if (!Array.isArray(opts) || opts.length === 0) return '-'
  return opts.join('、')
}

const formatSyncOpts = (opts) => {
  if (!opts || typeof opts !== 'object') return '-'
  const map = {
    full_resync: '全量同步',
    full_update: '更新同步',
    update_strm: '更新STRM',
    update_meta: '更新元数据',
    skip_strm: '跳过STRM',
    overwrite_meta: '元数据覆盖',
    skip_meta: '跳过元数据'
  }
  const enabled = Object.keys(map).filter((key) => opts[key])
  return enabled.length ? enabled.map((key) => map[key]).join('、') : '-'
}

const formatReplaceRules = (rules) => {
  if (!Array.isArray(rules) || rules.length === 0) return '-'
  return `${rules.length} 条`
}

const loadRuns = async (silent = false) => {
  if (!silent) {
    loading.value = true
  }
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
    if (selectedRunIds.value.length > 0) {
      const existing = new Set(list.map((item) => item.id))
      selectedRunIds.value = selectedRunIds.value.filter((id) => existing.has(id))
    }
  } catch (error) {
    console.error('加载运行记录失败:', error)
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

const handleBatchDelete = async () => {
  if (selectedRunIds.value.length === 0) return
  const confirmed = await confirmDialog({
    title: '删除执行记录',
    message: `将删除选中的 ${selectedRunIds.value.length} 条执行记录，且关联的明细日志也会删除，是否继续？`,
    type: 'error',
    items: selectedRunIds.value.map((id) => `ID:${id}`),
    confirmText: '确认删除',
    cancelText: '取消'
  })
  if (!confirmed) return
  try {
    await batchDeleteRuns(selectedRunIds.value)
    selectedRunIds.value = []
    loadRuns()
  } catch (error) {
    console.error('批量删除执行记录失败:', error)
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

const handleExpandChange = (_row, expandedRows) => {
  expandedRowKeys.value = expandedRows.map((item) => item.id)
}

const handleSelectionChange = (selection) => {
  selectedRunIds.value = selection.map((item) => item.id)
}

const handleDeleteRun = async (row) => {
  const confirmed = await confirmDialog({
    title: '删除执行记录',
    message: '该操作不可恢复，且关联的明细日志也会删除，是否继续？',
    type: 'error',
    items: [`ID:${row.id}`],
    confirmText: '确认删除',
    cancelText: '取消'
  })
  if (!confirmed) return
  try {
    await deleteRun(row.id)
    loadRuns(true)
  } catch (error) {
    console.error('删除执行记录失败:', error)
  }
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
  refreshTimer = setInterval(() => {
    loadRuns(true)
  }, 30000)
}

const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

watch(
  autoRefresh,
  (enabled) => {
    if (enabled) {
      startAutoRefresh()
    } else {
      stopAutoRefresh()
    }
  },
  { immediate: true }
)

onMounted(() => {
  loadJobs()
  loadRuns()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped lang="scss">
.action-button-large {
  font-size: 14px;
  padding: 6px 8px;
}
</style>

<style scoped lang="scss"></style>
