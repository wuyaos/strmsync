<template>
  <div class="run-events">
    <div class="run-events__filters">
      <el-select
        v-model="filters.kind"
        placeholder="类型"
        clearable
        size="small"
        style="width: 110px"
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
        style="width: 110px"
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
        style="width: 110px"
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
    <div v-if="loading" class="run-events__loading">加载中...</div>
    <div v-else-if="events.length === 0" class="run-events__empty">暂无详细执行日志</div>
    <div v-else class="run-events__list">
      <div v-for="event in events" :key="event.id" class="run-events__item">
        <el-tag :type="getStatusType(event.status)" size="small">
          {{ getOpText(event.op) }}
        </el-tag>
        <span class="run-events__kind">{{ getKindText(event.kind) }}</span>
        <span class="run-events__path">
          {{ event.source_path || '-' }}
          <span v-if="event.target_path" class="run-events__arrow">=> </span>
          {{ event.target_path || '-' }}
        </span>
        <span v-if="event.error_message" class="run-events__error">
          {{ event.error_message }}
        </span>
      </div>
    </div>
    <div v-if="hasMore" class="run-events__more">
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
.run-events {
  .run-events__loading,
  .run-events__empty {
    color: var(--el-text-color-secondary);
    font-size: 12px;
  }

  .run-events__filters {
    display: flex;
    gap: 8px;
    align-items: center;
    margin-bottom: 8px;
    flex-wrap: wrap;
  }

  .run-events__list {
    display: grid;
    row-gap: 6px;
  }

  .run-events__item {
    display: grid;
    grid-template-columns: 70px 60px 1fr;
    column-gap: 8px;
    align-items: start;
    font-size: 12px;
  }

  .run-events__kind {
    color: var(--el-text-color-secondary);
  }

  .run-events__path {
    font-family: var(--el-font-family-monospace, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace);
    word-break: break-all;
  }

  .run-events__arrow {
    margin: 0 4px;
    color: var(--el-text-color-secondary);
  }

  .run-events__error {
    grid-column: 3 / 4;
    color: var(--el-color-danger);
    font-size: 12px;
    word-break: break-all;
  }

  .run-events__more {
    margin-top: 6px;
  }
}
</style>
