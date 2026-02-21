<template>
  <el-dialog
    v-model="visible"
    :title="dialogTitle"
    width="680px"
    destroy-on-close
    :close-on-click-modal="false"
    class="task-config-dialog"
  >
    <el-form
      ref="formRef"
      :model="formModel"
      :rules="formRules"
      label-position="top"
      label-width="var(--form-label-width)"
      class="compact-form"
    >
      <!-- 基础信息 -->
      <el-card class="section-card" shadow="never">
        <template #header>
          <div class="section-header">
            <div class="section-title">基础信息</div>
            <div class="section-actions">
              <el-switch v-model="formData.enabled" />
            </div>
          </div>
        </template>
        <el-row :gutter="12" class="section-row base-info-row">
          <el-col :xs="24" :md="12">
            <el-form-item label="服务器名称" prop="name" class="compact-field">
              <el-input
                v-model="formData.name"
                placeholder="Emby"
                clearable
                @input="handleNameInput"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="服务器类型" prop="type" class="compact-field">
              <div class="type-inline">
                <el-select
                  v-model="formData.type"
                  filterable
                  placeholder="选择服务器类型"
                  class="type-select"
                  @change="handleTypeChange"
                >
                  <el-option
                    v-for="option in serverTypeOptions"
                    :key="option.value"
                    :label="option.label"
                    :value="option.value"
                  >
                    <span>{{ option.label }}</span>
                    <span class="option-desc">
                      {{ option.description }}
                    </span>
                  </el-option>
                </el-select>
              </div>
            </el-form-item>
          </el-col>
        </el-row>
      </el-card>

      <!-- 数据服务器：动态表单 -->
      <template v-if="isDataMode">
        <template v-if="activeTypeDef">
          <template v-for="section in activeTypeDef.sections" :key="section.id">
            <el-card class="section-card" shadow="never">
              <template #header>
                <div class="section-header">
                  <div class="section-title">{{ section.label }}</div>
                  <div v-if="section.id === 'auth' && showTestButton" class="section-actions">
                    <div v-if="testStatus !== 'idle'" class="test-status" :class="`test-status--${testStatus}`">
                      <el-icon v-if="testStatus === 'running'" class="is-loading">
                        <Loading />
                      </el-icon>
                      <el-icon v-else-if="testStatus === 'success'">
                        <CircleCheckFilled />
                      </el-icon>
                      <el-icon v-else-if="testStatus === 'error'">
                        <CircleCloseFilled />
                      </el-icon>
                      <span>{{ testStatusText }}</span>
                    </div>
                    <el-tooltip
                      :disabled="!!formData.id"
                      content="未保存的服务器将进行临时测试"
                      placement="top"
                    >
                      <span>
                        <el-button
                          :icon="Link"
                          :loading="testing"
                          :disabled="!canTest"
                          @click="handleTestConnection"
                        >
                          测试
                        </el-button>
                      </span>
                    </el-tooltip>
                  </div>
                </div>
              </template>

              <!-- 横向布局 (row) -->
              <div v-if="section.layout === 'row'" class="form-row">
                <el-row :gutter="12">
                  <el-col
                    v-for="field in getVisibleFields(section.fields)"
                    :key="field.name"
                    :span="field.col_span || 12"
                  >
                    <el-form-item
                      v-if="field.type !== 'hidden'"
                      :label="field.label"
                      :prop="field.name"
                      class="compact-field"
                    >
                      <template #label>
                        <div class="label-inline">
                          <span>{{ field.label }}</span>
                          <span v-if="field.help" class="label-help">{{ field.help }}</span>
                        </div>
                      </template>
                      <!-- 路径字段 -->
                      <el-input
                        v-if="isPathField(field)"
                        v-model="dynamicModel[field.name]"
                        :placeholder="field.placeholder"
                      >
                        <template #suffix>
                          <el-button link :icon="FolderOpened" @click="openPathDialog(field)" />
                        </template>
                      </el-input>
                      <!-- 文本字段 -->
                      <el-input
                        v-else-if="isTextField(field)"
                        v-model="dynamicModel[field.name]"
                        :placeholder="field.placeholder"
                        clearable
                      />
                      <!-- 密码字段 -->
                      <el-input
                        v-else-if="field.type === 'password'"
                        v-model="dynamicModel[field.name]"
                        type="password"
                        show-password
                        :placeholder="field.placeholder"
                        clearable
                      />
                      <!-- 数字字段 -->
                      <el-input
                        v-else-if="field.type === 'number'"
                        v-model.number="dynamicModel[field.name]"
                        :placeholder="field.placeholder"
                        type="number"
                        :min="field.min ?? 1"
                        :max="field.max ?? 65535"
                        class="input-short"
                      />
                      <!-- 下拉选择 -->
                      <el-select
                        v-else-if="field.type === 'select'"
                        v-model="dynamicModel[field.name]"
                        placeholder="请选择"
                        class="w-full"
                      >
                        <el-option
                          v-for="option in field.options || []"
                          :key="option.value"
                          :label="option.label"
                          :value="option.value"
                        />
                      </el-select>
                      <!-- 单选按钮 -->
                      <el-radio-group
                        v-else-if="field.type === 'radio'"
                        v-model="dynamicModel[field.name]"
                      >
                        <el-radio
                          v-for="option in field.options || []"
                          :key="option.value"
                          :value="option.value"
                        >
                          {{ option.label }}
                        </el-radio>
                      </el-radio-group>
                      <!-- 其他类型 -->
                      <el-input
                        v-else
                        v-model="dynamicModel[field.name]"
                        :placeholder="field.placeholder"
                        clearable
                      />
                    </el-form-item>
                  </el-col>
                </el-row>
              </div>

              <!-- 纵向布局（默认） -->
              <template v-else>
                <el-form-item
                  v-for="field in getVisibleFields(section.fields)"
                  :key="field.name"
                  v-show="field.type !== 'hidden'"
                  :label="field.label"
                  :prop="field.name"
                  class="compact-field"
                >
                  <template #label>
                    <div class="label-inline">
                      <span>{{ field.label }}</span>
                      <span v-if="field.help" class="label-help">{{ field.help }}</span>
                    </div>
                  </template>
                  <!-- 路径字段 -->
                  <el-input
                    v-if="isPathField(field)"
                    v-model="dynamicModel[field.name]"
                    :placeholder="field.placeholder"
                  >
                    <template #suffix>
                      <el-button link :icon="FolderOpened" @click="openPathDialog(field)" />
                    </template>
                  </el-input>
                  <!-- 文本字段 -->
                  <el-input
                    v-else-if="isTextField(field)"
                    v-model="dynamicModel[field.name]"
                    :placeholder="field.placeholder"
                    clearable
                  />
                  <!-- 密码字段 -->
                  <el-input
                    v-else-if="field.type === 'password'"
                    v-model="dynamicModel[field.name]"
                    type="password"
                    show-password
                    :placeholder="field.placeholder"
                    clearable
                  />
                  <!-- 数字字段 -->
                  <el-input
                    v-else-if="field.type === 'number'"
                    v-model.number="dynamicModel[field.name]"
                    :placeholder="field.placeholder"
                    type="number"
                    :min="field.min ?? 1"
                    :max="field.max ?? 65535"
                    class="input-short"
                  />
                  <!-- 下拉选择 -->
                  <el-select
                    v-else-if="field.type === 'select'"
                    v-model="dynamicModel[field.name]"
                    placeholder="请选择"
                    class="w-full"
                  >
                    <el-option
                      v-for="option in field.options || []"
                      :key="option.value"
                      :label="option.label"
                      :value="option.value"
                    />
                  </el-select>
                  <!-- 单选按钮 -->
                  <el-radio-group
                    v-else-if="field.type === 'radio'"
                    v-model="dynamicModel[field.name]"
                  >
                    <el-radio
                      v-for="option in field.options || []"
                      :key="option.value"
                      :value="option.value"
                    >
                      {{ option.label }}
                    </el-radio>
                  </el-radio-group>
                  <!-- 其他类型 -->
                  <el-input
                    v-else
                    v-model="dynamicModel[field.name]"
                    :placeholder="field.placeholder"
                    clearable
                  />
                </el-form-item>
              </template>
            </el-card>
          </template>
        </template>
        <el-alert
          v-else
          title="请选择服务器类型"
          type="info"
          :closable="false"
        />
      </template>

      <!-- 媒体服务器：静态表单 -->
      <template v-else>
        <!-- 连接信息 -->
        <el-card class="section-card" shadow="never">
          <template #header>
            <div class="section-header">
              <div class="section-title">连接信息</div>
              <div v-if="showTestButton" class="section-actions">
                <div v-if="testStatus !== 'idle'" class="test-status" :class="`test-status--${testStatus}`">
                  <el-icon v-if="testStatus === 'running'" class="is-loading">
                    <Loading />
                  </el-icon>
                  <el-icon v-else-if="testStatus === 'success'">
                    <CircleCheckFilled />
                  </el-icon>
                  <el-icon v-else-if="testStatus === 'error'">
                    <CircleCloseFilled />
                  </el-icon>
                  <span>{{ testStatusText }}</span>
                </div>
                <el-tooltip
                  :disabled="!!formData.id"
                  content="未保存的服务器将进行临时测试"
                  placement="top"
                >
                  <span>
                    <el-button
                      :icon="Link"
                      :loading="testing"
                      :disabled="!canTest"
                      @click="handleTestConnection"
                    >
                      测试
                    </el-button>
                  </span>
                </el-tooltip>
              </div>
            </div>
          </template>

          <div class="form-row">
            <el-row :gutter="12">
              <el-col :span="14">
                <el-form-item label="主机地址" prop="host" class="compact-field">
                  <el-input
                    v-model="formData.host"
                    :placeholder="hostPlaceholder"
                    clearable
                  />
                </el-form-item>
              </el-col>
              <el-col :span="10">
                <el-form-item label="端口号" prop="port" class="compact-field">
                  <el-input
                    v-model.number="formData.port"
                    type="number"
                    :min="1"
                    :max="65535"
                    :step="1"
                    class="input-short"
                  />
                </el-form-item>
              </el-col>
            </el-row>
          </div>

          <!-- API 密钥（如需要） -->
          <el-form-item v-if="needsApiKey" :label="apiKeyLabel" prop="api_key" class="compact-field">
            <template #label>
              <div class="label-inline">
                <span>{{ apiKeyLabel }}</span>
                <span v-if="serverTypeHint" class="label-help">{{ serverTypeHint }}</span>
              </div>
            </template>
            <el-input
              v-model="formData.api_key"
              type="password"
              show-password
              :placeholder="`请输入${apiKeyLabel}`"
              clearable
            />
          </el-form-item>
        </el-card>
      </template>

      <!-- 高级选项（接口速率配置） -->
      <el-card v-if="showRate" class="section-card" shadow="never">
        <template #header>
          <div class="section-title">高级选项</div>
        </template>
        <el-collapse>
          <el-collapse-item title="接口速率配置（可选）" name="rate">
            <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="下载队列每秒处理数量" class="compact-field">
                <template #label>
                  <div class="label-inline">
                    <span>下载队列每秒处理数量</span>
                    <span class="label-help">0 表示使用系统设置</span>
                  </div>
                </template>
                <el-input
                  v-model.number="formData.download_rate_per_sec"
                  type="number"
                  :min="0"
                  :max="1000"
                  class="input-short"
                />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="接口速率(每秒请求数)" class="compact-field">
                <template #label>
                  <div class="label-inline">
                    <span>接口速率(每秒请求数)</span>
                    <span class="label-help">0 表示使用系统设置</span>
                  </div>
                </template>
                <el-input
                  v-model.number="formData.api_rate"
                  type="number"
                  :min="0"
                  :max="1000"
                  class="input-short"
                />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="接口重试次数" class="compact-field">
                <template #label>
                  <div class="label-inline">
                    <span>接口重试次数</span>
                    <span class="label-help">0 表示使用系统设置</span>
                  </div>
                </template>
                <el-input
                  v-model.number="formData.api_retry_max"
                  type="number"
                  :min="0"
                  :max="10"
                  class="input-short"
                />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="接口重试间隔(秒)" class="compact-field">
                <template #label>
                  <div class="label-inline">
                    <span>接口重试间隔(秒)</span>
                    <span class="label-help">0 表示使用系统设置</span>
                  </div>
                </template>
                <el-input
                  v-model.number="formData.api_retry_interval_sec"
                  type="number"
                  :min="0"
                  :max="60"
                  class="input-short"
                />
              </el-form-item>
            </el-col>
            </el-row>
          </el-collapse-item>
        </el-collapse>
      </el-card>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <div class="dialog-footer-left">
          <el-button @click="visible = false">取消</el-button>
        </div>
        <div class="dialog-footer-actions">
          <!-- 创建/更新按钮 -->
          <el-button
            type="primary"
            :loading="saving"
            :icon="isEdit ? Edit : Plus"
            @click="handleSave"
          >
            {{ isEdit ? '更新' : '创建' }}
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>

  <PathDialog
    v-model="pathDlg.visible.value"
    :mode="pathDlg.mode.value"
    :loading="pathDlg.loading.value"
    :path="pathDlg.path.value"
    :rows="pathDlg.rows.value"
    :has-more="pathDlg.hasMore.value"
    :selected-name="pathDlg.selectedName.value"
    :selected-names="pathDlg.selectedNames.value"
    :at-root="pathDlg.atRoot.value"
    :refresh-key="pathDlg.refreshKey.value"
    @up="pathDlg.goUp"
    @to-root="pathDlg.goRoot"
    @jump="pathDlg.jump"
    @enter="(name) => pathDlg.enterDirectory(name)"
    @select="handlePathSelect"
    @refresh="() => pathDlg.load(pathDlg.path.value)"
    @load-more="pathDlg.loadMore"
    @confirm="handlePathConfirm"
  />
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import CircleCheckFilled from '~icons/ep/circle-check-filled'
import CircleCloseFilled from '~icons/ep/circle-close-filled'
import Edit from '~icons/ep/edit'
import FolderOpened from '~icons/ep/folder-opened'
import Link from '~icons/ep/link'
import Loading from '~icons/ep/loading'
import Plus from '~icons/ep/plus'
import { createServer, listDirectories, testServer, testServerTemp, updateServer } from '@/api/servers'
import { usePathDialog, normalizePath } from '@/composables/usePathDialog'
import PathDialog from '@/components/common/PathDialog.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  mode: { type: String, default: 'data' },
  editingServer: { type: Object, default: null },
  dataTypeDefs: { type: Array, default: () => [] },
  mediaTypeOptions: { type: Array, default: () => [] },
  serverList: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:modelValue', 'saved'])

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const isEdit = computed(() => !!props.editingServer)
const isDataMode = computed(() => props.mode === 'data')

const formRef = ref(null)
const formData = reactive({
  id: null,
  name: '',
  type: '',
  host: '',
  port: 80,
  api_key: '',
  options: '{}',
  enabled: true,
  // 高级配置（默认值，0 表示使用系统设置）
  download_rate_per_sec: 0,
  api_rate: 0,
  api_retry_max: 0,
  api_retry_interval_sec: 0
})

// 动态表单字段模型
const dynamicModel = reactive({})
// 自动命名状态（新建时启用，用户手动输入后禁用）
const autoNameActive = ref(true)
// 测试连接状态（idle/running/success/error）
const testStatus = ref('idle')
const saving = ref(false)
const testing = ref(false)

// 合并的表单模型（用于 el-form 验证）
// 数据服务器：包含基础字段（formData）+ 动态字段（dynamicModel）
// 媒体服务器：仅使用 formData
const formModel = computed(() => {
  if (props.mode === 'data') {
    return { ...formData, ...dynamicModel }
  }
  return formData
})

// 数据服务器类型选项（用于下拉选择）
const dataServerTypeOptions = computed(() =>
  props.dataTypeDefs.map((def) => ({
    label: def.label,
    value: def.type,
    description: def.description || ''
  }))
)

const serverTypeOptions = computed(() =>
  props.mode === 'data' ? dataServerTypeOptions.value : props.mediaTypeOptions
)

const dialogTitle = computed(() => (isEdit.value ? '编辑服务器' : '新增服务器'))

const buildRemoteRootField = () => ({
  name: 'remote_root',
  type: 'path',
  label: '远程根目录',
  placeholder: '/',
  help: '远程 API 的根路径（用于获取文件列表/信息）',
  required: false,
  default: '/'
})

const ensureRemoteRootField = (typeDef) => {
  if (!typeDef) return typeDef
  const type = String(typeDef.type || '').toLowerCase()
  if (type !== 'clouddrive2' && type !== 'openlist') return typeDef
  const sections = Array.isArray(typeDef.sections) ? typeDef.sections : []
  let hasField = false
  for (const section of sections) {
    for (const field of section.fields || []) {
      if (field?.name === 'remote_root') {
        hasField = true
        break
      }
    }
    if (hasField) break
  }
  if (hasField) return typeDef

  const nextSections = sections.map((section) => {
    if (section.id !== 'paths') return section
    return {
      ...section,
      fields: [...(section.fields || []), buildRemoteRootField()]
    }
  })
  const hasPaths = sections.some((section) => section.id === 'paths')
  if (!hasPaths) {
    nextSections.push({
      id: 'paths',
      label: '路径配置',
      fields: [buildRemoteRootField()]
    })
  }

  return {
    ...typeDef,
    sections: nextSections,
    storage: {
      ...(typeDef.storage || {}),
      remote_root: 'options'
    }
  }
}

// 当前选择的数据服务器类型定义（用于动态表单）
const activeTypeDef = computed(() => {
  if (props.mode !== 'data' || !formData.type) return null
  const def = props.dataTypeDefs.find((item) => item.type === formData.type) || null
  return ensureRemoteRootField(def)
})

// 当前选择的服务器类型配置（媒体服务器使用）
const currentTypeConfig = computed(() => {
  if (props.mode !== 'media') return null
  if (!formData.type) return null
  return serverTypeOptions.value.find((opt) => opt.value === formData.type)
})

// 服务器类型提示
const serverTypeHint = computed(() => {
  if (props.mode === 'data') {
    return activeTypeDef.value?.description || ''
  }
  return currentTypeConfig.value?.hint || ''
})

// 主机地址占位符（媒体服务器使用）
const hostPlaceholder = computed(() => '127.0.0.1 或 example.com')

// 是否需要 API 密钥（媒体服务器使用）
const needsApiKey = computed(() => currentTypeConfig.value?.needsApiKey || false)

// API 密钥标签（媒体服务器使用）
const apiKeyLabel = computed(() => currentTypeConfig.value?.apiKeyLabel || 'API Key')

// 是否可以测试连接（已选择类型）
const canTest = computed(() => {
  if (!formData.type) return false
  return true
})

const showTestButton = computed(() => !!formData.type && formData.type !== 'local')

// 是否显示接口速率配置（仅数据服务器的 cd2 和 openlist 类型）
const showRate = computed(() =>
  props.mode === 'data' && ['clouddrive2', 'openlist'].includes(formData.type)
)

// 测试状态文本
const testStatusText = computed(() => {
  switch (testStatus.value) {
    case 'running':
      return '测试中'
    case 'success':
      return '连接成功'
    case 'error':
      return '连接失败'
    default:
      return ''
  }
})

// 基础表单验证规则（所有类型通用）
const baseFormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }]
}

// 媒体服务器额外验证规则
const mediaFormRules = {
  host: [{ required: true, message: '请输入主机', trigger: 'blur' }],
  port: [
    { required: true, message: '请输入端口', trigger: 'change' },
    {
      validator: (_, value, callback) => {
        if (value < 1 || value > 65535) {
          callback(new Error('端口范围为 1-65535'))
          return
        }
        callback()
      },
      trigger: 'change'
    }
  ],
  options: [
    {
      validator: (_, value, callback) => {
        if (!value) {
          callback()
          return
        }
        try {
          JSON.parse(value)
          callback()
        } catch (error) {
          callback(new Error('Options 必须是合法 JSON'))
        }
      },
      trigger: 'blur'
    }
  ]
}

// 动态表单验证规则（根据活动的 typeDef 生成）
const formRules = computed(() => {
  if (props.mode !== 'data') {
    const rules = { ...baseFormRules, ...mediaFormRules }
    // 媒体服务器：API Key 动态必填验证
    if (needsApiKey.value) {
      rules.api_key = [{ required: true, message: `请输入${apiKeyLabel.value}`, trigger: 'blur' }]
    }
    return rules
  }
  const dynamic = buildDynamicRules(activeTypeDef.value)
  return { ...baseFormRules, ...dynamic }
})

// 重置动态模型
const resetDynamicModel = () => {
  for (const key of Object.keys(dynamicModel)) {
    delete dynamicModel[key]
  }
}

const resetForm = () => {
  formData.id = null
  formData.name = ''
  formData.type = ''
  formData.host = ''
  formData.port = 80
  formData.api_key = ''
  formData.options = '{}'
  formData.enabled = true
  // 高级配置默认值
  formData.download_rate_per_sec = 0
  formData.api_rate = 0
  formData.api_retry_max = 0
  formData.api_retry_interval_sec = 0
  autoNameActive.value = true
  testStatus.value = 'idle'
  resetDynamicModel()
}

// 获取服务器类型的显示名称
const getTypeLabel = () => {
  if (props.mode === 'data') {
    return activeTypeDef.value?.label || formData.type
  }
  return currentTypeConfig.value?.label || formData.type
}

// 生成默认服务器名称（类型名 + 递增数字）
const generateDefaultName = () => {
  const raw = getTypeLabel()
  if (!raw) return ''

  // 移除空格，作为基础名称
  const base = raw.replace(/\s+/g, '')

  // 查找当前服务器列表中相同类型的最大编号
  const pattern = new RegExp(`^${base}(\\d+)$`, 'i')
  let max = 0

  for (const server of props.serverList) {
    if (server.type !== formData.type) continue
    const name = String(server.name || '').replace(/\s+/g, '')
    const match = name.match(pattern)
    if (match) {
      const num = Number(match[1])
      if (!Number.isNaN(num)) max = Math.max(max, num)
    }
  }

  return `${base}${max + 1}`
}

// 应用默认名称（仅在新建且未手动输入时）
const applyDefaultName = () => {
  if (isEdit.value || !formData.type) return
  if (formData.name && !autoNameActive.value) return

  const nextName = generateDefaultName()
  if (nextName) {
    formData.name = nextName
    autoNameActive.value = true
  }
}

// 用户手动输入名称时禁用自动命名
const handleNameInput = () => {
  if (!isEdit.value) autoNameActive.value = false
}

// 展平所有字段
const flattenFields = (typeDef) => {
  if (!typeDef?.sections) return []
  return typeDef.sections.flatMap((section) => section.fields || [])
}

// 应用类型默认值
const applyTypeDefaults = (typeDef) => {
  resetDynamicModel()
  if (!typeDef) return
  for (const field of flattenFields(typeDef)) {
    if (field.default !== undefined) {
      dynamicModel[field.name] = field.default
      continue
    }
    if (field.type === 'number') {
      dynamicModel[field.name] = field.min ?? 0
      continue
    }
    dynamicModel[field.name] = ''
  }
}

// 规范化 options 值
const normalizeOptions = (raw) => {
  if (!raw) return {}
  if (typeof raw === 'object') return raw
  try {
    return JSON.parse(raw)
  } catch (error) {
    return {}
  }
}

// 从 row 数据回填动态模型
const hydrateDynamicModel = (row) => {
  const typeDef = activeTypeDef.value
  if (!typeDef) return
  applyTypeDefaults(typeDef)
  const options = normalizeOptions(row.options)
  const storage = typeDef.storage || {}
  for (const field of flattenFields(typeDef)) {
    const target = storage[field.name] || 'options'
    if (target === 'root' && row[field.name] !== undefined) {
      dynamicModel[field.name] = row[field.name]
      continue
    }
    if (target === 'api_key' && row.api_key !== undefined) {
      dynamicModel[field.name] = row.api_key
      continue
    }
    if (target === 'options' && options[field.name] !== undefined) {
      dynamicModel[field.name] = options[field.name]
    }
  }
}

// 判断字段是否可见
const isFieldVisible = (field) => {
  if (!field) return false
  if (field.type === 'hidden') return false
  if (!field.visible_if) return true
  return Object.entries(field.visible_if).every(
    ([key, expected]) => String(dynamicModel[key]) === String(expected)
  )
}

// 获取可见字段列表
const getVisibleFields = (fields) => (fields || []).filter((field) => isFieldVisible(field))

// 判断是否为文本字段
const isTextField = (field) => field.type === 'text'

// 判断是否为路径字段
const isPathField = (field) => field?.type === 'path'

// 构建动态表单的 payload
// 注意：只提交当前可见的字段，防止切换模式后提交隐藏字段的旧值
const buildPayload = () => {
  const payload = {
    name: formData.name,
    type: formData.type,
    enabled: formData.enabled,
    // 高级配置字段
    download_rate_per_sec: formData.download_rate_per_sec,
    api_rate: formData.api_rate,
    api_retry_max: formData.api_retry_max,
    api_retry_interval_sec: formData.api_retry_interval_sec
  }

  const typeDef = activeTypeDef.value
  if (!typeDef) {
    return payload
  }

  const options = {}
  const storage = typeDef.storage || {}
  for (const field of flattenFields(typeDef)) {
    // 跳过不可见的字段（防止切换模式后提交隐藏字段旧值）
    if (!isFieldVisible(field)) {
      continue
    }

    const value = dynamicModel[field.name]
    const target = storage[field.name] || 'options'
    if (value === undefined || value === '') {
      continue
    }
    if (target === 'root') {
      payload[field.name] = value
      continue
    }
    if (target === 'api_key') {
      payload.api_key = value
      continue
    }
    options[field.name] = value
  }

  payload.options = JSON.stringify(options)
  return payload
}

const buildMediaPayload = () => ({
  name: formData.name,
  type: formData.type,
  host: formData.host,
  port: formData.port,
  api_key: formData.api_key,
  options: formData.options,
  enabled: formData.enabled,
  // 高级配置字段
  download_rate_per_sec: formData.download_rate_per_sec,
  api_rate: formData.api_rate,
  api_retry_max: formData.api_retry_max,
  api_retry_interval_sec: formData.api_retry_interval_sec
})

// 构建动态验证规则
const buildDynamicRules = (typeDef) => {
  if (!typeDef) return {}
  const rules = {}
  for (const field of flattenFields(typeDef)) {
    if (!field.required) continue
    rules[field.name] = [
      {
        validator: (_, value, callback) => {
          if (!isFieldVisible(field)) {
            callback()
            return
          }
          if (value === undefined || value === null || value === '') {
            callback(new Error(`请输入${field.label || field.name}`))
            return
          }
          callback()
        },
        trigger: field.type === 'select' ? 'change' : 'blur'
      }
    ]
  }
  return rules
}

// 服务器类型变化时自动更新默认端口或应用默认值
const handleTypeChange = () => {
  if (props.mode === 'data') {
    // 数据服务器：应用类型默认值
    applyTypeDefaults(activeTypeDef.value)
    // 新建模式下更新默认名称
    if (!isEdit.value) {
      applyDefaultName()
    }
  } else {
    // 媒体服务器：更新默认端口
    const config = currentTypeConfig.value
    if (config && config.defaultPort > 0) {
      formData.port = config.defaultPort
    }
    // 新建模式下更新默认名称
    if (!isEdit.value) {
      applyDefaultName()
    }
  }
}

const prepareCreate = () => {
  resetForm()
  // 数据服务器：自动选择第一个类型并应用默认值
  if (props.mode === 'data' && props.dataTypeDefs.length > 0) {
    formData.type = props.dataTypeDefs[0].type
    applyTypeDefaults(activeTypeDef.value)
    applyDefaultName()
  }
  // 媒体服务器：默认选择 emby 类型并应用默认值
  else if (props.mode === 'media' && props.mediaTypeOptions.length > 0) {
    const defaultType = props.mediaTypeOptions.find(opt => opt.value === 'emby') || props.mediaTypeOptions[0]
    if (defaultType) {
      formData.type = defaultType.value
      if (defaultType.defaultPort > 0) {
        formData.port = defaultType.defaultPort
      }
    }
    applyDefaultName()
  }
}

const prepareEdit = (row) => {
  if (!row) return
  resetForm()
  testStatus.value = 'idle'
  formData.id = row.id
  formData.name = row.name
  formData.type = row.type
  formData.host = row.host
  formData.port = row.port
  formData.api_key = row.api_key || ''
  formData.options =
    typeof row.options === 'object'
      ? JSON.stringify(row.options, null, 2)
      : row.options || '{}'
  formData.enabled = row.enabled !== false
  // 高级配置字段加载（使用 ?? 提供默认值）
  formData.download_rate_per_sec = row.download_rate_per_sec ?? 0
  formData.api_rate = row.api_rate ?? 0
  formData.api_retry_max = row.api_retry_max ?? 0
  formData.api_retry_interval_sec = row.api_retry_interval_sec ?? 0
  // 数据服务器：确保类型定义已加载，然后回填动态字段
  if (props.mode === 'data' && props.dataTypeDefs.length > 0) {
    hydrateDynamicModel(row)
  }
}

// 收集测试连接时需要验证的字段列表
const collectTestValidationFields = () => {
  const fields = new Set()
  if (formData.type) fields.add('type')

  // 媒体服务器：验证基础字段
  if (props.mode === 'media') {
    fields.add('host')
    fields.add('port')
    if (needsApiKey.value) fields.add('api_key')
    return Array.from(fields)
  }

  // 数据服务器：验证动态字段
  const typeDef = activeTypeDef.value
  if (!typeDef) return Array.from(fields)
  for (const field of flattenFields(typeDef)) {
    if (!field.required) continue
    if (!isFieldVisible(field)) continue
    fields.add(field.name)
  }
  return Array.from(fields)
}

// 测试连接前的表单验证
const validateBeforeTest = async () => {
  if (!formRef.value) return true
  const fields = collectTestValidationFields()
  if (fields.length === 0) return true
  try {
    await formRef.value.validateField(fields)
    return true
  } catch (error) {
    return false
  }
}

// 测试连接
const handleTestConnection = async () => {
  // 测试前先验证必填字段
  const isValid = await validateBeforeTest()
  if (!isValid) {
    testStatus.value = 'idle'
    return
  }
  testing.value = true
  testStatus.value = 'running'
  try {
    if (formData.id) {
      // 已保存的服务器：使用原有接口
      await testServer(formData.id, formData.type)
    } else {
      // 未保存的服务器：使用临时测试接口
      const payload = props.mode === 'data'
        ? buildPayload()
        : buildMediaPayload()
      await testServerTemp(payload)
    }
    ElMessage.success('连接测试成功！')
    testStatus.value = 'success'
  } catch (error) {
    // 错误已由拦截器处理
    testStatus.value = 'error'
  } finally {
    testing.value = false
  }
}

const handleSave = async () => {
  try {
    await formRef.value?.validate()
    saving.value = true
    const payload = props.mode === 'data' ? buildPayload() : buildMediaPayload()
    if (isEdit.value) {
      await updateServer(formData.id, payload)
      ElMessage.success('服务器已更新')
    } else {
      await createServer(payload)
      ElMessage.success('服务器已创建')
    }
    visible.value = false
    emit('saved')
  } catch (error) {
    if (error?.message) {
      console.error('保存服务器失败:', error)
    }
  } finally {
    saving.value = false
  }
}

const pathDlg = usePathDialog({
  loader: (payload) => listDirectories({
    path: payload?.path,
    limit: payload?.limit,
    offset: payload?.offset,
    mode: 'local'
  }),
  onError: () => ElMessage.error('加载目录失败')
})
const pathDialogField = ref(null)

const openPathDialog = async (field) => {
  if (!field) return
  pathDialogField.value = field
  const currentValue = dynamicModel[field.name] || '/'
  await pathDlg.open({ mode: 'single', root: '/', path: currentValue })
  const normalized = normalizePath(currentValue)
  if (normalized !== '/') {
    pathDlg.selectedName.value = normalized
  } else {
    pathDlg.clearSelection()
  }
}

const handlePathSelect = (name) => {
  pathDlg.selectRow(name)
}

const handlePathConfirm = () => {
  if (!pathDialogField.value) return
  const selectedPath = pathDlg.getSelectedSingle()
  dynamicModel[pathDialogField.value.name] = selectedPath
  pathDlg.close()
}

// 监听弹窗开启
watch(() => props.modelValue, (val) => {
  if (!val) return
  if (props.editingServer) {
    prepareEdit(props.editingServer)
  } else {
    prepareCreate()
  }
})

// 监听类型定义延迟加载（编辑模式）
watch(() => props.dataTypeDefs, (defs) => {
  if (!visible.value || !props.editingServer || props.mode !== 'data') return
  if (defs.length > 0) {
    hydrateDynamicModel(props.editingServer)
  }
})
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

  .option-desc {
    float: right;
    color: var(--el-text-color-secondary);
    font-size: 13px;
    margin-left: 15px;
  }

  .section-row {
    align-items: flex-start;
  }

  .base-info-row {
    --base-label-height: 22px;
    align-items: flex-start;
  }

  .label-inline {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
  }

  .label-help {
    margin-left: auto;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    line-height: 1.4;
    max-width: 320px;
    white-space: normal;
    text-align: right;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .test-inline {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-wrap: wrap;
  }

  .type-inline {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    flex-wrap: nowrap;
  }

  .type-select {
    flex: 1;
    min-width: 0;
  }

  .type-switch {
    flex-shrink: 0;
  }

  .label-switch {
    margin-left: auto;
  }

  .base-info-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .section-actions {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .base-status-row {
    margin-top: 8px;
  }

  // 动态表单横向布局
  .form-row {
    width: 100%;
  }

  // 对话框底部按钮组
  .dialog-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;

    .dialog-footer-left {
      flex: 0 0 auto;
    }

    .dialog-footer-actions {
      flex: 1;
      display: flex;
      justify-content: flex-end;
      align-items: center;
      gap: 12px;
    }
  }

  // 测试状态显示
  .test-status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 14px;
    padding: 0 8px;

    .el-icon {
      font-size: 16px;
    }

    &.test-status--running {
      color: var(--el-color-primary);
    }

    &.test-status--success {
      color: var(--el-color-success);
    }

    &.test-status--error {
      color: var(--el-color-danger);
    }
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
    flex-wrap: nowrap;
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

  :deep(.no-label .el-form-item__label) {
    display: none;
  }

  :deep(.base-info-row .el-form-item__label) {
    height: var(--base-label-height);
    line-height: var(--base-label-height);
  }

  :deep(.base-info-row .el-form-item__content) {
    min-height: 32px;
    align-items: center;
  }
}

// 路径选择对话框
:deep(.path-dialog) {
  .path-toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 0;

    .path-current {
      flex: 1;
      margin-left: 12px;
      display: flex;
      align-items: center;
      font-size: 13px;
      background: var(--el-fill-color-light);
      padding: 6px 12px;
      border-radius: 4px;

      .path-label {
        color: var(--el-text-color-secondary);
        margin-right: 6px;
      }

      .path-value {
        color: var(--el-text-color-primary);
        font-family: 'Courier New', monospace;
      }
    }
  }

  .dir-name {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;

    .el-icon {
      color: var(--el-color-warning);
      font-size: 18px;
    }

    &.is-selected {
      font-weight: 600;
      color: var(--el-color-primary);

      .el-icon {
        color: var(--el-color-primary);
      }
    }
  }

  .empty-hint {
    text-align: center;
    color: var(--el-text-color-secondary);
    padding: 32px 0;
    font-size: 14px;
  }
}

// 分割线样式优化
:deep(.el-divider__text) {
  font-weight: 500;
  color: var(--el-text-color-primary);
}
</style>
