<template>
  <div class="execution-detail">
    <div class="execution-filters">
      <el-select
        v-model="filters.kind"
        placeholder="类型"
        clearable
        size="small"
        class="w-[110px]"
        @change="reload"
      >
        <el-option label="STRM" value="strm" />
        <el-option label="元数据" value="meta" />
      </el-select>
      <el-select
        v-model="filters.op"
        placeholder="操作"
        clearable
        size="small"
        class="w-[110px]"
        @change="reload"
      >
        <el-option label="生成" value="create" />
        <el-option label="更新" value="update" />
        <el-option label="删除" value="delete" />
        <el-option label="复制" value="copy" />
        <el-option label="跳过" value="skip" />
      </el-select>
      <el-select
        v-model="filters.status"
        placeholder="状态"
        clearable
        size="small"
        class="w-[110px]"
        @change="reload"
      >
        <el-option label="成功" value="success" />
        <el-option label="失败" value="failed" />
        <el-option label="跳过" value="skipped" />
      </el-select>
      <el-button text size="small" :loading="loading" @click="reload">
        刷新
      </el-button>
    </div>
    <div v-if="loading" class="text-12 text-[var(--el-text-color-secondary)]">加载中...</div>
    <div v-else-if="events.length === 0" class="text-12 text-[var(--el-text-color-secondary)]">暂无详细执行日志</div>
    <div v-else class="execution-list">
      <div
        v-for="event in events"
        :key="event.id"
        class="execution-item"
      >
        <div class="execution-head">
          <el-tag :type="getStatusType(event.status)" size="small">
            {{ getOpText(event.op) }}
          </el-tag>
          <span class="kind">{{ getKindText(event.kind) }}</span>
        </div>
        <div class="execution-path">
          <span class="font-mono break-all">
            {{ event.source_path || '-' }}
            <span v-if="event.target_path" class="arrow">→</span>
            {{ event.target_path || '-' }}
          </span>
          <span v-if="event.error_message" class="error-message">
            {{ event.error_message }}
          </span>
        </div>
      </div>
    </div>
    <div v-if="hasMore" class="mt-8">
      <el-button text size="small" :loading="loadingMore" @click="loadMore">
        加载更多
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { getRunEvents } from '@/api/runs'

const props = defineProps({
  runId: {
    type: [String, Number],
    required: true
  }
})

const events = ref([])
const loading = ref(false)
const loadingMore = ref(false)
const page = ref(1)
const pageSize = 100
const total = ref(0)
const filters = ref({
  kind: '',
  op: '',
  status: ''
})

const hasMore = computed(() => events.value.length < total.value)

const loadEvents = async (targetPage = 1) => {
  if (!props.runId) return
  if (targetPage === 1) {
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const response = await getRunEvents(props.runId, {
      page: targetPage,
      page_size: pageSize,
      kind: filters.value.kind || undefined,
      op: filters.value.op || undefined,
      status: filters.value.status || undefined
    })
    const items = Array.isArray(response?.items) ? response.items : []
    if (targetPage === 1) {
      events.value = items
    } else {
      events.value = events.value.concat(items)
    }
    total.value = Number(response?.total || 0)
    page.value = targetPage
  } catch (error) {
    console.error('加载执行事件失败:', error)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

const loadMore = () => {
  if (!hasMore.value || loadingMore.value) return
  loadEvents(page.value + 1)
}

const reload = () => {
  events.value = []
  total.value = 0
  page.value = 1
  loadEvents(1)
}

const getStatusType = (status) => {
  const map = {
    success: 'success',
    failed: 'danger',
    skipped: 'info'
  }
  return map[status] || 'info'
}

const getOpText = (op) => {
  const map = {
    create: '生成',
    update: '更新',
    delete: '删除',
    copy: '复制',
    skip: '跳过',
    unknown: '未知'
  }
  return map[op] || op || '-'
}

const getKindText = (kind) => {
  const map = {
    strm: 'STRM',
    meta: '元数据'
  }
  return map[kind] || kind || '-'
}

watch(
  () => props.runId,
  () => {
    reload()
  }
)

onMounted(() => {
  loadEvents(1)
})
</script>

<style scoped lang="scss">
.execution-detail {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.execution-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.execution-list {
  display: grid;
  gap: 10px;
}

.execution-item {
  display: grid;
  grid-template-columns: 160px 1fr;
  gap: 8px 12px;
  align-items: start;
  padding: 8px 10px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  background: var(--el-fill-color-blank);
  font-size: 12px;
}

.execution-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.kind {
  color: var(--el-text-color-secondary);
}

.execution-path {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.arrow {
  margin: 0 6px;
  color: var(--el-text-color-secondary);
}

.error-message {
  color: var(--el-color-danger);
  font-size: 12px;
}
</style>
