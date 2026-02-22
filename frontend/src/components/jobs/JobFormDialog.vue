<template>
  <el-dialog
    v-model="visible"
    :title="title"
    width="800px"
    destroy-on-close
    class="task-config-dialog"
  >
    <el-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-position="top"
      label-width="var(--form-label-width)"
      class="compact-form"
    >
      <JobBaseInfoCard
        :form-data="formData"
        :data-server-options="dataServerOptions"
        :media-server-options="mediaServerOptions"
        :handle-server-change="handleServerChange"
      />

      <JobPathConfigCard
        :form-data="formData"
        :access-path-text="accessPathText"
        :mount-path-text="mountPathText"
        :remote-root-text="remoteRootText"
        :media-dir-proxy="mediaDirProxy"
        :exclude-dirs-proxy="excludeDirsProxy"
        :media-dir-disabled="mediaDirDisabled"
        :show-media-dir-warning="showMediaDirWarning"
        :current-server-has-api="currentServerHasApi"
        :open-path="openPath"
        @update:mediaDirProxy="mediaDirProxy = $event"
        @update:excludeDirsProxy="excludeDirsProxy = $event"
      />

      <JobCleanupCard :form-data="formData" :cleanup-options="cleanupOptions" />

      <JobSyncCard
        :form-data="formData"
        :sync-option-groups="syncOptionGroups"
        v-model:sync-strategy="syncStrategy"
        v-model:meta-strategy="metaStrategy"
        :current-server-has-api="currentServerHasApi"
        :current-server-is-local="currentServerIsLocal"
        :force-remote-only="forceRemoteOnly"
      />

      <JobStrmCard
        :form-data="formData"
        :current-server-supports-url="currentServerSupportsUrl"
        :force-remote-only="forceRemoteOnly"
        :exts-loading="extsLoading"
        :add-strm-rule="addStrmRule"
        :remove-strm-rule="removeStrmRule"
        :move-strm-rule="moveStrmRule"
      />
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleSubmit">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref } from 'vue'
import { joinPath, normalizePath } from '@/composables/usePathDialog'
import JobBaseInfoCard from '@/components/jobs/form/JobBaseInfoCard.vue'
import JobPathConfigCard from '@/components/jobs/form/JobPathConfigCard.vue'
import JobCleanupCard from '@/components/jobs/form/JobCleanupCard.vue'
import JobSyncCard from '@/components/jobs/form/JobSyncCard.vue'
import JobStrmCard from '@/components/jobs/form/JobStrmCard.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, required: true },
  formData: { type: Object, required: true },
  formRules: { type: Object, required: true },
  saving: { type: Boolean, default: false },
  dataServerOptions: { type: Array, default: () => [] },
  mediaServerOptions: { type: Array, default: () => [] },
  extsLoading: { type: Boolean, default: false },
  excludeDirsText: { type: String, default: '' },
  currentServerHasApi: { type: Boolean, default: false },
  currentServerSupportsUrl: { type: Boolean, default: false },
  currentServerIsLocal: { type: Boolean, default: false },
  showMediaDirWarning: { type: Boolean, default: false },
  forceRemoteOnly: { type: Boolean, default: false },
  mediaDirDisabled: { type: Boolean, default: false },
  dataServerAccessPath: { type: String, default: '' },
  dataServerMountPath: { type: String, default: '' },
  dataServerRemoteRoot: { type: String, default: '' }
})

const emit = defineEmits([
  'update:modelValue',
  'update:excludeDirsText',
  'submit',
  'server-change',
  'open-path'
])

const formRef = ref(null)

const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const excludeDirsProxy = computed({
  get: () => props.excludeDirsText,
  set: (value) => emit('update:excludeDirsText', value)
})

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

const accessPathRoot = computed(() => {
  const raw = String(props.dataServerAccessPath || '').trim()
  if (!raw) return '/'
  return normalizePath(raw)
})

const accessPathText = computed(() => {
  const raw = String(props.dataServerAccessPath || '').trim()
  if (!raw) return '未设置'
  return normalizePath(raw)
})

const mountPathText = computed(() => {
  const raw = String(props.dataServerMountPath || '').trim()
  if (!raw) return '未设置'
  return normalizePath(raw)
})

const remoteRootText = computed(() => {
  const raw = String(props.formData.remote_root || props.dataServerRemoteRoot || '').trim()
  if (!raw) return '未设置'
  return normalizePath(raw)
})

const mediaDirProxy = computed({
  get: () => {
    const value = String(props.formData.media_dir || '').trim()
    if (!value) return ''
    return toRelativePath(value, accessPathRoot.value)
  },
  set: (value) => {
    const trimmed = String(value || '').trim()
    if (!trimmed) {
      props.formData.media_dir = ''
      return
    }
    if (trimmed === '.') {
      props.formData.media_dir = accessPathRoot.value
      return
    }
    const cleaned = trimmed.replace(/^\.\/+/, '')
    if (trimmed.startsWith('/')) {
      props.formData.media_dir = normalizePath(trimmed)
      return
    }
    props.formData.media_dir = normalizePath(joinPath(accessPathRoot.value, cleaned))
  }
})

const syncStrategy = computed({
  get: () => {
    if (props.formData?.sync_opts?.full_resync) return 'full_resync'
    return 'full_update'
  },
  set: (value) => {
    if (!props.formData?.sync_opts) return
    props.formData.sync_opts.full_resync = value === 'full_resync'
    props.formData.sync_opts.full_update = value === 'full_update'
  }
})

const metaStrategy = computed({
  get: () => {
    if (props.formData?.sync_opts?.overwrite_meta) return 'overwrite_meta'
    if (props.formData?.sync_opts?.skip_meta) return 'skip_meta'
    return 'update_meta'
  },
  set: (value) => {
    if (!props.formData?.sync_opts) return
    props.formData.sync_opts.update_meta = value === 'update_meta'
    props.formData.sync_opts.overwrite_meta = value === 'overwrite_meta'
    props.formData.sync_opts.skip_meta = value === 'skip_meta'
  }
})

const handleServerChange = (value) => emit('server-change', value)

const openPath = (field, options) => {
  emit('open-path', { field, options })
}

const handleSubmit = async () => {
  try {
    await formRef.value?.validate()
    emit('submit')
  } catch (error) {
    // validation failed
  }
}

const addStrmRule = () => {
  if (!Array.isArray(props.formData.strm_replace_rules)) {
    props.formData.strm_replace_rules = []
  }
  props.formData.strm_replace_rules.push({ from: '', to: '' })
}

const removeStrmRule = (index) => {
  if (!Array.isArray(props.formData.strm_replace_rules)) return
  props.formData.strm_replace_rules.splice(index, 1)
}

const moveStrmRule = (index, direction) => {
  const rules = props.formData.strm_replace_rules
  if (!Array.isArray(rules)) return
  const target = index + direction
  if (target < 0 || target >= rules.length) return
  const [item] = rules.splice(index, 1)
  rules.splice(target, 0, item)
}

const cleanupOptions = [
  { value: 'clean_local', label: '清除本地目录', help: '删除本地输出中找不到源文件的条目' },
  { value: 'clean_folders', label: '清除无效文件夹', help: '移除没有有效媒体的空目录' },
  { value: 'clean_symlinks', label: '清除无效软链接', help: '清理指向不存在源文件的软链接' },
  { value: 'clean_meta', label: '清除无效元数据', help: '移除无对应媒体的元数据文件' }
]

const syncOptionGroups = {
  general: [
    { value: 'full_resync', label: '全量同步', help: '不管之前是否同步过，重新扫描并同步全部内容（会覆盖 STRM）' },
    { value: 'full_update', label: '更新同步', help: '仅对新增条目执行同步/更新' }
  ],
  meta: [
    { value: 'update_meta', label: '更新元数据', help: '比对内容，相同跳过，不同/缺失则更新' },
    { value: 'overwrite_meta', label: '元数据覆盖', help: '无条件覆盖已存在的元数据文件' },
    { value: 'skip_meta', label: '元数据跳过', help: '跳过同名元数据文件，不做处理' }
  ]
}
</script>

<style scoped lang="scss">
.task-config-dialog {
  :deep(.inline-field .el-form-item__content) {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }

  :deep(.el-form-item__label) {
    font-size: 14px;
    color: var(--el-text-color-regular);
    font-weight: 500;
    display: flex;
    align-items: center;
  }

  :deep(.el-form-item__label::before) {
    margin-right: 4px;
  }

  :deep(.el-form-item) {
    margin-bottom: 12px;
  }

  :deep(.compact-field .el-form-item__content) {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }

  :deep(.compact-field .el-input),
  :deep(.compact-field .el-select),
  :deep(.compact-field .el-input-number) {
    flex: 1;
    min-width: 0;
  }

  :deep(.compact-form .el-form-item) {
    position: relative;
  }

  :deep(.compact-form .el-form-item__error) {
    position: absolute;
    right: 0;
    top: 0;
    margin-top: 0;
    padding-top: 0;
    line-height: 1.2;
    font-size: 12px;
    text-align: right;
    max-width: 260px;
  }
}
</style>
