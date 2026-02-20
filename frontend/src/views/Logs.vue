<template>
  <div class="logs-page page-with-pagination">
    <div class="page-header">
      <div>
        <h1 class="page-title">系统日志</h1>
        <p class="page-description">按级别和关键词筛选系统运行日志</p>
      </div>
    </div>

    <div class="toolbar">
      <el-input
        v-model="searchText"
        placeholder="搜索日志内容"
        clearable
        style="width: 300px"
      />

      <el-select
        v-model="levelFilter"
        placeholder="日志级别"
        clearable
        style="width: 150px"
      >
        <el-option label="全部" value="" />
        <el-option label="DEBUG" value="debug" />
        <el-option label="INFO" value="info" />
        <el-option label="WARN" value="warn" />
        <el-option label="ERROR" value="error" />
      </el-select>

      <el-select
        v-model="taskFilter"
        placeholder="任务"
        clearable
        style="width: 200px"
      >
        <el-option label="全部任务" value="" />
      </el-select>

      <div style="flex: 1"></div>

      <el-button @click="loadLogs">刷新</el-button>
      <el-button @click="handleCleanup" type="danger">清理日志</el-button>
    </div>

    <div class="log-container">
      <el-empty v-if="!loading && filteredLogs.length === 0" description="暂无日志" />

      <div v-else-if="!loading" class="log-list">
        <div
          v-for="log in filteredLogs"
          :key="log.id"
          :class="['log-item', `log-${log.level || 'info'}`]"
        >
          <span class="log-time">{{ formatTime(log.created_at) }}</span>
          <span class="log-level">{{ (log.level || 'info').toUpperCase() }}</span>
          <span v-if="log.module" class="log-module">{{ log.module }}</span>
          <span class="log-message">{{ log.message || '-' }}</span>
        </div>
      </div>

      <div v-else style="text-align: center; padding: 40px;">
        <el-icon class="is-loading" :size="40"><Loading /></el-icon>
        <p style="margin-top: 12px; color: var(--el-text-color-secondary);">加载中...</p>
      </div>

      <div v-if="total > 0" class="page-pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[50, 100, 200]"
          layout="total, sizes, prev, pager, next"
          @current-change="loadLogs"
          @size-change="loadLogs"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getLogList, cleanupLogs } from '@/api/log'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import dayjs from 'dayjs'

const searchText = ref('')
const levelFilter = ref('info')
const taskFilter = ref('')
const logs = ref([])
const loading = ref(false)
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(50)

const getLogTime = (log) => {
  const time = Date.parse(log?.created_at || '')
  if (!Number.isNaN(time)) return time
  return typeof log?.id === 'number' ? log.id : 0
}

const filteredLogs = computed(() => {
  const list = logs.value.filter(log => {
    if (searchText.value && !(log.message || '').includes(searchText.value)) {
      return false
    }
    if (levelFilter.value && log.level !== levelFilter.value) {
      return false
    }
    if (taskFilter.value && log.job_id !== parseInt(taskFilter.value)) {
      return false
    }
    return true
  })
  return list.slice().sort((a, b) => getLogTime(b) - getLogTime(a))
})

const loadLogs = async () => {
  try {
    loading.value = true
    const params = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (levelFilter.value) params.level = levelFilter.value
    if (taskFilter.value) params.job_id = taskFilter.value
    if (searchText.value) params.search = searchText.value

    const data = await getLogList(params)
    logs.value = data.logs || []
    total.value = data.total || 0
  } catch (error) {
    console.error('加载日志失败:', error)
    ElMessage.error('加载日志失败')
  } finally {
    loading.value = false
  }
}

const handleCleanup = async () => {
  try {
    await ElMessageBox.confirm(
      '将清理7天前的日志，是否继续？',
      '清理日志',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    await cleanupLogs({ days: 7 })
    ElMessage.success('日志清理成功')
    loadLogs()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('清理日志失败:', error)
      ElMessage.error('清理日志失败')
    }
  }
}

const formatTime = (time) => {
  if (!time) return '-'
  const parsed = dayjs(time)
  if (!parsed.isValid()) return '-'
  return parsed.format('YYYY-MM-DD HH:mm:ss')
}

onMounted(() => {
  loadLogs()
})
</script>

<style scoped lang="scss">
.logs-page {
  padding: 20px;
  height: calc(100vh - 80px);
  display: flex;
  flex-direction: column;

  .log-container {
    flex: 1;
    overflow-y: auto;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color);
    border-radius: 4px;
    padding: 12px;
    display: flex;
    flex-direction: column;

    .log-list {
      flex: 1;
      overflow-y: auto;
      font-family: 'Consolas', 'Monaco', monospace;
      font-size: 13px;

      .log-item {
        padding: 6px 12px;
        margin-bottom: 4px;
        border-radius: 3px;
        display: flex;
        gap: 12px;

        &:hover {
          background: var(--el-fill-color-light);
        }

        .log-time {
          color: var(--el-text-color-secondary);
          min-width: 160px;
        }

        .log-level {
          font-weight: 600;
          min-width: 60px;
        }

        .log-module {
          color: var(--el-text-color-secondary);
          min-width: 100px;
        }

        .log-message {
          flex: 1;
        }

        &.log-debug {
          .log-level {
            color: var(--el-color-info);
          }
        }

        &.log-info {
          .log-level {
            color: var(--el-color-primary);
          }
        }

        &.log-warn {
          .log-level {
            color: var(--el-color-warning);
          }
        }

        &.log-error {
          .log-level {
            color: var(--el-color-danger);
          }
          background: var(--el-color-danger-light-9);
        }
      }
    }

  }
}
</style>
