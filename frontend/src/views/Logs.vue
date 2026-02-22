<template>
  <div class="logs-page flex flex-col min-h-[calc(100vh-80px)] p-20">
    <div class="page-header">
      <div>
        <h1 class="page-title">系统日志</h1>
        <p class="page-description">按级别和关键词筛选系统运行日志</p>
      </div>
    </div>
    <div class="log-panel">
      <div class="log-panel-toolbar">
        <el-input
          v-model="searchText"
          placeholder="搜索日志内容"
          clearable
          class="toolbar-search"
        />

        <el-select
          v-model="levelFilter"
          placeholder="日志级别"
          clearable
          class="toolbar-level"
        >
          <el-option label="标准" value="standard" />
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
          class="toolbar-task"
        >
          <el-option label="全部任务" value="" />
        </el-select>
      </div>

      <div class="log-panel-body">
        <div class="log-viewer">
          <el-empty v-if="!loading && filteredLogs.length === 0" description="暂无日志" />

          <div v-else-if="!loading" class="log-list">
            <div
              v-for="log in filteredLogs"
              :key="log.id"
              :class="['log-item', `log-${(log.level || 'info').toLowerCase()}`]"
            >
              <span class="log-time">{{ formatTime(log.created_at) }}</span>
              <span class="log-level-badge">{{ (log.level || 'info').toUpperCase() }}</span>
              <span v-if="log.module" class="log-module">{{ log.module }}</span>
              <span class="log-message">{{ log.message || '-' }}</span>
            </div>
          </div>

          <div v-else class="log-loading">
            <el-icon class="is-loading" :size="40"><Loading /></el-icon>
            <p>加载中...</p>
          </div>
        </div>
      </div>

      <div class="log-panel-footer">
        <div class="log-status">当前显示 {{ filteredLogs.length }} 条 / 总计 {{ total }} 条</div>
        <div v-if="total > 0" class="flex justify-end">
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getLogList } from '@/api/log'
import { ElMessage } from 'element-plus'
import Loading from '~icons/ep/loading'
import dayjs from 'dayjs'

const searchText = ref('')
const levelFilter = ref('standard')
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
    const level = String(log.level || 'info').toLowerCase()
    if (searchText.value && !(log.message || '').includes(searchText.value)) {
      return false
    }
    if (levelFilter.value === 'standard') {
      if (!['info', 'warn', 'error'].includes(level)) return false
    } else if (levelFilter.value) {
      if (level !== levelFilter.value) return false
    }
    if (taskFilter.value && log.job_id !== parseInt(taskFilter.value, 10)) {
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
    if (levelFilter.value && levelFilter.value !== 'standard') params.level = levelFilter.value
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
  .log-panel {
    flex: 1;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-light);
    border-radius: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;

    .log-panel-toolbar {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 12px 20px;
      border-bottom: 1px solid var(--el-border-color-lighter);
      background: var(--el-fill-color-blank);

      .toolbar-search {
        width: 300px;
      }

      .toolbar-level {
        width: 150px;
      }

      .toolbar-task {
        width: 200px;
      }
    }

    .log-panel-body {
      flex: 1;
      min-height: 0;
      padding: 12px 20px;
      display: flex;
    }

    .log-viewer {
      flex: 1;
      min-height: 0;
      background: var(--el-fill-color-blank);
      border: 1px solid var(--el-border-color-lighter);
      border-radius: 0;
      padding: 6px 0;
      overflow-y: auto;

      .log-loading {
        text-align: center;
        padding: 40px 0;
        color: var(--el-text-color-secondary);
      }
    }

    .log-list {
      font-family: 'Consolas', 'Monaco', monospace;
      font-size: 12px;
      line-height: 1.6;

      .log-item {
        display: flex;
        align-items: flex-start;
        gap: 12px;
        padding: 4px 16px;
        border-bottom: 1px solid var(--el-border-color-extra-light);

        &:hover {
          background: var(--el-fill-color-light);
        }

        &:last-child {
          border-bottom: none;
        }

        .log-time {
          color: var(--el-text-color-secondary);
          min-width: 190px;
          font-variant-numeric: tabular-nums;
        }

        .log-level-badge {
          min-width: 56px;
          padding: 2px 8px;
          border-radius: 0;
          font-size: 12px;
          font-weight: 600;
          text-align: center;
          line-height: 1.2;
        }

        .log-module {
          color: var(--el-text-color-secondary);
          min-width: 120px;
        }

        .log-message {
          flex: 1;
        }

        &.log-debug {
          .log-level-badge {
            color: var(--el-color-info);
            background: var(--el-color-info-light-9);
            border: 1px solid var(--el-color-info-light-7);
          }
        }

        &.log-info {
          .log-level-badge {
            color: var(--el-color-primary);
            background: var(--el-color-primary-light-9);
            border: 1px solid var(--el-color-primary-light-7);
          }
        }

        &.log-warn {
          .log-level-badge {
            color: var(--el-color-warning);
            background: var(--el-color-warning-light-9);
            border: 1px solid var(--el-color-warning-light-7);
          }
        }

        &.log-error {
          .log-level-badge {
            color: var(--el-color-danger);
            background: var(--el-color-danger-light-9);
            border: 1px solid var(--el-color-danger-light-7);
          }
        }
      }
    }

    .log-panel-footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      padding: 10px 20px;
      border-top: 1px solid var(--el-border-color-lighter);
      background: var(--el-fill-color-blank);
      flex-wrap: wrap;

      .log-status {
        font-size: 12px;
        color: var(--el-text-color-secondary);
      }
    }
  }
}
</style>
