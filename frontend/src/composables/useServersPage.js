import { computed, onMounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import dayjs from "dayjs"
import relativeTime from "dayjs/plugin/relativeTime"
import "dayjs/locale/zh-cn"
import { deleteServer, getServerList, getServerTypes, testServer, updateServer } from "@/api/servers"
import { normalizeListResponse } from "@/api/normalize"
import { useServerBatch } from "@/composables/useServerBatch"
import { useServerConnectivity } from "@/composables/useServerConnectivity"
import { MEDIA_SERVER_TYPE_OPTIONS } from "@/constants/mediaServerTypes"
import { getServerIcon, getServerIconUrl } from "@/constants/serverIcons"
import { confirmDialog } from "@/composables/useConfirmDialog"

dayjs.extend(relativeTime)
dayjs.locale("zh-cn")

export const useServersPage = (options = {}) => {
  const enableConnectivity = options?.enableConnectivity === true
  const activeConnectivity = options?.isActive
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

  const activeTab = ref("data")
  const loading = ref(false)
  const serverList = ref([])

  const filters = reactive({
    keyword: "",
    serverType: ""
  })

  const pagination = reactive({
    page: 1,
    pageSize: 12,
    total: 0
  })

  const { getConnectionStatus, refreshConnectionStatus, setConnectionStatus } = useServerConnectivity({
    serverList,
    intervalMs: 10000,
    maxConcurrentTests: 3,
    autoStart: enableConnectivity,
    isActive: activeConnectivity
  })

  const dialogVisible = ref(false)
  const editingServer = ref(null)

  const dataServerTypeDefs = ref([])

  const mediaServerTypeOptions = MEDIA_SERVER_TYPE_OPTIONS
  const serverTypeOptions = computed(() => {
    if (activeTab.value === "media") {
      return mediaServerTypeOptions.map((item) => ({
        label: item.label,
        value: item.value
      }))
    }
    return dataServerTypeDefs.value.map((item) => ({
      label: item.label || item.type,
      value: item.type
    }))
  })

  const formatTime = (time) => dayjs(time).fromNow()

  const loadServers = async () => {
    loading.value = true
    try {
      const params = {
        category: activeTab.value,
        serverType: filters.serverType || undefined,
        keyword: filters.keyword,
        page: pagination.page,
        pageSize: pagination.pageSize
      }
      const response = await getServerList(params)
      const { list, total } = normalizeListResponse(response)
      serverList.value = list
      pagination.total = total
      if (enableConnectivity) {
        await refreshConnectionStatus()
      }
    } catch (error) {
      console.error("加载服务器列表失败:", error)
    } finally {
      loading.value = false
    }
  }

  const handleSearch = () => {
    pagination.page = 1
    clearSelection()
    loadServers()
  }

  const handleTabChange = () => {
    pagination.page = 1
    filters.keyword = ""
    filters.serverType = ""
    clearSelection()
    if (activeTab.value === "data") {
      loadDataServerTypes()
    }
    loadServers()
  }

  const loadDataServerTypes = async (force = false) => {
    if (!force && dataServerTypeDefs.value.length > 0) return
    try {
      const response = await getServerTypes({ category: "data" })
      dataServerTypeDefs.value = response?.types || []
    } catch (error) {
      console.error("加载服务器类型失败:", error)
      ElMessage.error("加载服务器类型定义失败")
    }
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

  const normalizeOptions = (raw) => {
    if (!raw) return {}
    if (typeof raw === "object") return raw
    if (typeof raw === "string") {
      try {
        const parsed = JSON.parse(raw)
        return parsed && typeof parsed === "object" ? parsed : {}
      } catch (error) {
        return {}
      }
    }
    return {}
  }

  const buildTogglePayload = (server, enabled) => {
    const options = normalizeOptions(server.options)
    return {
      name: server.name ?? "",
      type: server.type ?? "",
      host: server.host ?? "",
      port: server.port ?? 0,
      api_key: server.api_key ?? server.apiKey ?? "",
      options,
      enabled,
      download_rate_per_sec: server.download_rate_per_sec ?? server.downloadRatePerSec,
      api_rate: server.api_rate ?? server.apiRate,
      api_retry_max: server.api_retry_max ?? server.apiRetryMax,
      api_retry_interval_sec: server.api_retry_interval_sec ?? server.apiRetryIntervalSec
    }
  }

  const handleToggleEnabled = async (server, enabled) => {
    const prevEnabled = server.enabled
    server.enabled = enabled
    try {
      const payload = buildTogglePayload(server, enabled)
      await updateServer(server.id, payload)
      if (enableConnectivity) {
        await refreshConnectionStatus()
      }
    } catch (error) {
      server.enabled = prevEnabled
      ElMessage.error("更新服务器状态失败")
      console.error("更新服务器状态失败:", error)
    }
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
      const confirmed = await confirmDialog({
        title: "删除服务器",
        message: "该操作不可恢复，将删除以下服务器：",
        type: "error",
        items: [row.name || `ID:${row.id}`],
        confirmText: "确认删除",
        cancelText: "取消"
      })
      if (!confirmed) return
      await deleteServer(row.id, row.type)
      ElMessage.success("服务器已删除")
      loadServers()
    } catch (error) {
      if (error !== "cancel") {
        console.error("删除服务器失败:", error)
      }
    }
  }

  const handleTest = async (row) => {
    const loadingMsg = ElMessage.info("正在测试连接...")
    try {
      await testServer(row.id, row.type)
      loadingMsg.close()
      setConnectionStatus(row.id, "success")
      ElMessage.success("连接测试成功")
    } catch (error) {
      loadingMsg.close()
      setConnectionStatus(row.id, "error")
    }
  }

  const setSingleSelection = (server) => {
    if (selectedIds.value.size === 1 && selectedIds.value.has(server.id)) {
      return
    }
    selectedIds.value = new Set([server.id])
  }

  const handleCardClick = (server) => {
    setSingleSelection(server)
  }

  onMounted(() => {
    loadDataServerTypes()
    loadServers()
  })

  return {
    activeTab,
    loading,
    serverList,
    filters,
    pagination,
    dialogVisible,
    editingServer,
    dataServerTypeDefs,
    mediaServerTypeOptions,
    serverTypeOptions,
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
    handleBatchDelete,
    formatTime,
    loadServers,
    handleSearch,
    handleTabChange,
    handleAdd,
    handleEdit,
    handleFormSaved,
    handleDelete,
    handleTest,
    handleToggleEnabled,
    updateServer,
    deleteServer,
    getServerIcon,
    getServerIconUrl,
    getConnectionStatus,
    handleCardClick
  }
}
