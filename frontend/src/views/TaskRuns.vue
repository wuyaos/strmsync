<template>
  <div class="runs-page">
    <div class="toolbar">
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
            <div class="expand-title">错误信息</div>
            <div class="expand-body">
              {{ row.error_message || row.error || '无' }}
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
      <el-table-column prop="finished_at" label="结束时间" width="160">
        <template #default="{ row }">
          {{ row.finished_at ? formatTime(row.finished_at) : '-' }}
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
      <el-table-column label="统计" min-width="160">
        <template #default="{ row }">
          {{ getStatsText(row) }}
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination">
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
import { normalizeListResponse } from '@/api/normalize'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

const loading = ref(false)
const runList = ref([])
const autoRefresh = ref(false)
let refreshTimer = null

const filters = reactive({
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
  const end = row.finished_at ? dayjs(row.finished_at) : dayjs()
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

const getStatsText = (row) => {
  const stats = row.stats || {}
  const processed = stats.processed || row.processed_count || row.processed || 0
  const created = stats.created || row.created_count || row.created || 0
  return `处理 ${processed}，生成 ${created}`
}

const loadRuns = async () => {
  loading.value = true
  try {
    const [from, to] = filters.timeRange || []
    const params = {
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
  loadRuns()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped lang="scss">
.runs-page {
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
    padding: 12px 16px;
    background: var(--el-bg-color);
    border-radius: 4px;
  }

  .pagination {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }

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
    white-space: pre-wrap;
    color: var(--el-text-color-regular);
  }
}
</style>
