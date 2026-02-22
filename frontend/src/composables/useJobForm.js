import { computed, ref, watchEffect } from "vue"
import { ElMessage } from "element-plus"
import { createJob, getJob, updateJob } from "@/api/jobs"
import { getServer, getServerList } from "@/api/servers"
import { normalizeListResponse } from "@/api/normalize"
import { parseJobOptions } from "@/composables/useJobOptions"
import { useJobFormState } from "@/composables/useJobFormState"

export const useJobForm = ({ isActive, onSaved } = {}) => {
  const activeRef = isActive || ref(true)
  const saving = ref(false)
  const dialogVisible = ref(false)
  const isEdit = ref(false)
  const extsLoading = ref(false)

  const dataServers = ref([])
  const mediaServers = ref([])

  const afterReset = ref(null)
  const afterServerChange = ref(null)

  const currentServerHasApiRef = ref(false)

  const {
    formData,
    formRules,
    resetForm,
    normalizeSyncOptions,
    applyOptionsToForm
  } = useJobFormState({
    currentServerHasApi: currentServerHasApiRef
  })

  const dialogTitle = computed(() => (isEdit.value ? "编辑任务" : "新增任务"))

  const resolveServerCapabilities = (type) => {
    if (!type) {
      return { hasApi: false, supportsUrl: false }
    }
    if (type === "local") {
      return { hasApi: false, supportsUrl: false }
    }
    if (type === "openlist") {
      return { hasApi: true, supportsUrl: true }
    }
    return { hasApi: true, supportsUrl: false }
  }

  const parseServerOptions = (server) => {
    if (!server) return {}
    const options = parseJobOptions(server.options)
    return typeof options === "object" && options ? options : {}
  }

  const dataServerOptions = computed(() => {
    return dataServers.value.map(server => {
      const capabilities = resolveServerCapabilities(server.type)
      const options = parseServerOptions(server)
      return {
        ...server,
        label: server.name ? `${server.name} (${server.type})` : String(server.type || server.id),
        hasApi: capabilities.hasApi,
        supportsUrl: capabilities.supportsUrl,
        accessPath: options.access_path || "",
        mountPath: options.mount_path || "",
        remoteRoot: options.remote_root || ""
      }
    })
  })

  const mediaServerOptions = computed(() => {
    return mediaServers.value.map(server => ({
      ...server,
      label: server.name ? `${server.name} (${server.type})` : String(server.type || server.id)
    }))
  })

  const currentServer = computed(() => {
    return dataServerOptions.value.find(s => Number(s.id) === Number(formData.data_server_id)) || null
  })

  const currentServerHasApi = computed(() => {
    return currentServer.value?.hasApi || false
  })

  watchEffect(() => {
    currentServerHasApiRef.value = currentServerHasApi.value
  })

  const mediaDirDisabled = computed(() => !currentServer.value)

  const currentServerSupportsUrl = computed(() => {
    return currentServer.value?.supportsUrl || false
  })

  const currentServerRemoteOnly = computed(() => {
    const server = currentServer.value
    if (!server || server.type !== "openlist") return false
    return !String(server.accessPath || "").trim()
  })

  const currentServerIsLocal = computed(() => {
    return currentServer.value?.type === "local"
  })

  const showMediaDirWarning = computed(() => currentServer.value?.type === "openlist")

  const loadServers = async () => {
    try {
      const [dataResp, mediaResp] = await Promise.all([
        getServerList({ type: "data", page: 1, pageSize: 200 }),
        getServerList({ type: "media", page: 1, pageSize: 200 })
      ])
      if (!activeRef.value) return
      dataServers.value = normalizeListResponse(dataResp).list
      mediaServers.value = normalizeListResponse(mediaResp).list
    } catch (error) {
      console.error("加载服务器列表失败:", error)
    }
  }

  const applyJobDetail = (job) => {
    if (!job) return
    formData.id = job.id || null
    formData.name = job.name || ""
    formData.data_server_id = job.data_server_id ?? job.data_server?.id ?? null
    formData.media_server_id = job.media_server_id ?? job.media_server?.id ?? null
    formData.media_dir = job.source_path || ""
    formData.remote_root = job.remote_root || ""
    formData.local_dir = job.target_path || ""
    formData.cron = job.cron || ""
    formData.schedule_enabled = Boolean(job.cron)
    formData.enabled = job.enabled !== false

    const options = parseJobOptions(job.options)
    applyOptionsToForm(options)
    normalizeSyncOptions()
  }

  const loadJobDetail = async (row) => {
    if (!row?.id) return row
    try {
      const response = await getJob(row.id)
      return response?.job || row
    } catch (error) {
      return row
    }
  }

  const handleAdd = () => {
    isEdit.value = false
    resetForm()
    normalizeSyncOptions()
    if (afterReset.value) {
      afterReset.value()
    }
    dialogVisible.value = true
  }

  const handleEdit = async (row) => {
    isEdit.value = true
    resetForm()
    if (afterReset.value) {
      afterReset.value()
    }
    const job = await loadJobDetail(row)
    applyJobDetail(job)
    await handleServerChange()
    dialogVisible.value = true
  }

  const handleServerChange = async () => {
    if (currentServer.value) {
      try {
        const response = await getServer(currentServer.value.id, currentServer.value.type)
        if (!activeRef.value) return
        const server = response?.server || response?.data || response
        if (server?.id) {
          const index = dataServers.value.findIndex(item => Number(item.id) === Number(server.id))
          if (index >= 0) {
            dataServers.value.splice(index, 1, server)
          }
        }
      } catch (error) {
        console.error("读取服务器配置失败:", error)
      }
    }
    if (!currentServerSupportsUrl.value) {
      formData.strm_mode = "local"
    }
    if (currentServerRemoteOnly.value) {
      formData.strm_mode = "url"
      formData.metadata_mode = "download"
    }
    if (currentServerIsLocal.value && formData.metadata_mode === "download") {
      formData.metadata_mode = "copy"
      formData.prefer_remote_list = true
    }
    if (currentServerHasApi.value && !String(formData.remote_root || "").trim()) {
      formData.remote_root = currentServer.value?.remoteRoot || "/"
    }

    if (afterServerChange.value) {
      afterServerChange.value()
    }
  }

  const resolveWatchMode = () => {
    if (!currentServer.value) return "local"
    return currentServer.value.type === "local" ? "local" : "api"
  }

  const resolveStrmPath = () => {
    return String(formData.media_dir || "").trim()
  }

  const toNumberOrNull = (value) => {
    if (value === null || value === undefined || value === "") return null
    const parsed = Number(value)
    if (!Number.isFinite(parsed) || parsed <= 0) return null
    return parsed
  }

  const normalizeNumber = (value, fallback = 0) => {
    const parsed = Number(value)
    if (!Number.isFinite(parsed)) return fallback
    return parsed
  }

  const buildOptionsPayload = () => ({
    exclude_dirs: formData.exclude_dirs,
    sync_opts: formData.sync_opts,
    metadata_mode: formData.metadata_mode,
    thread_count: normalizeNumber(formData.thread_count, 1),
    cleanup_opts: formData.cleanup_opts,
    strm_mode: formData.strm_mode,
    prefer_remote_list: formData.prefer_remote_list,
    min_file_size: normalizeNumber(formData.min_file_size, 0),
    strm_replace_rules: formData.strm_replace_rules.filter(rule => rule.from || rule.to),
    media_exts: formData.media_exts,
    meta_exts: formData.meta_exts
  })

  const buildJobPayload = () => {
    const watchMode = resolveWatchMode()
    return {
      name: formData.name,
      data_server_id: toNumberOrNull(formData.data_server_id),
      media_server_id: toNumberOrNull(formData.media_server_id),
      watch_mode: watchMode,
      source_path: formData.media_dir,
      remote_root: formData.remote_root,
      target_path: formData.local_dir,
      strm_path: resolveStrmPath(),
      options: JSON.stringify(buildOptionsPayload()),
      enabled: formData.enabled,
      cron: formData.schedule_enabled ? formData.cron : ""
    }
  }

  const handleSave = async () => {
    try {
      saving.value = true
      const payload = buildJobPayload()
      if (isEdit.value) {
        await updateJob(formData.id, payload)
        ElMessage.success("任务已更新")
      } else {
        await createJob(payload)
        ElMessage.success("任务已创建")
      }
      if (!activeRef.value) return
      dialogVisible.value = false
      if (onSaved) {
        onSaved()
      }
    } catch (error) {
      if (error?.message) {
        console.error("保存任务失败:", error)
      }
    } finally {
      if (activeRef.value) {
        saving.value = false
      }
    }
  }

  const setAfterReset = (callback) => {
    afterReset.value = callback
  }

  const setAfterServerChange = (callback) => {
    afterServerChange.value = callback
  }

  return {
    saving,
    dialogVisible,
    dialogTitle,
    isEdit,
    extsLoading,
    formData,
    formRules,
    dataServerOptions,
    mediaServerOptions,
    currentServer,
    currentServerHasApi,
    currentServerSupportsUrl,
    currentServerIsLocal,
    currentServerRemoteOnly,
    showMediaDirWarning,
    mediaDirDisabled,
    loadServers,
    handleAdd,
    handleEdit,
    handleSave,
    handleServerChange,
    applyJobDetail,
    loadJobDetail,
    normalizeSyncOptions,
    setAfterReset,
    setAfterServerChange
  }
}
