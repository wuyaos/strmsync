<template>
  <div class="servers-page page-with-pagination">
    <el-tabs v-model="activeTab" @tab-change="handleTabChange">
      <el-tab-pane label="数据服务器" name="data" />
      <el-tab-pane label="媒体服务器" name="media" />
    </el-tabs>

    <div class="toolbar">
      <el-input
        v-model="filters.keyword"
        placeholder="搜索名称/主机"
        :prefix-icon="Search"
        clearable
        class="w-320"
        @keyup.enter="handleSearch"
      />
      <div class="flex-1"></div>
      <el-button
        :type="batchMode ? 'warning' : 'default'"
        @click="batchMode ? clearSelection() : (batchMode = true)"
      >
        {{ batchMode ? '退出批量' : '批量操作' }}
      </el-button>
      <el-button type="primary" :icon="Plus" @click="handleAdd">
        新增服务器
      </el-button>
    </div>

    <!-- 批量操作工具栏 -->
    <div v-show="batchMode" class="batch-toolbar">
      <el-checkbox
        :model-value="isAllSelected(serverList)"
        :indeterminate="selectedCount > 0 && !isAllSelected(serverList)"
        @change="(val) => toggleSelectAll(serverList, val)"
      >
        全选
      </el-checkbox>
      <span class="selected-count">已选择 {{ selectedCount }} 项</span>
      <div class="flex-1"></div>
      <el-button
        :disabled="selectedCount === 0"
        @click="handleBatchEnable(serverList, updateServer, loadServers)"
      >
        批量启用
      </el-button>
      <el-button
        :disabled="selectedCount === 0"
        @click="handleBatchDisable(serverList, updateServer, loadServers)"
      >
        批量禁用
      </el-button>
      <el-button
        type="danger"
        :disabled="selectedCount === 0"
        @click="handleBatchDelete(serverList, deleteServer, loadServers)"
      >
        批量删除
      </el-button>
    </div>

    <!-- 卡片网格 -->
    <div
      v-loading="loading"
      class="server-grid"
      role="listbox"
      :aria-multiselectable="batchMode"
      aria-label="服务器列表"
    >
      <el-card
        v-for="server in serverList"
        :key="server.id"
        shadow="hover"
        class="server-card"
        :class="{ 'is-selected': isSelected(server), 'is-batch-mode': batchMode }"
        tabindex="0"
        :aria-selected="isSelected(server)"
        role="option"
        :aria-label="`${server.name} - ${server.type} - ${server.enabled ? '已启用' : '已禁用'}`"
        @click="handleCardClick(server)"
        @keydown.enter="handleCardClick(server)"
        @keydown.space.prevent="handleCardClick(server)"
      >
        <div class="card-header">
          <div v-if="batchMode" class="checkbox-wrapper">
            <el-checkbox
              :model-value="isSelected(server)"
              @click.stop
              @change="toggleSelect(server)"
            />
          </div>
          <div class="server-icon">
            <img
              v-if="getServerIconUrl(server)"
              :src="getServerIconUrl(server)"
              :alt="`${server.name} 图标`"
              class="server-icon-image"
            />
            <el-icon v-else :size="20">
              <component :is="getServerIcon(server.type)" />
            </el-icon>
          </div>
          <span class="server-name">{{ server.name }}</span>
          <span
            v-if="server.type !== 'local'"
            class="status-dot"
            :class="getConnectionStatus(server)"
            :title="server.enabled ? '已启用' : '已禁用'"
            :aria-label="server.enabled ? '已启用' : '已禁用'"
            role="status"
          />
        </div>
        <div class="card-body">
          <div class="info-row">
            <span class="label">类型：</span>
            <el-tag size="small">{{ server.type }}</el-tag>
          </div>
          <div class="info-row">
            <span class="label">地址：</span>
            <span class="value">{{ server.host || '-' }}{{ server.port ? ':' + server.port : '' }}</span>
          </div>
          <div class="info-row">
            <span class="label">状态：</span>
            <el-switch :model-value="server.enabled" size="small" disabled />
          </div>
          <div class="info-row">
            <span class="label">创建：</span>
            <span class="value">{{ server.created_at ? formatTime(server.created_at) : '-' }}</span>
          </div>
          <div class="card-divider"></div>
          <div class="card-actions">
            <el-button
              size="default"
              text
              :icon="Link"
              :disabled="server.type === 'local'"
              class="action-button"
              @click.stop="handleTest(server)"
            >
              测试
            </el-button>
            <el-button
              size="default"
              text
              :icon="Edit"
              class="action-button"
              @click.stop="handleEdit(server)"
            >
              编辑
            </el-button>
            <el-button
              size="default"
              text
              type="danger"
              :icon="Delete"
              class="action-button"
              @click.stop="handleDelete(server)"
            >
              删除
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <ListPagination
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :total="pagination.total"
      :page-sizes="[12, 24, 48]"
      @change="loadServers"
    />
    <ServerFormDialog
      v-model="dialogVisible"
      :mode="activeTab"
      :editing-server="editingServer"
      :data-type-defs="dataServerTypeDefs"
      :media-type-options="mediaServerTypeOptions"
      :server-list="serverList"
      @saved="handleFormSaved"
    />
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import Cloudy from '~icons/ep/cloudy'
import Delete from '~icons/ep/delete'
import Edit from '~icons/ep/edit'
import Link from '~icons/ep/link'
import Monitor from '~icons/ep/monitor'
import Plus from '~icons/ep/plus'
import Search from '~icons/ep/search'
import VideoPlay from '~icons/ep/video-play'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'
import {
  deleteServer,
  getServerList,
  getServerTypes,
  testServer,
  testServerSilent,
  updateServer
} from '@/api/servers'
import { normalizeListResponse } from '@/api/normalize'
import { useServerBatch } from '@/composables/useServerBatch'
import ListPagination from '@/components/common/ListPagination.vue'
import ServerFormDialog from '@/components/servers/ServerFormDialog.vue'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

// 批量操作 composable
const {
  selectedIds,
  batchMode,
  selectedCount,
  isAllSelected,
  toggleSelect,
  toggleSelectAll,
  clearSelection,
  isSelected,
  handleBatchEnable,
  handleBatchDisable,
  handleBatchDelete
} = useServerBatch()

const activeTab = ref('data')
const loading = ref(false)
const serverList = ref([])

const filters = reactive({
  keyword: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 12,
  total: 0
})

// 连接状态缓存
const connectionStatusMap = reactive({})
const pollingTimer = ref(null)
const pollingInFlight = ref(false)
const pollingIntervalMs = 10000
const lastTestAtMap = reactive({})
const inFlightKeyMap = reactive({})
const maxConcurrentTests = 3

const dialogVisible = ref(false)
const editingServer = ref(null)

// 数据服务器类型定义（从后端加载）
const dataServerTypeDefs = ref([])

const mediaServerTypeOptions = [
  {
    label: 'Emby',
    value: 'emby',
    description: '媒体服务器',
    defaultPort: 8096,
    needsApiKey: true,
    apiKeyLabel: 'API Key',
    hint: 'Emby Server，需要在设置中生成 API 密钥'
  },
  {
    label: 'Jellyfin',
    value: 'jellyfin',
    description: '开源媒体服务器',
    defaultPort: 8096,
    needsApiKey: true,
    apiKeyLabel: 'API Key',
    hint: 'Jellyfin Server，需要在设置中生成 API 密钥'
  },
  {
    label: 'Plex',
    value: 'plex',
    description: '媒体服务器',
    defaultPort: 32400,
    needsApiKey: true,
    apiKeyLabel: 'X-Plex-Token',
    hint: 'Plex Media Server，需要获取 X-Plex-Token'
  }
]

const formatTime = (time) => {
  return dayjs(time).fromNow()
}

const loadServers = async () => {
  loading.value = true
  try {
    const params = {
      type: activeTab.value,
      keyword: filters.keyword,
      page: pagination.page,
      pageSize: pagination.pageSize
    }
    const response = await getServerList(params)
    const { list, total } = normalizeListResponse(response)
    serverList.value = list
    pagination.total = total
    await refreshConnectionStatus()
  } catch (error) {
    console.error('加载服务器列表失败:', error)
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  pagination.page = 1
  clearSelection() // 搜索时清空选择
  loadServers()
}

const handleTabChange = () => {
  pagination.page = 1
  filters.keyword = ''
  clearSelection() // 切换 tab 时清空选择
  loadServers()
}

const handleAdd = async () => {
  await loadDataServerTypes(true)
  editingServer.value = null
  dialogVisible.value = true
}

const handleEdit = async (row) => {
  await loadDataServerTypes(true)
  editingServer.value = row
  dialogVisible.value = true
}

const handleFormSaved = () => {
  const isCreate = !editingServer.value
  editingServer.value = null
  if (isCreate) {
    pagination.page = 1
  }
  loadServers()
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除服务器「${row.name}」吗？`, '提示', {
      type: 'warning'
    })
    await deleteServer(row.id, row.type)
    ElMessage.success('服务器已删除')
    loadServers()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除服务器失败:', error)
    }
  }
}

const handleTest = async (row) => {
  const loadingMsg = ElMessage.info('正在测试连接...')
  try {
    await testServer(row.id, row.type)
    loadingMsg.close()
    connectionStatusMap[row.id] = 'success'
    ElMessage.success('连接测试成功')
  } catch (error) {
    loadingMsg.close()
    connectionStatusMap[row.id] = 'error'
    // 错误已由拦截器处理
  }
}

// 加载数据服务器类型定义
const loadDataServerTypes = async (force = false) => {
  if (!force && dataServerTypeDefs.value.length > 0) return
  try {
    const response = await getServerTypes({ category: 'data' })
    dataServerTypeDefs.value = response?.types || []
  } catch (error) {
    console.error('加载服务器类型失败:', error)
    ElMessage.error('加载服务器类型定义失败')
  }
}

// ============ 卡片网格辅助函数 ============

// 获取服务器类型图标
const getServerIcon = (type) => {
  const iconMap = {
    local: Monitor,
    clouddrive2: Cloudy,
    openlist: Cloudy,
    emby: VideoPlay,
    jellyfin: VideoPlay,
    plex: VideoPlay
  }
  return iconMap[type] || Monitor
}

const localSvgModules = import.meta.glob('/src/assets/icons/*.svg', {
  eager: true,
  import: 'default'
})
const localPngModules = import.meta.glob('/src/assets/icons/*.png', {
  eager: true,
  import: 'default'
})

const getLocalIconUrl = (name) => {
  if (!name) return ''
  const svgKey = `/src/assets/icons/${name}.svg`
  const pngKey = `/src/assets/icons/${name}.png`
  return localSvgModules[svgKey] || localPngModules[pngKey] || ''
}

const getServerIconUrl = (server) => {
  const typeName = server?.type ? String(server.type).toLowerCase() : ''
  const localIcon = getLocalIconUrl(typeName)
  return localIcon || server?.icon || server?.icon_url || server?.ico || server?.favicon || ''
}

// 获取连接状态
const getConnectionStatus = (server) => {
  if (!server.enabled) return 'status-disabled'
  const cached = connectionStatusMap[server.id]
  if (cached) return `status-${cached}`
  return 'status-unknown'
}

const refreshConnectionStatus = async () => {
  if (pollingInFlight.value) return
  const targets = serverList.value.filter(server => server.enabled && server.type !== 'local')
  if (targets.length === 0) return

  pollingInFlight.value = true
  try {
    const now = Date.now()
    const keyMap = new Map()
    for (const server of targets) {
      const host = String(server.host || '').trim()
      const port = server.port ?? ''
      const key = `${host}:${port}`
      if (!keyMap.has(key)) keyMap.set(key, [])
      keyMap.get(key).push(server)
    }

    const queue = []
    for (const [key, servers] of keyMap.entries()) {
      if (!key) continue
      const lastAt = lastTestAtMap[key] || 0
      if (now - lastAt < pollingIntervalMs) continue
      if (inFlightKeyMap[key]) continue
      const representative = servers[0]
      queue.push({ key, servers, representative })
    }

    const workers = Array.from({ length: maxConcurrentTests }, async () => {
      while (queue.length > 0) {
        const item = queue.shift()
        if (!item) return
        inFlightKeyMap[item.key] = true
        try {
          const result = await testServerSilent(item.representative.id, item.representative.type)
          const ok = !!result || result === undefined
          const status = ok ? 'success' : 'error'
          item.servers.forEach((server) => {
            connectionStatusMap[server.id] = status
          })
        } catch (error) {
          item.servers.forEach((server) => {
            connectionStatusMap[server.id] = 'error'
          })
        } finally {
          lastTestAtMap[item.key] = Date.now()
          delete inFlightKeyMap[item.key]
        }
      }
    })

    await Promise.all(workers)
  } finally {
    pollingInFlight.value = false
  }
}

const setSingleSelection = (server) => {
  if (selectedIds.value.size === 1 && selectedIds.value.has(server.id)) {
    return
  }
  selectedIds.value.clear()
  selectedIds.value.add(server.id)
}

// 卡片点击处理
const handleCardClick = (server) => {
  if (batchMode.value) {
    toggleSelect(server)
  } else {
    setSingleSelection(server)
  }
}

// 监听列表刷新，清理无效的连接状态缓存
watch(serverList, (newList) => {
  const validIds = new Set(newList.map(s => s.id))
  for (const id in connectionStatusMap) {
    if (!validIds.has(Number(id))) {
      delete connectionStatusMap[id]
    }
  }
  for (const server of newList) {
    if (!server.enabled || server.type === 'local') {
      delete connectionStatusMap[server.id]
    }
  }
})

onMounted(() => {
  loadDataServerTypes()
  loadServers()
  pollingTimer.value = setInterval(() => {
    refreshConnectionStatus()
  }, pollingIntervalMs)
})

onUnmounted(() => {
  if (pollingTimer.value) {
    clearInterval(pollingTimer.value)
    pollingTimer.value = null
  }
})
</script>

<style scoped lang="scss">
// 批量操作工具栏
.batch-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  padding: 12px 16px;
  background: var(--el-color-warning-light-9);
  border: 1px solid var(--el-color-warning-light-7);
  border-radius: 4px;

  .selected-count {
    font-size: 14px;
    color: var(--el-text-color-regular);
    font-weight: 500;
  }
}

// 响应式卡片网格
.server-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(1, 1fr);
  min-height: 200px;

  @media (min-width: 768px) {
    grid-template-columns: repeat(2, 1fr);
  }

  @media (min-width: 1024px) {
    grid-template-columns: repeat(3, 1fr);
  }

  @media (min-width: 1440px) {
    grid-template-columns: repeat(4, 1fr);
  }
}

// 服务器卡片
.server-card {
  transition: all 0.2s ease;
  cursor: pointer;
  border: 2px solid transparent;

  &:hover {
    transform: translateY(-2px);
    border-color: var(--el-color-primary-light-5);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }

  &:focus {
    outline: 2px solid var(--el-color-primary);
    outline-offset: 2px;
  }

  &.is-selected {
    border-color: var(--el-color-primary);
    background-color: var(--el-color-primary-light-9);
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--el-border-color-lighter);

    .checkbox-wrapper {
      opacity: 0;
      width: 0;
      overflow: hidden;
      transition: all 0.2s ease;

      // 非批量模式下从 Tab 顺序中移除
      :deep(.el-checkbox) {
        pointer-events: none;
      }
    }

    .server-icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 22px;
      height: 22px;
      color: var(--el-color-primary);
      flex-shrink: 0;
    }

    .server-icon-image {
      width: 20px;
      height: 20px;
      border-radius: 4px;
      object-fit: contain;
    }

    .server-name {
      font-weight: 500;
      font-size: 15px;
      color: var(--el-text-color-primary);
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .status-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      flex-shrink: 0;

      &.status-unknown {
        background-color: var(--el-color-info);
      }

      &.status-success,
      &.status-green {
        background-color: var(--el-color-success);
      }

      &.status-error,
      &.status-red {
        background-color: var(--el-color-danger);
      }

      &.status-disabled {
        background-color: var(--el-color-info-light-5);
      }
    }

  }

  .card-body {
    padding-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;

    .card-divider {
      height: 1px;
      background: var(--el-border-color-lighter);
      margin-top: 4px;
    }

    .info-row {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;

      .label {
        color: var(--el-text-color-secondary);
        min-width: 40px;
      }

      .value {
        color: var(--el-text-color-primary);
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .card-actions {
      min-height: 36px;
      margin-top: 6px;
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 8px;
      align-items: center;
      justify-items: center;
    }

    .action-button {
      font-size: 14px;
      line-height: 1;
      padding: 6px 0;
      min-width: 64px;
      justify-content: center;
    }
  }

  // Checkbox 渐显动画
  &:hover .checkbox-wrapper,
  &.is-batch-mode .checkbox-wrapper,
  &.is-selected .checkbox-wrapper {
    opacity: 1;
    width: 24px;

    :deep(.el-checkbox) {
      pointer-events: auto;
    }
  }

  &:focus .checkbox-wrapper {
    opacity: 1;
    width: 24px;

    :deep(.el-checkbox) {
      pointer-events: auto;
    }
  }

  // 批量模式下始终启用 checkbox
  &.is-batch-mode .checkbox-wrapper :deep(.el-checkbox) {
    pointer-events: auto;
  }
}

</style>
