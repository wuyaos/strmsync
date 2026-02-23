import { reactive } from 'vue'
import { DEFAULT_MEDIA_EXTS, DEFAULT_META_EXTS } from '@/constants/defaults'

export const createDefaultSyncOpts = () => ({
  full_resync: false,
  update_meta: true,
  skip_meta: false
})

export const defaultCleanupOptions = ['clean_local', 'clean_folders', 'clean_meta']

export const createDefaultFormData = () => ({
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
  max_concurrency: 4,
  cleanup_opts: defaultCleanupOptions.slice(),
  strm_mode: 'local',
  prefer_remote_list: false,
  min_file_size: 10,
  strm_replace_rules: [],
  media_exts: DEFAULT_MEDIA_EXTS.slice(),
  meta_exts: DEFAULT_META_EXTS.slice(),
  enabled: true
})

export const useJobFormState = ({ currentServerHasApi }) => {
  const formData = reactive(createDefaultFormData())

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

  const resetForm = () => {
    Object.assign(formData, createDefaultFormData())
  }

  const normalizeMetadataMode = (mode) => {
    if (mode === 'copy' || mode === 'download' || mode === 'none') return mode
    if (mode === 'api') return 'download'
    return 'copy'
  }

  const normalizeSyncOptions = () => {
    const syncOpts = formData.sync_opts || {}
    if (syncOpts.full_resync) {
      // Full resync implies skipping updates based on other criteria, but we'll let backend handle exact logic
    }

    if (syncOpts.skip_meta) {
      syncOpts.update_meta = false
    } else {
      syncOpts.update_meta = true
    }

    // Remove overwrite_meta handling, as it's deprecated from backend

    formData.metadata_mode = normalizeMetadataMode(formData.metadata_mode)
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

  return {
    formData,
    formRules,
    resetForm,
    normalizeSyncOptions,
    applyOptionsToForm
  }
}
