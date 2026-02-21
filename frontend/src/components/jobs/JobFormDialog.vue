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
      <!-- 基本信息 -->
      <el-card class="section-card" shadow="never">
        <template #header>
          <div class="section-title">基本信息</div>
        </template>
        <el-row :gutter="20" class="section-row">
          <el-col :xs="24" :md="8">
            <el-form-item label="任务名称" prop="name">
              <el-input v-model="formData.name" placeholder="每周电影同步" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="数据服务器" prop="data_server_id">
              <el-select
                v-model="formData.data_server_id"
                placeholder="选择数据服务器"
                class="w-full"
                @change="handleServerChange"
              >
                <el-option
                  v-for="server in dataServerOptions"
                  :key="server.id"
                  :label="server.label"
                  :value="server.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item prop="media_server_id">
              <template #label>
                <span>媒体服务器（可选）</span>
                <el-tooltip content="用于后续媒体库联动，不影响同步流程" placement="top">
                  <el-icon class="info-icon"><InfoFilled /></el-icon>
                </el-tooltip>
              </template>
              <el-select
                v-model="formData.media_server_id"
                placeholder="可选：选择媒体服务器"
                clearable
                class="w-full"
              >
                <el-option
                  v-for="server in mediaServerOptions"
                  :key="server.id"
                  :label="server.label"
                  :value="server.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
      </el-card>

      <!-- 目录配置 -->
      <el-card class="section-card" shadow="never">
        <template #header>
          <div class="section-title">目录配置</div>
        </template>
        <div class="path-summary">
          <div>
            <span class="path-summary-label">访问目录：</span>
            <span class="path-summary-value">{{ accessPathText }}</span>
          </div>
          <div>
            <span class="path-summary-label">挂载目录：</span>
            <span class="path-summary-value">{{ mountPathText }}</span>
          </div>
          <div>
            <span class="path-summary-label">远程根目录：</span>
            <span class="path-summary-value">{{ remoteRootText }}</span>
          </div>
        </div>
        <el-row :gutter="20" class="section-row">
          <el-col :xs="24" :md="12">
            <el-form-item label="媒体目录" prop="media_dir" class="compact-field">
              <template #label>
                <div class="label-inline">
                  <span class="label-text">媒体目录</span>
                  <span class="label-help">
                    <span v-if="mediaDirDisabled">请先选择数据服务器。</span>
                    <template v-else>
                      <el-icon v-if="showMediaDirWarning" class="warning-icon"><WarningFilled /></el-icon>
                      默认为数据服务器的访问目录的根目录
                    </template>
                  </span>
                </div>
              </template>
              <el-input v-model="mediaDirProxy" placeholder="movies" :disabled="mediaDirDisabled">
                <template #suffix>
                  <el-button link :icon="FolderOpened" :disabled="mediaDirDisabled" @click="openPath('media_dir')" />
                </template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col v-if="currentServerHasApi" :xs="24" :md="12">
            <el-form-item label="远程根目录" prop="remote_root" class="compact-field">
              <template #label>
                <div class="label-inline">
                  <span class="label-text">远程根目录</span>
                  <span class="label-help">用于 CD2/OpenList 的远程 API 根路径</span>
                </div>
              </template>
              <el-input v-model="formData.remote_root" placeholder="/">
                <template #suffix>
                  <el-button link :icon="FolderOpened" @click="openPath('remote_root', { forceApi: true })" />
                </template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="本地输出" prop="local_dir" class="compact-field">
              <template #label>
                <div class="label-inline">
                  <span class="label-text">本地输出</span>
                  <span class="label-help">用于存放生成的 STRM 文件和元数据</span>
                </div>
              </template>
              <el-input v-model="formData.local_dir" placeholder="/local/strm/movies">
                <template #suffix>
                  <el-button link :icon="FolderOpened" @click="openPath('local_dir')" />
                </template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="排除目录" prop="exclude_dirs" class="compact-field">
          <template #label>
            <div class="label-inline">
              <span class="label-text">排除目录</span>
              <span class="label-help">可多选，支持手动输入(用","隔开)</span>
            </div>
          </template>
          <el-input v-model="excludeDirsProxy" placeholder="temp, .trash">
            <template #suffix>
              <el-button link :icon="FolderOpened" @click="openPath('exclude_dirs', { multiple: true })" />
            </template>
          </el-input>
        </el-form-item>
      </el-card>

      <!-- 清除功能 -->
      <el-card class="section-card" shadow="never">
        <template #header>
          <div class="section-title">清除功能</div>
        </template>
        <div class="option-grid">
          <div v-for="item in cleanupOptions" :key="item.value" class="option-item">
            <el-checkbox v-model="formData.cleanup_opts" :label="item.value">
              {{ item.label }}
            </el-checkbox>
            <div class="help-text">{{ item.help }}</div>
          </div>
        </div>
      </el-card>

      <!-- 同步功能 -->
      <el-card class="section-card" shadow="never">
        <template #header>
          <div class="section-title">同步功能</div>
        </template>

        <el-form-item label="定时同步">
          <template #label>
            <div class="label-inline">
              <span class="label-text">定时同步</span>
              <span class="label-help">启用后按 Cron 规则定时执行同步</span>
            </div>
          </template>
          <div class="switch-row">
            <el-switch v-model="formData.schedule_enabled" />
          </div>
        </el-form-item>

        <el-form-item v-if="formData.schedule_enabled" label="Cron 表达式" prop="cron" class="compact-field">
          <template #label>
            <div class="label-inline">
              <span class="label-text">Cron 表达式</span>
              <span class="label-help">格式：分 时 日 月 周（例如：0 */6 * * * 表示每 6 小时执行一次）</span>
            </div>
          </template>
          <el-input v-model="formData.cron" placeholder="0 */6 * * *" />
        </el-form-item>

        <div class="subsection-title">同步策略</div>
        <div class="option-grid">
          <div v-for="item in syncOptionGroups.general" :key="item.value" class="option-item">
            <el-radio v-model="syncStrategy" :label="item.value">{{ item.label }}</el-radio>
            <div class="help-text">{{ item.help }}</div>
          </div>
        </div>
        <el-form-item v-if="currentServerHasApi" label="远程列表优先">
          <div class="switch-row">
            <el-switch v-model="formData.prefer_remote_list" :disabled="forceRemoteOnly" />
            <span class="help-text">
              优先使用远程 API 获取文件列表（更快），本地默认关闭
              <span v-if="forceRemoteOnly">（OpenList 未配置访问目录时强制开启）</span>
            </span>
          </div>
        </el-form-item>

        <div class="subsection-title">元数据选项</div>
        <div class="option-grid">
          <div v-for="item in syncOptionGroups.meta" :key="item.value" class="option-item">
            <el-radio v-model="metaStrategy" :label="item.value">{{ item.label }}</el-radio>
            <div class="help-text">{{ item.help }}</div>
          </div>
        </div>

        <el-row :gutter="20" class="section-row">
          <el-col :xs="24" :md="12">
            <el-form-item label="元数据模式" class="inline-field">
              <el-radio-group v-model="formData.metadata_mode">
                <el-radio label="copy" :disabled="forceRemoteOnly">复制文件</el-radio>
                <el-radio label="download" :disabled="currentServerIsLocal">下载文件</el-radio>
                <el-radio label="none" :disabled="forceRemoteOnly">不处理</el-radio>
              </el-radio-group>
              <div v-if="currentServerIsLocal && formData.metadata_mode === 'download'" class="warning-text">
                <el-icon class="warning-icon"><WarningFilled /></el-icon>
                Local 模式不支持下载，已自动切换为复制模式
              </div>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="同步线程数" class="inline-field">
              <el-input v-model="formData.thread_count" class="input-short" type="number" min="1" max="16" step="1" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-card>

      <!-- STRM 配置 -->
      <el-card class="section-card" shadow="never">
        <template #header>
          <div class="section-title">STRM 配置</div>
        </template>
        <el-form-item label="STRM 模式" class="compact-field">
          <template #label>
            <div class="label-inline">
              <span class="label-text">STRM 模式</span>
              <span
                v-if="!currentServerSupportsUrl"
                class="label-help label-help--warning"
              >
                当前数据服务器不支持 URL 访问模式
              </span>
              <span
                v-else-if="forceRemoteOnly"
                class="label-help label-help--warning"
              >
                OpenList 未配置访问目录时仅支持远程 URL
              </span>
            </div>
          </template>
          <el-radio-group v-model="formData.strm_mode" :disabled="!currentServerSupportsUrl">
            <el-radio label="local" :disabled="forceRemoteOnly">本地路径</el-radio>
            <el-radio label="url">远程 URL</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-row :gutter="20" class="section-row">
          <el-col :xs="24" :md="12">
            <el-form-item>
              <template #label>
                <span class="field-label">媒体文件大小阈值</span>
                <el-tooltip content="小于此大小的文件将直接复制/下载，而不是生成 STRM" placement="top">
                  <el-icon class="info-icon"><InfoFilled /></el-icon>
                </el-tooltip>
              </template>
              <div class="inline-input">
                <el-input v-model="formData.min_file_size" class="input-short" type="number" min="0" step="100" />
                <span class="unit-text">MB</span>
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20" class="section-row">
          <el-col :xs="24" :md="24">
            <el-form-item>
              <template #label>
                <span class="field-label">STRM 替换规则</span>
                <el-tooltip content="按顺序依次替换，支持路径与URL" placement="top">
                  <el-icon class="info-icon"><InfoFilled /></el-icon>
                </el-tooltip>
              </template>
              <div class="strm-rule-list">
                <div v-if="!formData.strm_replace_rules?.length" class="help-text">暂无规则</div>
                <div
                  v-for="(rule, index) in formData.strm_replace_rules"
                  :key="index"
                  class="strm-rule-row"
                >
                  <el-input v-model="rule.from" placeholder="原始字符串" class="rule-input" />
                  <el-input v-model="rule.to" placeholder="替换字符串" class="rule-input" />
                  <div class="rule-actions">
                    <el-button
                      link
                      :icon="ArrowUp"
                      :disabled="index === 0"
                      @click="moveStrmRule(index, -1)"
                    />
                    <el-button
                      link
                      :icon="ArrowDown"
                      :disabled="index === formData.strm_replace_rules.length - 1"
                      @click="moveStrmRule(index, 1)"
                    />
                    <el-button link :icon="Delete" @click="removeStrmRule(index)" />
                  </div>
                </div>
                <el-button size="small" type="primary" plain @click="addStrmRule">添加规则</el-button>
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="媒体文件后缀" class="compact-field">
          <template #label>
            <div class="label-inline">
              <span class="label-text">媒体文件后缀</span>
              <span class="label-help">媒体文件扩展名白名单</span>
            </div>
          </template>
          <el-select
            v-model="formData.media_exts"
            multiple
            filterable
            allow-create
            default-first-option
            :loading="extsLoading"
            placeholder="添加后缀（.mkv, .mp4）"
            class="w-full"
          />
        </el-form-item>

        <el-form-item label="元数据后缀" class="compact-field">
          <template #label>
            <div class="label-inline">
              <span class="label-text">元数据后缀</span>
              <span class="label-help">元数据文件扩展名白名单</span>
            </div>
          </template>
          <el-select
            v-model="formData.meta_exts"
            multiple
            filterable
            allow-create
            default-first-option
            :loading="extsLoading"
            placeholder="添加后缀（.nfo, .jpg）"
            class="w-full"
          />
        </el-form-item>
      </el-card>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="handleSubmit">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref } from 'vue'
import ArrowDown from '~icons/ep/arrow-down'
import ArrowUp from '~icons/ep/arrow-up'
import Delete from '~icons/ep/delete'
import FolderOpened from '~icons/ep/folder-opened'
import InfoFilled from '~icons/ep/info-filled'
import WarningFilled from '~icons/ep/warning-filled'
import { joinPath, normalizePath } from '@/composables/usePathDialog'

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
  .section-card {
    margin-bottom: 12px;
    border-color: var(--el-border-color-lighter);
  }

  .section-title {
    font-size: 16px;
    font-weight: 600;
    color: var(--el-text-color-primary);
  }

  .label-inline {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
  }

  .label-text {
    white-space: nowrap;
  }

  .label-help {
    margin-left: auto;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    line-height: 1.4;
    max-width: 360px;
    white-space: normal;
    text-align: right;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .label-help--warning {
    color: var(--el-color-warning);
  }

  .help-text {
    font-size: 12px;
    color: var(--el-text-color-secondary);
    line-height: 1.5;
    margin-top: 4px;
    display: flex;
    align-items: flex-start;
    gap: 4px;
  }

  .warning-text {
    font-size: 12px;
    color: var(--el-color-warning);
    margin-top: 4px;
    display: flex;
    align-items: flex-start;
    gap: 4px;
    flex-basis: 100%;
  }

  .warning-icon {
    color: var(--el-color-warning);
    margin-top: 2px;
  }

  .info-icon {
    margin-left: 4px;
    color: var(--el-text-color-secondary);
  }

  .subsection-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--el-text-color-regular);
    margin: 12px 0 8px;
  }

  .option-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 12px;
    margin-bottom: 8px;
  }

  .option-item {
    padding: 8px 10px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
    background: var(--el-fill-color-blank);
  }

  .path-summary {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-wrap: wrap;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    margin-bottom: 12px;
  }

  .path-summary-label {
    color: var(--el-text-color-regular);
  }

  .path-summary-value {
    font-family: 'Courier New', monospace;
    color: var(--el-text-color-primary);
  }

  .section-row {
    align-items: flex-start;
  }

  .switch-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .inline-field {
    :deep(.el-form-item__content) {
      display: flex;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
    }
  }

  .inline-input {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .unit-text {
    color: var(--el-text-color-regular);
  }

  .strm-rule-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
  }

  .strm-rule-row {
    display: grid;
    grid-template-columns: 1fr 1fr auto;
    gap: 8px;
    align-items: center;
  }

  .rule-input {
    width: 100%;
  }

  .rule-actions {
    display: flex;
    align-items: center;
    gap: 4px;
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
