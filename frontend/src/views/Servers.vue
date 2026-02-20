<template>
  <div class="servers-page">
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
        style="width: 320px"
        @keyup.enter="handleSearch"
      />
      <div style="flex: 1"></div>
      <el-button type="primary" :icon="Plus" @click="handleAdd">
        新增服务器
      </el-button>
    </div>

    <el-table v-loading="loading" :data="serverList" stripe style="width: 100%">
      <el-table-column prop="name" label="名称" min-width="140" />
      <el-table-column prop="type" label="类型" width="140">
        <template #default="{ row }">
          <el-tag size="small">{{ row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="host" label="主机" min-width="160" />
      <el-table-column prop="port" label="端口" width="90" />
      <el-table-column prop="api_key" label="API密钥" min-width="160">
        <template #default="{ row }">
          <span>{{ maskApiKey(row.api_key) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="启用" width="90">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" size="small" disabled />
        </template>
      </el-table-column>
      <el-table-column prop="uid" label="UID" min-width="160" show-overflow-tooltip />
      <el-table-column prop="created_at" label="创建时间" width="150">
        <template #default="{ row }">
          {{ row.created_at ? formatTime(row.created_at) : '-' }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-tooltip content="测试连接" placement="top">
            <el-button
              size="small"
              :icon="Link"
              :disabled="row.type === 'local'"
              @click="handleTest(row)"
            />
          </el-tooltip>
          <el-tooltip content="编辑服务器" placement="top">
            <el-button size="small" :icon="Edit" @click="handleEdit(row)" />
          </el-tooltip>
          <el-tooltip content="删除服务器" placement="top">
            <el-button size="small" type="danger" :icon="Delete" @click="handleDelete(row)" />
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="pagination.total"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="680px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <el-form
        ref="formRef"
        :model="formModel"
        :rules="formRules"
        label-width="120px"
      >
        <!-- 基础信息 -->
        <el-divider content-position="left">基础信息</el-divider>

        <el-form-item label="服务器名称" prop="name">
          <el-input
            v-model="formData.name"
            placeholder="例如：Emby"
            clearable
            @input="handleNameInput"
          />
        </el-form-item>

        <el-form-item label="服务器类型" prop="type">
          <el-select
            v-model="formData.type"
            filterable
            placeholder="选择服务器类型"
            style="width: 100%"
            @change="handleTypeChange"
          >
            <el-option
              v-for="option in serverTypeOptions"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            >
              <span>{{ option.label }}</span>
              <span style="float: right; color: var(--el-text-color-secondary); font-size: 13px; margin-left: 15px">
                {{ option.description }}
              </span>
            </el-option>
          </el-select>
          <div v-if="serverTypeHint" class="type-hint">
            <el-icon><InfoFilled /></el-icon>
            {{ serverTypeHint }}
          </div>
        </el-form-item>

        <!-- 数据服务器：动态表单 -->
        <template v-if="activeTab === 'data'">
          <template v-if="activeTypeDef">
            <template v-for="section in activeTypeDef.sections" :key="section.id">
              <el-divider content-position="left">{{ section.label }}</el-divider>

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
                    >
                      <!-- 路径字段 -->
                      <el-input
                        v-if="isPathField(field)"
                        v-model="dynamicModel[field.name]"
                        :placeholder="field.placeholder"
                        clearable
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
                      <el-input-number
                        v-else-if="field.type === 'number'"
                        v-model="dynamicModel[field.name]"
                        :min="field.min ?? 1"
                        :max="field.max ?? 65535"
                        style="width: 100%"
                      />
                      <!-- 下拉选择 -->
                      <el-select
                        v-else-if="field.type === 'select'"
                        v-model="dynamicModel[field.name]"
                        placeholder="请选择"
                        style="width: 100%"
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
                      <div v-if="field.help" class="field-hint">{{ field.help }}</div>
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
                >
                  <!-- 路径字段 -->
                  <el-input
                    v-if="isPathField(field)"
                    v-model="dynamicModel[field.name]"
                    :placeholder="field.placeholder"
                    clearable
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
                  <el-input-number
                    v-else-if="field.type === 'number'"
                    v-model="dynamicModel[field.name]"
                    :min="field.min ?? 1"
                    :max="field.max ?? 65535"
                    style="width: 100%"
                  />
                  <!-- 下拉选择 -->
                  <el-select
                    v-else-if="field.type === 'select'"
                    v-model="dynamicModel[field.name]"
                    placeholder="请选择"
                    style="width: 100%"
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
                  <div v-if="field.help" class="field-hint">{{ field.help }}</div>
                </el-form-item>
              </template>
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
          <el-divider content-position="left">连接信息</el-divider>

          <div class="form-row">
            <el-row :gutter="12">
              <el-col :span="14">
                <el-form-item label="主机地址" prop="host">
                  <el-input
                    v-model="formData.host"
                    :placeholder="hostPlaceholder"
                    clearable
                  />
                </el-form-item>
              </el-col>
              <el-col :span="10">
                <el-form-item label="端口号" prop="port">
                  <el-input-number
                    v-model="formData.port"
                    :min="1"
                    :max="65535"
                    :step="1"
                    style="width: 100%"
                  />
                  <div class="field-hint">{{ portHint }}</div>
                </el-form-item>
              </el-col>
            </el-row>
          </div>

          <!-- API 密钥（如需要） -->
          <el-form-item v-if="needsApiKey" :label="apiKeyLabel" prop="api_key">
            <el-input
              v-model="formData.api_key"
              type="password"
              show-password
              :placeholder="`请输入${apiKeyLabel}`"
              clearable
            />
          </el-form-item>
        </template>

        <!-- 启用状态（所有类型通用） -->
        <el-form-item label="启用状态" prop="enabled">
          <el-switch
            v-model="formData.enabled"
            active-text="启用"
            inactive-text="禁用"
          />
        </el-form-item>

        <!-- 高级选项（QoS配置） -->
        <el-divider v-if="showQoS" content-position="left">高级选项</el-divider>
        <el-collapse v-if="showQoS">
          <el-collapse-item title="QoS 配置（可选）" name="qos">
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="请求超时(毫秒)">
                  <el-input-number v-model="formData.request_timeout_ms" :min="1000" :max="120000" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="连接超时(毫秒)">
                  <el-input-number v-model="formData.connect_timeout_ms" :min="500" :max="60000" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="重试次数">
                  <el-input-number v-model="formData.retry_max" :min="0" :max="10" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="退避时间(毫秒)">
                  <el-input-number v-model="formData.retry_backoff_ms" :min="0" :max="10000" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="最大并发">
                  <el-input-number v-model="formData.max_concurrent" :min="1" :max="1000" />
                </el-form-item>
              </el-col>
            </el-row>
          </el-collapse-item>
        </el-collapse>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <div class="dialog-footer-left">
            <el-button @click="dialogVisible = false">取消</el-button>
          </div>
          <div class="dialog-footer-actions">
            <!-- 测试状态图标 -->
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
            <!-- 测试连接按钮 -->
            <el-tooltip
              v-if="canTest"
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
                  测试连接
                </el-button>
              </span>
            </el-tooltip>
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

    <!-- 目录选择对话框 -->
    <el-dialog
      v-model="pathDialogVisible"
      title="选择目录"
      width="600px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <div v-loading="pathDialogLoading" class="path-dialog">
        <!-- 工具栏 -->
        <div class="path-toolbar">
          <el-button
            :icon="ArrowLeft"
            :disabled="pathDialogPath === '/' || pathDialogPath === ''"
            @click="handlePathUp"
          >
            返回上级
          </el-button>
          <el-button
            :icon="HomeFilled"
            :disabled="pathDialogPath === '/'"
            @click="loadDirectories('/')"
          >
            根目录
          </el-button>
          <div class="path-current">
            <span class="path-label">当前路径：</span>
            <span class="path-value">{{ pathDialogPath || '/' }}</span>
          </div>
        </div>

        <!-- 目录列表 -->
        <el-table
          :data="pathDialogRows"
          stripe
          highlight-current-row
          style="width: 100%; margin-top: 12px"
          max-height="400px"
          @row-click="selectDirectory"
          @row-dblclick="(row) => enterDirectory(row.name)"
        >
          <el-table-column label="目录名称" prop="name">
            <template #default="{ row }">
              <div class="dir-name" :class="{ 'is-selected': row.name === pathDialogSelectedDir }">
                <el-icon><FolderOpened /></el-icon>
                <span>{{ row.name }}</span>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <div v-if="pathDialogRows.length === 0" class="empty-hint">
          当前目录下没有子目录
        </div>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="pathDialogVisible = false">取消</el-button>
          <div style="flex: 1"></div>
          <el-button
            v-if="pathDialogSelectedDir"
            :icon="FolderOpened"
            @click="enterSelectedDirectory"
          >
            进入"{{ pathDialogSelectedDir }}"
          </el-button>
          <el-button type="primary" @click="selectCurrentPath">选择当前目录</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowLeft,
  CircleCheckFilled,
  CircleCloseFilled,
  Delete,
  Edit,
  FolderOpened,
  HomeFilled,
  InfoFilled,
  Link,
  Loading,
  Plus,
  Search
} from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'
import {
  createServer,
  deleteServer,
  getServerList,
  getServerTypes,
  listDirectories,
  testServer,
  testServerTemp,
  updateServer
} from '@/api/servers'
import { normalizeListResponse } from '@/api/normalize'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

const activeTab = ref('data')
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const serverList = ref([])

const filters = reactive({
  keyword: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

const dialogVisible = ref(false)
const isEdit = ref(false)
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
  // QoS 配置（默认值）
  request_timeout_ms: 30000,
  connect_timeout_ms: 10000,
  retry_max: 3,
  retry_backoff_ms: 1000,
  max_concurrent: 10
})

// 数据服务器类型定义（从后端加载）
const dataServerTypeDefs = ref([])
// 动态表单字段模型
const dynamicModel = reactive({})
// 自动命名状态（新建时启用，用户手动输入后禁用）
const autoNameActive = ref(true)

// 测试连接状态（idle/running/success/error）
const testStatus = ref('idle')

// 目录选择对话框状态
const pathDialogVisible = ref(false)
const pathDialogLoading = ref(false)
const pathDialogPath = ref('/')
const pathDialogDirs = ref([])
const pathDialogField = ref(null)
const pathDialogSelectedDir = ref('')
const pathDialogContext = reactive({
  mode: 'local',
  type: '',
  host: '',
  port: '',
  apiKey: ''
})

// 合并的表单模型（用于 el-form 验证）
// 数据服务器：包含基础字段（formData）+ 动态字段（dynamicModel）
// 媒体服务器：仅使用 formData
const formModel = computed(() => {
  if (activeTab.value === 'data') {
    return { ...formData, ...dynamicModel }
  }
  return formData
})

// 数据服务器类型选项（用于下拉选择）
const dataServerTypeOptions = computed(() =>
  dataServerTypeDefs.value.map((def) => ({
    label: def.label,
    value: def.type,
    description: def.description || ''
  }))
)

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

const serverTypeOptions = computed(() =>
  activeTab.value === 'data' ? dataServerTypeOptions.value : mediaServerTypeOptions
)

const dialogTitle = computed(() => (isEdit.value ? '编辑服务器' : '新增服务器'))

// 当前选择的数据服务器类型定义（用于动态表单）
const activeTypeDef = computed(() => {
  if (activeTab.value !== 'data' || !formData.type) return null
  return dataServerTypeDefs.value.find((def) => def.type === formData.type) || null
})

// 当前选择的服务器类型配置（媒体服务器使用）
const currentTypeConfig = computed(() => {
  if (activeTab.value !== 'media') return null
  if (!formData.type) return null
  return serverTypeOptions.value.find((opt) => opt.value === formData.type)
})

// 服务器类型提示
const serverTypeHint = computed(() => {
  if (activeTab.value === 'data') {
    return activeTypeDef.value?.description || ''
  }
  return currentTypeConfig.value?.hint || ''
})

// 主机地址占位符（媒体服务器使用）
const hostPlaceholder = computed(() => {
  return '例如：127.0.0.1 或 example.com'
})

// 端口号提示（媒体服务器使用）
const portHint = computed(() => {
  const config = currentTypeConfig.value
  if (!config) return ''
  return `${config.label} 默认端口：${config.defaultPort}`
})

// 是否需要 API 密钥（媒体服务器使用）
const needsApiKey = computed(() => currentTypeConfig.value?.needsApiKey || false)

// API 密钥标签（媒体服务器使用）
const apiKeyLabel = computed(() => currentTypeConfig.value?.apiKeyLabel || 'API Key')

// 是否可以测试连接（已选择类型且非 Local 类型）
const canTest = computed(() => {
  if (!formData.type) return false
  return formData.type !== 'local'
})

// 是否显示 QoS 配置（仅数据服务器的 cd2 和 openlist 类型）
const showQoS = computed(() =>
  activeTab.value === 'data' && ['clouddrive2', 'openlist'].includes(formData.type)
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

// 目录列表行数据
const pathDialogRows = computed(() => {
  return pathDialogDirs.value.map(name => ({ name }))
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
  if (activeTab.value !== 'data') {
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

// 获取服务器类型的显示名称
const getTypeLabel = () => {
  if (activeTab.value === 'data') {
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

  for (const server of serverList.value) {
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


const formatTime = (time) => {
  return dayjs(time).fromNow()
}

const maskApiKey = (value) => {
  if (!value) return '-'
  const text = String(value)
  if (text.length <= 8) return `${text.slice(0, 1)}***${text.slice(-1)}`
  return `${text.slice(0, 4)}****${text.slice(-4)}`
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
  } catch (error) {
    console.error('加载服务器列表失败:', error)
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  pagination.page = 1
  loadServers()
}

const handleTabChange = () => {
  pagination.page = 1
  filters.keyword = ''
  loadServers()
}

const handlePageChange = () => {
  loadServers()
}

const handleSizeChange = () => {
  pagination.page = 1
  loadServers()
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
  // QoS 默认值
  formData.request_timeout_ms = 30000
  formData.connect_timeout_ms = 10000
  formData.retry_max = 3
  formData.retry_backoff_ms = 1000
  formData.max_concurrent = 10
  autoNameActive.value = true
  testStatus.value = 'idle'
  resetDynamicModel()
}

const handleAdd = () => {
  isEdit.value = false
  resetForm()
  // 数据服务器：自动选择第一个类型并应用默认值
  if (activeTab.value === 'data' && dataServerTypeDefs.value.length > 0) {
    formData.type = dataServerTypeDefs.value[0].type
    applyTypeDefaults(activeTypeDef.value)
    applyDefaultName()
  }
  // 媒体服务器：默认选择 emby 类型并应用默认值
  else if (activeTab.value === 'media' && mediaServerTypeOptions.length > 0) {
    formData.type = 'emby'
    const config = mediaServerTypeOptions.find(opt => opt.value === 'emby')
    if (config && config.defaultPort > 0) {
      formData.port = config.defaultPort
    }
    applyDefaultName()
  }
  dialogVisible.value = true
}

const handleEdit = async (row) => {
  isEdit.value = true
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
  // QoS 字段加载（使用 ?? 提供默认值）
  formData.request_timeout_ms = row.request_timeout_ms ?? 30000
  formData.connect_timeout_ms = row.connect_timeout_ms ?? 10000
  formData.retry_max = row.retry_max ?? 3
  formData.retry_backoff_ms = row.retry_backoff_ms ?? 1000
  formData.max_concurrent = row.max_concurrent ?? 10
  // 数据服务器：确保类型定义已加载，然后回填动态字段
  if (activeTab.value === 'data') {
    await loadDataServerTypes()
    hydrateDynamicModel(row)
  }
  dialogVisible.value = true
}

// 服务器类型变化时自动更新默认端口或应用默认值
const handleTypeChange = () => {
  if (activeTab.value === 'data') {
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

// 收集测试连接时需要验证的字段列表
const collectTestValidationFields = () => {
  const fields = new Set()
  if (formData.type) fields.add('type')

  // 媒体服务器：验证基础字段
  if (activeTab.value === 'media') {
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
      const payload = activeTab.value === 'data'
        ? buildPayload()
        : {
          name: formData.name,
          type: formData.type,
          host: formData.host,
          port: formData.port,
          api_key: formData.api_key,
          options: formData.options,
          enabled: formData.enabled,
          request_timeout_ms: formData.request_timeout_ms,
          connect_timeout_ms: formData.connect_timeout_ms,
          retry_max: formData.retry_max,
          retry_backoff_ms: formData.retry_backoff_ms,
          max_concurrent: formData.max_concurrent
        }
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

    // 根据 tab 类型构建不同的 payload
    const payload = activeTab.value === 'data' ? buildPayload() : {
      name: formData.name,
      type: formData.type,
      host: formData.host,
      port: formData.port,
      api_key: formData.api_key,
      options: formData.options,
      enabled: formData.enabled,
      // QoS 字段
      request_timeout_ms: formData.request_timeout_ms,
      connect_timeout_ms: formData.connect_timeout_ms,
      retry_max: formData.retry_max,
      retry_backoff_ms: formData.retry_backoff_ms,
      max_concurrent: formData.max_concurrent
    }

    if (isEdit.value) {
      await updateServer(formData.id, payload)
      ElMessage.success('服务器已更新')
    } else {
      await createServer(payload)
      ElMessage.success('服务器已创建')
      // 创建成功后重置到第一页，确保新记录可见
      pagination.page = 1
    }

    dialogVisible.value = false
    loadServers()
  } catch (error) {
    if (error?.message) {
      console.error('保存服务器失败:', error)
    }
  } finally {
    saving.value = false
  }
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
    ElMessage.success('连接测试成功')
  } catch (error) {
    loadingMsg.close()
    // 错误已由拦截器处理
  }
}

// ============ 动态表单辅助函数 ============

// 加载数据服务器类型定义
const loadDataServerTypes = async () => {
  if (dataServerTypeDefs.value.length > 0) return
  try {
    const response = await getServerTypes({ category: 'data' })
    dataServerTypeDefs.value = response?.types || []
  } catch (error) {
    console.error('加载服务器类型失败:', error)
    ElMessage.error('加载服务器类型定义失败')
  }
}

// 重置动态模型
const resetDynamicModel = () => {
  for (const key of Object.keys(dynamicModel)) {
    delete dynamicModel[key]
  }
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
const isTextField = (field) => {
  return field.type === 'text'
}

// 判断是否为路径字段
const isPathField = (field) => {
  return field?.type === 'path'
}

// 解析目录选择上下文
const resolveDirectoryContext = (field) => {
  const stype = formData.type
  if (!field || activeTab.value !== 'data') {
    return { mode: 'local' }
  }
  // 所有路径字段都使用 local 模式
  // access_path 和 mount_path 都是本地文件系统路径
  return { mode: 'local' }
}

// 路径拼接
const joinPath = (base, name) => {
  if (!base || base === '/') return `/${name}`
  return `${base.replace(/\/+$/, '')}/${name}`
}

// 打开目录选择对话框
const openPathDialog = async (field) => {
  if (!field) return
  const ctx = resolveDirectoryContext(field)

  // API模式需要先填写host和port
  if (ctx.mode === 'api' && (!ctx.host || !ctx.port)) {
    ElMessage.warning('请先填写 Host 和 Port')
    return
  }

  pathDialogField.value = field
  pathDialogContext.mode = ctx.mode
  pathDialogContext.type = ctx.type || ''
  pathDialogContext.host = ctx.host || ''
  pathDialogContext.port = ctx.port || ''
  pathDialogContext.apiKey = ctx.apiKey || ''
  pathDialogPath.value = dynamicModel[field.name] || '/'
  pathDialogSelectedDir.value = ''
  pathDialogVisible.value = true
  await loadDirectories(pathDialogPath.value)
}

// 加载目录列表
const loadDirectories = async (path) => {
  pathDialogLoading.value = true
  pathDialogSelectedDir.value = ''
  try {
    const params = {
      path,
      mode: pathDialogContext.mode
    }
    if (pathDialogContext.mode === 'api') {
      params.type = pathDialogContext.type
      params.host = pathDialogContext.host
      params.port = pathDialogContext.port
      if (pathDialogContext.apiKey) {
        params.apiKey = pathDialogContext.apiKey
      }
    }
    const response = await listDirectories(params)
    pathDialogPath.value = response?.path || path
    pathDialogDirs.value = response?.directories || []
  } catch (error) {
    ElMessage.error('加载目录失败')
  } finally {
    pathDialogLoading.value = false
  }
}

// 进入子目录
const enterDirectory = async (name) => {
  if (!name) return
  const nextPath = joinPath(pathDialogPath.value, name)
  await loadDirectories(nextPath)
}

// 单击选中目录
const selectDirectory = (row) => {
  pathDialogSelectedDir.value = row.name
}

// 进入选中的目录
const enterSelectedDirectory = async () => {
  if (!pathDialogSelectedDir.value) return
  await enterDirectory(pathDialogSelectedDir.value)
  pathDialogSelectedDir.value = ''
}

// 返回上级目录
const handlePathUp = async () => {
  const current = pathDialogPath.value || '/'
  if (current === '/' || current === '') return
  const next = current.replace(/\/+$/, '').split('/').slice(0, -1).join('/')
  await loadDirectories(next === '' ? '/' : next)
}

// 选择当前目录
const selectCurrentPath = () => {
  if (!pathDialogField.value) return
  dynamicModel[pathDialogField.value.name] = pathDialogPath.value || '/'
  pathDialogVisible.value = false
}

// 构建动态表单的 payload
// 注意：只提交当前可见的字段，防止切换模式后提交隐藏字段的旧值
const buildPayload = () => {
  const payload = {
    name: formData.name,
    type: formData.type,
    enabled: formData.enabled,
    // QoS 字段
    request_timeout_ms: formData.request_timeout_ms,
    connect_timeout_ms: formData.connect_timeout_ms,
    retry_max: formData.retry_max,
    retry_backoff_ms: formData.retry_backoff_ms,
    max_concurrent: formData.max_concurrent
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

onMounted(() => {
  loadDataServerTypes()
  loadServers()
})
</script>

<style scoped lang="scss">
.servers-page {
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    margin: 12px 0 16px;
    padding: 12px 16px;
    background: var(--el-bg-color);
    border-radius: 4px;
  }

  .pagination {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }
}

// 表单提示样式
.type-hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-color-info);
  display: flex;
  align-items: center;
  gap: 4px;

  .el-icon {
    font-size: 14px;
  }
}

.field-hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
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

// 路径选择对话框
.path-dialog {
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
