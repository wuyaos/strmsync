import { reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import dayjs from "dayjs"
import relativeTime from "dayjs/plugin/relativeTime"
import "dayjs/locale/zh-cn"
import { deleteJob, disableJob, enableJob, getJobList, triggerJob } from "@/api/jobs"
import { normalizeListResponse } from "@/api/normalize"
import { confirmDialog } from "@/composables/useConfirmDialog"
import { useJobBatch } from "@/composables/useJobBatch"
import { parseJobOptions } from "@/composables/useJobOptions"

dayjs.extend(relativeTime)
dayjs.locale("zh-cn")

export const useJobList = ({ isActive }) => {
  const activeRef = isActive || ref(true)
  const loading = ref(false)
  const jobList = ref([])

  const actionLoading = reactive({
    trigger: {},
    toggle: {}
  })

  const filters = reactive({
    status: "",
    dataServerId: "",
    strmMode: "",
    keyword: ""
  })

  const pagination = reactive({
    page: 1,
    pageSize: 10,
    total: 0
  })

  const formatTime = (time) => {
    return dayjs(time).fromNow()
  }

  const getMediaDir = (row) => {
    if (!row) return "-"
    return row.source_path || row.media_dir || "-"
  }

  const getSyncStrategy = (row) => {
    const options = parseJobOptions(row?.options)
    const syncOpts = options.sync_opts || row?.sync_opts || {}
    if (syncOpts.full_resync) return "全量同步"
    return "更新同步"
  }

  const getStrmMode = (row) => {
    const options = parseJobOptions(row?.options)
    const mode = options.strm_mode || row?.strm_mode || "local"
    return mode === "url" ? "远程 URL" : "本地路径"
  }

  const isRowActionPending = (rowId) => {
    return actionLoading.trigger[rowId] || actionLoading.toggle[rowId]
  }

  const loadJobs = async () => {
    if (!activeRef.value) return
    loading.value = true
    try {
      const params = {
        enabled: filters.status === "enabled" ? "true"
          : filters.status === "disabled" ? "false" : undefined,
        name: filters.keyword,
        data_server_id: filters.dataServerId || undefined,
        strm_mode: filters.strmMode || undefined,
        page: pagination.page,
        pageSize: pagination.pageSize
      }
      const response = await getJobList(params)
      const { list, total } = normalizeListResponse(response)
      if (!activeRef.value) return
      jobList.value = list
      pagination.total = total
    } catch (error) {
      console.error("加载任务列表失败:", error)
    } finally {
      if (activeRef.value) {
        loading.value = false
      }
    }
  }

  const handleSearch = () => {
    pagination.page = 1
    loadJobs()
  }

  const handleDelete = async (row) => {
    try {
      const confirmed = await confirmDialog({
        title: "删除任务",
        message: "该操作不可恢复，将删除以下任务：",
        type: "error",
        items: [row.name || `ID:${row.id}`],
        confirmText: "确认删除",
        cancelText: "取消"
      })
      if (!confirmed) return
      await deleteJob(row.id)
      ElMessage.success("任务已删除")
      loadJobs()
    } catch (error) {
      if (error !== "cancel") {
        console.error("删除任务失败:", error)
      }
    }
  }

  const handleToggle = async (row) => {
    if (actionLoading.toggle[row.id]) return

    try {
      actionLoading.toggle[row.id] = true

      if (row.enabled) {
        await disableJob(row.id)
        ElMessage.success("任务已禁用")
      } else {
        await enableJob(row.id)
        ElMessage.success("任务已启用")
      }

      await loadJobs()
    } catch (error) {
      console.error("切换任务状态失败:", error)
    } finally {
      delete actionLoading.toggle[row.id]
    }
  }

  const handleTrigger = async (row) => {
    if (actionLoading.trigger[row.id]) return

    try {
      actionLoading.trigger[row.id] = true
      await triggerJob(row.id)
      ElMessage.success("任务已触发执行")
      await loadJobs()
    } catch (error) {
      console.error("触发任务失败:", error)
    } finally {
      delete actionLoading.trigger[row.id]
    }
  }

  const getServerName = (row, type) => {
    const resolveName = (name, id) => {
      if (typeof name === "string") {
        const trimmed = name.trim()
        if (trimmed) return trimmed
      } else if (name) {
        return name
      }
      return id ?? "-"
    }
    if (type === "data") {
      const name = row.data_server_name ?? row.data_server?.name
      return resolveName(name, row.data_server_id)
    }
    const name = row.media_server_name ?? row.media_server?.name
    return resolveName(name, row.media_server_id)
  }

  const batchActions = useJobBatch({ jobList, loadJobs })

  return {
    loading,
    jobList,
    actionLoading,
    filters,
    pagination,
    loadJobs,
    handleSearch,
    handleDelete,
    handleToggle,
    handleTrigger,
    getServerName,
    getSyncStrategy,
    getStrmMode,
    getMediaDir,
    isRowActionPending,
    formatTime,
    ...batchActions
  }
}
