<template>
  <div class="jobs-page page-with-pagination">
    <FilterToolbar>
      <template #filters>
        <el-select
          v-model="filters.status"
          placeholder="状态"
          clearable
          class="w-140"
          @change="handleSearch"
        >
          <el-option label="启用" value="enabled" />
          <el-option label="禁用" value="disabled" />
        </el-select>
        <el-input
          v-model="filters.keyword"
          placeholder="搜索任务名称"
          :prefix-icon="Search"
          clearable
          class="w-260"
          @keyup.enter="handleSearch"
        />
      </template>
      <template #actions>
        <el-button type="primary" :icon="Plus" @click="handleAdd">
          新增任务
        </el-button>
      </template>
    </FilterToolbar>

    <el-table v-loading="loading" :data="jobList" stripe class="w-full">
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="数据服务器" min-width="160">
        <template #default="{ row }">
          {{ getServerName(row, 'data') }}
        </template>
      </el-table-column>
      <el-table-column label="媒体服务器" min-width="160">
        <template #default="{ row }">
          {{ getServerName(row, 'media') }}
        </template>
      </el-table-column>
      <el-table-column prop="cron" label="调度配置" min-width="160">
        <template #default="{ row }">
          <el-tag size="small" type="info">{{ row.cron || '-' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
            {{ row.enabled ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="last_run_at" label="最后运行" width="150">
        <template #default="{ row }">
          {{ row.last_run_at ? formatTime(row.last_run_at) : '-' }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-tooltip content="立即执行" placement="top">
            <el-button
              size="small"
              :icon="VideoPlay"
              :loading="actionLoading.trigger[row.id]"
              :disabled="isRowActionPending(row.id)"
              @click="handleTrigger(row)"
            />
          </el-tooltip>
          <el-tooltip content="编辑任务" placement="top">
            <el-button
              size="small"
              :icon="Edit"
              :disabled="isRowActionPending(row.id)"
              @click="handleEdit(row)"
            />
          </el-tooltip>
          <el-tooltip :content="row.enabled ? '禁用任务' : '启用任务'" placement="top">
            <el-button
              size="small"
              :icon="Switch"
              :loading="actionLoading.toggle[row.id]"
              :disabled="isRowActionPending(row.id)"
              @click="handleToggle(row)"
            />
          </el-tooltip>
          <el-tooltip content="删除任务" placement="top">
            <el-button
              size="small"
              type="danger"
              :icon="Delete"
              :disabled="isRowActionPending(row.id)"
              @click="handleDelete(row)"
            />
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <ListPagination
      v-model:page="pagination.page"
      v-model:page-size="pagination.pageSize"
      :total="pagination.total"
      :page-sizes="[10, 20, 50, 100]"
      @change="loadJobs"
    />

    <JobFormDialog
      v-model="dialogVisible"
      :title="dialogTitle"
      :form-data="formData"
      :form-rules="formRules"
      :saving="saving"
      :data-server-options="dataServerOptions"
      :media-server-options="mediaServerOptions"
      :exts-loading="extsLoading"
      :exclude-dirs-text="excludeDirsText"
      :current-server-has-api="currentServerHasApi"
      :current-server-supports-url="currentServerSupportsUrl"
      :show-media-dir-warning="showMediaDirWarning"
      :data-server-access-path="currentServer?.accessPath || ''"
      :data-server-mount-path="currentServer?.mountPath || ''"
      @update:exclude-dirs-text="excludeDirsText = $event"
      @submit="handleSave"
      @server-change="handleServerChange"
      @open-path="({ field, options }) => openPathDialog(field, options)"
    />

    <PathDialog
      v-model="pathDlg.visible.value"
      :mode="pathDlg.mode.value"
      :path="pathDlg.path.value"
      :rows="pathDlg.rows.value"
      :loading="pathDlg.loading.value"
      :selected-name="pathDlg.selectedName.value"
      :selected-names="pathDlg.selectedNames.value"
      :at-root="pathDlg.atRoot.value"
      @up="pathDlg.goUp"
      @to-root="pathDlg.goRoot"
      @jump="pathDlg.jump"
      @enter="(name) => pathDlg.enterDirectory(name)"
      @select="handlePathSelect"
      @toggle="handlePathToggle"
      @confirm="handlePathConfirm"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Plus, Search, Switch, VideoPlay } from '@element-plus/icons-vue'
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
import { getServerList, listDirectories } from '@/api/servers'
import { normalizeListResponse } from '@/api/normalize'
import ListPagination from '@/components/common/ListPagination.vue'
import FilterToolbar from '@/components/common/FilterToolbar.vue'
import JobFormDialog from '@/components/jobs/JobFormDialog.vue'
import PathDialog from '@/components/common/PathDialog.vue'
import { usePathDialog, normalizePath, joinPath } from '@/composables/usePathDialog'
import { DEFAULT_MEDIA_EXTS, DEFAULT_META_EXTS } from '@/constants/defaults'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

const loading = ref(false)
const saving = ref(false)
const jobList = ref([])
const dataServers = ref([])
const mediaServers = ref([])

// 局部操作loading状态（按行ID追踪）
const actionLoading = reactive({
  trigger: {}, // 触发执行的loading状态
  toggle: {}   // 启用/禁用的loading状态
})

const filters = reactive({
  status: '',
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
  local_dir: '',
  exclude_dirs: [],
  schedule_enabled: false,
  cron: '',
  sync_opts: createDefaultSyncOpts(),
  metadata_mode: 'copy',
  thread_count: 4,
  cleanup_opts: defaultCleanupOptions.slice(),
  strm_mode: 'local',
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

const formRules = {
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  data_server_id: [{ required: true, message: '请选择数据服务器', trigger: 'change' }],
  media_dir: [{ required: true, message: '请输入媒体目录', trigger: 'blur' }],
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
      mountPath: options.mount_path || ''
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

const currentServerSupportsUrl = computed(() => {
  return currentServer.value?.supportsUrl || false
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

// 判断某一行是否有操作正在进行
const isRowActionPending = (rowId) => {
  return actionLoading.trigger[rowId] || actionLoading.toggle[rowId]
}

function parseOptions(options) {
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
  if (mode === 'copy' || mode === 'download') return mode
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
    dataServers.value = normalizeListResponse(dataResp).list
    mediaServers.value = normalizeListResponse(mediaResp).list
  } catch (error) {
    console.error('加载服务器列表失败:', error)
  }
}

const loadJobs = async () => {
  loading.value = true
  try {
    const params = {
      enabled: filters.status === 'enabled' ? 'true' :
               filters.status === 'disabled' ? 'false' : undefined,
      keyword: filters.keyword,
      page: pagination.page,
      pageSize: pagination.pageSize
    }
    const response = await getJobList(params)
    const { list, total } = normalizeListResponse(response)
    jobList.value = list
    pagination.total = total
  } catch (error) {
    console.error('加载任务列表失败:', error)
  } finally {
    loading.value = false
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

// 方法：服务器变更时的处理
const handleServerChange = () => {
  // 如果切换到不支持 URL 的服务器，自动切换到 local 模式
  if (!currentServerSupportsUrl.value) {
    formData.strm_mode = 'local'
  }

  applyDefaultMediaDir()
}

// 目录选择
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
  if (server.type !== 'local' && isLocalLikePath(accessPath)) {
    return '/'
  }
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

const buildDirectoryParams = (path) => {
  const server = currentServer.value
  if (!server || server.type === 'local' || isLocalLikePath(path)) {
    return { path, mode: 'local' }
  }
  return {
    path,
    mode: 'api',
    type: server.type,
    host: server.host,
    port: server.port,
    apiKey: server.api_key || server.apiKey
  }
}

const pathDlg = usePathDialog({
  loader: (path) => listDirectories(buildDirectoryParams(path)),
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
  if (field === 'exclude_dirs' && formData.media_dir) {
    return normalizePath(formData.media_dir)
  }
  if (field === 'exclude_dirs') {
    return resolveAccessRoot()
  }
  return '/'
}

const openPathDialog = async (field, options = {}) => {
  pathDialogField.value = field
  const dialogRoot = resolveDialogRoot(field)
  const initialPath = field === 'exclude_dirs' || field === 'media_dir'
    ? dialogRoot
    : (typeof formData[field] === 'string' ? formData[field] : '/')
  await pathDlg.open({
    mode: options.multiple ? 'multi' : 'single',
    root: dialogRoot,
    path: initialPath || dialogRoot
  })

  if (options.multiple) {
    pathDlg.selectedNames.value = (formData.exclude_dirs || [])
      .map(item => toAbsolutePath(item, dialogRoot))
      .filter(Boolean)
  } else if (typeof formData[field] === 'string' && formData[field]) {
    pathDlg.selectedName.value = normalizePath(formData[field])
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
    if (selected.length === 0) {
      ElMessage.warning('请至少选择一个目录')
      return
    }
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
    dialogVisible.value = false
    loadJobs()
  } catch (error) {
    if (error?.message) {
      console.error('保存任务失败:', error)
    }
  } finally {
    saving.value = false
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除任务「${row.name}」吗？`, '提示', {
      type: 'warning'
    })
    await deleteJob(row.id)
    ElMessage.success('任务已删除')
    loadJobs()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除任务失败:', error)
    }
  }
}

const handleToggle = async (row) => {
  if (actionLoading.toggle[row.id]) return // 防止重复点击

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
    // 确保loading状态被清理
    delete actionLoading.toggle[row.id]
  }
}

const handleTrigger = async (row) => {
  if (actionLoading.trigger[row.id]) return // 防止重复点击

  try {
    actionLoading.trigger[row.id] = true
    await triggerJob(row.id)
    ElMessage.success('任务已触发执行')

    // 刷新列表以更新 last_run_at 等字段
    await loadJobs()
  } catch (error) {
    console.error('触发任务失败:', error)
  } finally {
    // 确保loading状态被清理
    delete actionLoading.trigger[row.id]
  }
}

const getServerName = (row, type) => {
  if (type === 'data') {
    return row.data_server_name || row.data_server?.name || row.data_server_id || '-'
  }
  return row.media_server_name || row.media_server?.name || row.media_server_id || '-'
}

onMounted(() => {
  loadServers()
  loadJobs()
})
</script>

<style scoped lang="scss">
</style>
