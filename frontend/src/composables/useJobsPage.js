import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'
import {
  createJob,
  deleteJob,
  disableJob,
  enableJob,
  getJob,
  getJobList,
  triggerJob,
  updateJob
} from '@/api/jobs'
import { getServer, getServerList, listDirectories } from '@/api/servers'
import { normalizeListResponse } from '@/api/normalize'
import { usePathDialog, normalizePath, joinPath } from '@/composables/usePathDialog'
import { confirmDialog } from '@/composables/useConfirmDialog'
import { DEFAULT_MEDIA_EXTS, DEFAULT_META_EXTS } from '@/constants/defaults'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

export const useJobsPage = () => {
  const isActive = ref(true)
  const loading = ref(false)
  const saving = ref(false)
  const jobList = ref([])
  const dataServers = ref([])
  const mediaServers = ref([])

  const actionLoading = reactive({
    trigger: {},
    toggle: {}
  })

  const filters = reactive({
    status: '',
    dataServerId: '',
    strmMode: '',
    keyword: ''
  })

  const pagination = reactive({
    page: 1,
    pageSize: 10,
    total: 0
  })

  const dialogVisible = ref(false)
  const isEdit = ref(false)
  const extsLoading = ref(false)

  const createDefaultSyncOpts = () => ({
    full_resync: false,
    full_update: true,
    update_strm: true,
    update_meta: true,
    skip_strm: false,
    overwrite_meta: false,
    skip_meta: false
  })

  const defaultCleanupOptions = ['clean_local', 'clean_folders', 'clean_symlinks', 'clean_meta']

  const createDefaultFormData = () => ({
    id: null,
    name: '',
    data_server_id: null,
    media_server_id: null,
    media_dir: '',
    remote_root: '',
    local_dir: '',
    exclude_dirs: [],
    schedule_enabled: false,
    cron: '',
    sync_opts: createDefaultSyncOpts(),
    metadata_mode: 'copy',
    thread_count: 4,
    cleanup_opts: defaultCleanupOptions.slice(),
    strm_mode: 'local',
    prefer_remote_list: false,
    min_file_size: 10,
    strm_replace_rules: [],
    media_exts: DEFAULT_MEDIA_EXTS.slice(),
    meta_exts: DEFAULT_META_EXTS.slice(),
    enabled: true
  })

  const formData = reactive(createDefaultFormData())

  const dialogTitle = computed(() => (isEdit.value ? '编辑任务' : '新增任务'))

  const validateCron = (rule, value, callback) => {
    if (!formData.schedule_enabled) {
      callback()
      return
    }
    if (!value || !String(value).trim()) {
      callback(new Error('请输入 Cron 表达式'))
      return
    }
    callback()
  }

  const validateRemoteRoot = (rule, value, callback) => {
    if (!currentServerHasApi.value) {
      callback()
      return
    }
    if (!value || !String(value).trim()) {
      callback(new Error('请输入远程根目录'))
      return
    }
    callback()
  }

  const formRules = {
    name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
    data_server_id: [{ required: true, message: '请选择数据服务器', trigger: 'change' }],
    media_dir: [{ required: true, message: '请输入媒体目录', trigger: 'blur' }],
    remote_root: [{ validator: validateRemoteRoot, trigger: 'blur' }],
    local_dir: [{ required: true, message: '请输入本地输出', trigger: 'blur' }],
    cron: [{ validator: validateCron, trigger: 'blur' }]
  }

  const resolveServerCapabilities = (type) => {
    if (!type) {
      return { hasApi: false, supportsUrl: false }
    }
    if (type === 'local') {
      return { hasApi: false, supportsUrl: false }
    }
    if (type === 'openlist') {
      return { hasApi: true, supportsUrl: true }
    }
    return { hasApi: true, supportsUrl: false }
  }

  const parseOptions = (options) => {
    if (!options) return {}
    if (typeof options === 'object') return options
    if (typeof options === 'string') {
      try {
        return JSON.parse(options)
      } catch (error) {
        return {}
      }
    }
    return {}
  }

  const parseServerOptions = (server) => {
    if (!server) return {}
    const options = parseOptions(server.options)
    return typeof options === 'object' && options ? options : {}
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
        accessPath: options.access_path || '',
        mountPath: options.mount_path || '',
        remoteRoot: options.remote_root || ''
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

  const mediaDirDisabled = computed(() => !currentServer.value)

  const currentServerSupportsUrl = computed(() => {
    return currentServer.value?.supportsUrl || false
  })

  const currentServerRemoteOnly = computed(() => {
    const server = currentServer.value
    if (!server || server.type !== 'openlist') return false
    return !String(server.accessPath || '').trim()
  })

  const currentServerIsLocal = computed(() => {
    return currentServer.value?.type === 'local'
  })

  const showMediaDirWarning = computed(() => currentServer.value?.type === 'openlist')

  const toRelativePath = (path, root) => {
    const normalizedPath = normalizePath(path || '/')
    const normalizedRoot = normalizePath(root || '/')
    if (!normalizedRoot || normalizedRoot === '/') {
      return normalizedPath.replace(/^\/+/, '')
    }
    if (normalizedPath === normalizedRoot) {
      return '.'
    }
    if (normalizedPath.startsWith(`${normalizedRoot}/`)) {
      return normalizedPath.slice(normalizedRoot.length + 1)
    }
    return normalizedPath.replace(/^\/+/, '')
  }

  const normalizeExcludeInput = (value) => {
    const trimmed = String(value || '').trim()
    if (!trimmed) return ''
    if (trimmed === '.') return '.'
    if (trimmed.startsWith('./')) return trimmed.slice(2)
    if (trimmed.startsWith('/')) {
      return toRelativePath(trimmed, formData.media_dir || '/')
    }
    return trimmed
  }

  const excludeDirsText = computed({
    get: () => formData.exclude_dirs.join(', '),
    set: (value) => {
      if (!value) {
        formData.exclude_dirs = []
        return
      }
      formData.exclude_dirs = value
        .split(',')
        .map(item => normalizeExcludeInput(item))
        .filter(Boolean)
    }
  })

  const formatTime = (time) => {
    return dayjs(time).fromNow()
  }

  const getMediaDir = (row) => {
    if (!row) return '-'
    return row.source_path || row.media_dir || '-'
  }

  const getSyncStrategy = (row) => {
    const options = parseOptions(row?.options)
    const syncOpts = options.sync_opts || row?.sync_opts || {}
    if (syncOpts.full_resync) return '全量同步'
    return '更新同步'
  }

  const getStrmMode = (row) => {
    const options = parseOptions(row?.options)
    const mode = options.strm_mode || row?.strm_mode || 'local'
    return mode === 'url' ? '远程 URL' : '本地路径'
  }

  const isRowActionPending = (rowId) => {
    return actionLoading.trigger[rowId] || actionLoading.toggle[rowId]
  }

  const applyOptionsToForm = (options) => {
    if (Array.isArray(options.exclude_dirs)) {
      formData.exclude_dirs = options.exclude_dirs
    }
    if (options.sync_opts && typeof options.sync_opts === 'object') {
      formData.sync_opts = { ...createDefaultSyncOpts(), ...options.sync_opts }
    }
    if (options.metadata_mode) {
      formData.metadata_mode = options.metadata_mode
    }
    if (options.thread_count !== undefined) {
      formData.thread_count = options.thread_count
    }
    if (Array.isArray(options.cleanup_opts)) {
      formData.cleanup_opts = options.cleanup_opts
    }
    if (options.strm_mode) {
      formData.strm_mode = options.strm_mode
    }
    if (options.prefer_remote_list !== undefined) {
      formData.prefer_remote_list = Boolean(options.prefer_remote_list)
    }
    if (options.min_file_size !== undefined) {
      formData.min_file_size = options.min_file_size
    }
    if (Array.isArray(options.strm_replace_rules)) {
      formData.strm_replace_rules = options.strm_replace_rules
        .map(rule => ({
          from: typeof rule?.from === 'string' ? rule.from : '',
          to: typeof rule?.to === 'string' ? rule.to : ''
        }))
        .filter(rule => rule.from || rule.to)
    }
    if (Array.isArray(options.media_exts)) {
      formData.media_exts = options.media_exts
    }
    if (Array.isArray(options.meta_exts)) {
      formData.meta_exts = options.meta_exts
    }
  }

  const normalizeMetadataMode = (mode) => {
    if (mode === 'copy' || mode === 'download' || mode === 'none') return mode
    if (mode === 'api') return 'download'
    return 'copy'
  }

  const normalizeSyncOptions = () => {
    const syncOpts = formData.sync_opts || {}
    if (syncOpts.full_resync) {
      syncOpts.full_update = false
    } else {
      syncOpts.full_update = true
    }

    if (syncOpts.overwrite_meta) {
      syncOpts.update_meta = false
      syncOpts.skip_meta = false
    } else if (syncOpts.skip_meta) {
      syncOpts.update_meta = false
      syncOpts.overwrite_meta = false
    } else {
      syncOpts.update_meta = true
      syncOpts.overwrite_meta = false
      syncOpts.skip_meta = false
    }

    formData.metadata_mode = normalizeMetadataMode(formData.metadata_mode)
  }

  const resolveWatchMode = () => {
    if (!currentServer.value) return 'local'
    return currentServer.value.type === 'local' ? 'local' : 'api'
  }

  const resolveStrmPath = () => {
    return String(formData.media_dir || '').trim()
  }

  const toNumberOrNull = (value) => {
    if (value === null || value === undefined || value === '') return null
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
      cron: formData.schedule_enabled ? formData.cron : ''
    }
  }

  const loadServers = async () => {
    try {
      const [dataResp, mediaResp] = await Promise.all([
        getServerList({ type: 'data', page: 1, pageSize: 200 }),
        getServerList({ type: 'media', page: 1, pageSize: 200 })
      ])
      if (!isActive.value) return
      dataServers.value = normalizeListResponse(dataResp).list
      mediaServers.value = normalizeListResponse(mediaResp).list
    } catch (error) {
      console.error('加载服务器列表失败:', error)
    }
  }

  const loadJobs = async () => {
    if (!isActive.value) return
    loading.value = true
    try {
      const params = {
        enabled: filters.status === 'enabled' ? 'true'
          : filters.status === 'disabled' ? 'false' : undefined,
        name: filters.keyword,
        data_server_id: filters.dataServerId || undefined,
        strm_mode: filters.strmMode || undefined,
        page: pagination.page,
        pageSize: pagination.pageSize
      }
      const response = await getJobList(params)
      const { list, total } = normalizeListResponse(response)
      if (!isActive.value) return
      jobList.value = list
      pagination.total = total
    } catch (error) {
      console.error('加载任务列表失败:', error)
    } finally {
      if (isActive.value) {
        loading.value = false
      }
    }
  }

  const handleSearch = () => {
    pagination.page = 1
    loadJobs()
  }

  const resetForm = () => {
    const defaults = createDefaultFormData()
    Object.assign(formData, defaults)
    normalizeSyncOptions()
    autoMediaDir.value = ''
  }

  const handleAdd = () => {
    isEdit.value = false
    resetForm()
    dialogVisible.value = true
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

  const applyJobDetail = (job) => {
    if (!job) return
    formData.id = job.id || null
    formData.name = job.name || ''
    formData.data_server_id = job.data_server_id ?? job.data_server?.id ?? null
    formData.media_server_id = job.media_server_id ?? job.media_server?.id ?? null
    formData.media_dir = job.source_path || ''
    formData.remote_root = job.remote_root || ''
    formData.local_dir = job.target_path || ''
    formData.cron = job.cron || ''
    formData.schedule_enabled = Boolean(job.cron)
    formData.enabled = job.enabled !== false

    const options = parseOptions(job.options)
    applyOptionsToForm(options)
    normalizeSyncOptions()
    handleServerChange()
  }

  const handleEdit = async (row) => {
    isEdit.value = true
    resetForm()
    const job = await loadJobDetail(row)
    applyJobDetail(job)
    dialogVisible.value = true
  }

  const handleServerChange = async () => {
    if (currentServer.value) {
      try {
        const response = await getServer(currentServer.value.id, currentServer.value.type)
        if (!isActive.value) return
        const server = response?.server || response?.data || response
        if (server?.id) {
          const index = dataServers.value.findIndex(item => Number(item.id) === Number(server.id))
          if (index >= 0) {
            dataServers.value.splice(index, 1, server)
          }
        }
      } catch (error) {
        console.error('读取服务器配置失败:', error)
      }
    }
    if (!currentServerSupportsUrl.value) {
      formData.strm_mode = 'local'
    }
    if (currentServerRemoteOnly.value) {
      formData.strm_mode = 'url'
      formData.metadata_mode = 'download'
    }
    if (currentServerIsLocal.value && formData.metadata_mode === 'download') {
      formData.metadata_mode = 'copy'
      formData.prefer_remote_list = true
    }
    if (currentServerHasApi.value && !String(formData.remote_root || '').trim()) {
      formData.remote_root = currentServer.value?.remoteRoot || '/'
    }

    applyDefaultMediaDir()
  }

  const pathDialogField = ref('')
  const autoMediaDir = ref('')

  const isLocalLikePath = (value) => {
    const raw = String(value || '').trim()
    if (!raw) return false
    if (/^[a-zA-Z]:[\\/]/.test(raw)) return true
    return raw.startsWith('/mnt/')
  }

  const resolveAccessRoot = () => {
    const server = currentServer.value
    if (!server) return '/'
    const accessPath = normalizePath(server.accessPath || '')
    if (server.type === 'openlist' && !String(server.accessPath || '').trim()) {
      return '/'
    }
    return accessPath
  }

  const applyDefaultMediaDir = () => {
    if (isEdit.value) return
    if (formData.media_dir && formData.media_dir !== autoMediaDir.value) return
    const root = resolveAccessRoot()
    formData.media_dir = root
    autoMediaDir.value = root
  }

  const buildDirectoryParams = (input) => {
    const path = typeof input === 'string' ? input : input?.path
    const limit = typeof input === 'object' ? input?.limit : undefined
    const offset = typeof input === 'object' ? input?.offset : undefined
    const forceApi = typeof input === 'object' ? input?.forceApi : false
    const server = currentServer.value
    if (!server) {
      return { path, mode: 'local', limit, offset }
    }
    if (!forceApi && (server.type === 'local' || isLocalLikePath(path) || String(server.accessPath || '').trim())) {
      return { path, mode: 'local', limit, offset }
    }
    return {
      path,
      limit,
      offset,
      mode: 'api',
      type: server.type,
      host: server.host,
      port: server.port,
      apiKey: server.api_key || server.apiKey
    }
  }

  const pathDlg = usePathDialog({
    loader: (payload) => listDirectories(buildDirectoryParams(payload)),
    onError: () => ElMessage.error('加载目录失败')
  })

  const toAbsolutePath = (value, root) => {
    if (!value || value === '.') return normalizePath(root || '/')
    if (String(value).startsWith('/')) return normalizePath(value)
    return normalizePath(joinPath(root || '/', value))
  }

  const resolveDialogRoot = (field) => {
    if (field === 'media_dir') {
      return resolveAccessRoot()
    }
    if (field === 'remote_root') {
      return '/'
    }
    if (field === 'exclude_dirs' && formData.media_dir) {
      return normalizePath(formData.media_dir)
    }
    if (field === 'exclude_dirs') {
      return resolveAccessRoot()
    }
    return '/'
  }

  const openPathDialog = async (field, options = {}) => {
    if (field === 'media_dir' && !currentServer.value) {
      ElMessage.warning('请先选择数据服务器')
      return
    }
    pathDialogField.value = field
    const dialogRoot = resolveDialogRoot(field)
    const initialPath = field === 'exclude_dirs' || field === 'media_dir'
      ? dialogRoot
      : (typeof formData[field] === 'string' ? formData[field] : '/')
    await pathDlg.open({
      mode: options.multiple ? 'multi' : 'single',
      root: dialogRoot,
      path: initialPath || dialogRoot,
      extra: { forceApi: options.forceApi }
    })

    if (options.multiple) {
      pathDlg.selectedNames.value = (formData.exclude_dirs || [])
        .map(item => toAbsolutePath(item, dialogRoot))
        .filter(Boolean)
    } else if (typeof formData[field] === 'string' && formData[field]) {
      const normalized = normalizePath(formData[field])
      if (normalized !== '/' && normalized !== dialogRoot) {
        pathDlg.selectedName.value = normalized
      }
    }
  }

  const handlePathSelect = (name) => {
    pathDlg.selectRow(name)
  }

  const handlePathToggle = (name) => {
    pathDlg.toggleRow(name)
  }

  const handlePathConfirm = () => {
    if (pathDlg.mode.value === 'multi') {
      const root = pathDlg.root.value || formData.media_dir || '/'
      const selected = pathDlg.getSelectedMulti()
      formData.exclude_dirs = selected
        .map(item => toRelativePath(item, root))
        .filter(Boolean)
      pathDlg.close()
      return
    }

    if (!pathDialogField.value) return
    const selectedPath = pathDlg.getSelectedSingle()
    formData[pathDialogField.value] = selectedPath
    pathDlg.close()
  }

  const handleSave = async () => {
    try {
      saving.value = true
      const payload = buildJobPayload()
      if (isEdit.value) {
        await updateJob(formData.id, payload)
        ElMessage.success('任务已更新')
      } else {
        await createJob(payload)
        ElMessage.success('任务已创建')
      }
      if (!isActive.value) return
      dialogVisible.value = false
      loadJobs()
    } catch (error) {
      if (error?.message) {
        console.error('保存任务失败:', error)
      }
    } finally {
      if (isActive.value) {
        saving.value = false
      }
    }
  }

  const handleDelete = async (row) => {
    try {
      const confirmed = await confirmDialog({
        title: '删除任务',
        message: '该操作不可恢复，将删除以下任务：',
        type: 'error',
        items: [row.name || `ID:${row.id}`],
        confirmText: '确认删除',
        cancelText: '取消'
      })
      if (!confirmed) return
      await deleteJob(row.id)
      ElMessage.success('任务已删除')
      loadJobs()
    } catch (error) {
      if (error !== 'cancel') {
        console.error('删除任务失败:', error)
      }
    }
  }

  const resolveJobNames = (ids) => {
    const map = new Map(jobList.value.map((job) => [job.id, job.name || `ID:${job.id}`]))
    return ids.map((id) => map.get(id) || `ID:${id}`)
  }

  const handleBatchEnable = async (ids) => {
    if (!Array.isArray(ids) || ids.length === 0) return
    const confirmed = await confirmDialog({
      title: '批量启用',
      message: `将启用选中的 ${ids.length} 个任务，是否继续？`,
      type: 'info',
      items: resolveJobNames(ids),
      confirmText: '确认启用',
      cancelText: '取消'
    })
    if (!confirmed) return
    const results = await Promise.allSettled(ids.map((id) => enableJob(id)))
    const failCount = results.filter((r) => r.status === 'rejected').length
    if (failCount === 0) {
      ElMessage.success(`成功启用 ${ids.length} 个任务`)
    } else {
      ElMessage.warning(`启用完成：成功 ${ids.length - failCount} 个，失败 ${failCount} 个`)
    }
    loadJobs()
  }

  const handleBatchDisable = async (ids) => {
    if (!Array.isArray(ids) || ids.length === 0) return
    const confirmed = await confirmDialog({
      title: '批量禁用',
      message: `将禁用选中的 ${ids.length} 个任务，是否继续？`,
      type: 'warning',
      items: resolveJobNames(ids),
      confirmText: '确认禁用',
      cancelText: '取消'
    })
    if (!confirmed) return
    const results = await Promise.allSettled(ids.map((id) => disableJob(id)))
    const failCount = results.filter((r) => r.status === 'rejected').length
    if (failCount === 0) {
      ElMessage.success(`成功禁用 ${ids.length} 个任务`)
    } else {
      ElMessage.warning(`禁用完成：成功 ${ids.length - failCount} 个，失败 ${failCount} 个`)
    }
    loadJobs()
  }

  const handleBatchRun = async (ids) => {
    if (!Array.isArray(ids) || ids.length === 0) return
    const confirmed = await confirmDialog({
      title: '批量运行',
      message: `将运行选中的 ${ids.length} 个任务，是否继续？`,
      type: 'info',
      items: resolveJobNames(ids),
      confirmText: '确认运行',
      cancelText: '取消'
    })
    if (!confirmed) return
    const results = await Promise.allSettled(ids.map((id) => triggerJob(id)))
    const failCount = results.filter((r) => r.status === 'rejected').length
    if (failCount === 0) {
      ElMessage.success(`成功运行 ${ids.length} 个任务`)
    } else {
      ElMessage.warning(`运行完成：成功 ${ids.length - failCount} 个，失败 ${failCount} 个`)
    }
    loadJobs()
  }

  const handleBatchDelete = async (ids) => {
    if (!Array.isArray(ids) || ids.length === 0) return
    const confirmed = await confirmDialog({
      title: '批量删除',
      message: `将删除选中的 ${ids.length} 个任务，且无法恢复，是否继续？`,
      type: 'error',
      items: resolveJobNames(ids),
      confirmText: '确认删除',
      cancelText: '取消'
    })
    if (!confirmed) return
    const results = await Promise.allSettled(ids.map((id) => deleteJob(id)))
    const failCount = results.filter((r) => r.status === 'rejected').length
    if (failCount === 0) {
      ElMessage.success(`成功删除 ${ids.length} 个任务`)
    } else {
      ElMessage.warning(`删除完成：成功 ${ids.length - failCount} 个，失败 ${failCount} 个`)
    }
    loadJobs()
  }

  const handleToggle = async (row) => {
    if (actionLoading.toggle[row.id]) return

    try {
      actionLoading.toggle[row.id] = true

      if (row.enabled) {
        await disableJob(row.id)
        ElMessage.success('任务已禁用')
      } else {
        await enableJob(row.id)
        ElMessage.success('任务已启用')
      }

      await loadJobs()
    } catch (error) {
      console.error('切换任务状态失败:', error)
    } finally {
      delete actionLoading.toggle[row.id]
    }
  }

  const handleTrigger = async (row) => {
    if (actionLoading.trigger[row.id]) return

    try {
      actionLoading.trigger[row.id] = true
      await triggerJob(row.id)
      ElMessage.success('任务已触发执行')
      await loadJobs()
    } catch (error) {
      console.error('触发任务失败:', error)
    } finally {
      delete actionLoading.trigger[row.id]
    }
  }

  const getServerName = (row, type) => {
    const resolveName = (name, id) => {
      if (typeof name === 'string') {
        const trimmed = name.trim()
        if (trimmed) return trimmed
      } else if (name) {
        return name
      }
      return id ?? '-'
    }
    if (type === 'data') {
      const name = row.data_server_name ?? row.data_server?.name
      return resolveName(name, row.data_server_id)
    }
    const name = row.media_server_name ?? row.media_server?.name
    return resolveName(name, row.media_server_id)
  }

  onMounted(() => {
    loadServers()
    loadJobs()
  })

  onUnmounted(() => {
    isActive.value = false
  })

  return {
    loading,
    saving,
    jobList,
    actionLoading,
    filters,
    pagination,
    dialogVisible,
    dialogTitle,
    formData,
    formRules,
    extsLoading,
    dataServerOptions,
    mediaServerOptions,
    currentServer,
    currentServerHasApi,
    currentServerSupportsUrl,
    currentServerIsLocal,
    currentServerRemoteOnly,
    showMediaDirWarning,
    mediaDirDisabled,
    excludeDirsText,
    pathDlg,
    loadJobs,
    handleSearch,
    handleAdd,
    handleEdit,
    handleSave,
    handleDelete,
    handleToggle,
    handleTrigger,
    handleBatchEnable,
    handleBatchDisable,
    handleBatchRun,
    handleBatchDelete,
    handleServerChange,
    getServerName,
    getSyncStrategy,
    getStrmMode,
    isRowActionPending,
    getMediaDir,
    openPathDialog,
    handlePathSelect,
    handlePathToggle,
    handlePathConfirm,
    parseOptions,
    formatTime
  }
}
