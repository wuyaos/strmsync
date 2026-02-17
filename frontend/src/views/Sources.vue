<template>
  <div class="sources-page">
    <!-- 工具栏 -->
    <div class="toolbar">
      <el-input
        v-model="searchText"
        placeholder="搜索数据源"
        :prefix-icon="Search"
        clearable
        style="width: 300px"
      />
      <el-select
        v-model="filterType"
        placeholder="类型"
        clearable
        style="width: 150px"
      >
        <el-option label="全部" value="" />
        <el-option label="Local" value="local" />
        <el-option label="CloudDrive2" value="clouddrive2" />
        <el-option label="OpenList" value="openlist" />
      </el-select>
      <el-select
        v-model="filterStatus"
        placeholder="状态"
        clearable
        style="width: 150px"
      >
        <el-option label="全部" value="" />
        <el-option label="空闲" value="idle" />
        <el-option label="扫描中" value="scanning" />
        <el-option label="监控中" value="watching" />
        <el-option label="错误" value="error" />
      </el-select>

      <div style="flex: 1"></div>

      <el-button-group>
        <el-button
          :type="viewMode === 'card' ? 'primary' : ''"
          :icon="Grid"
          @click="viewMode = 'card'"
        />
        <el-button
          :type="viewMode === 'list' ? 'primary' : ''"
          :icon="List"
          @click="viewMode = 'list'"
        />
      </el-button-group>

      <el-button type="primary" :icon="Plus" @click="handleAdd">
        添加数据源
      </el-button>
    </div>

    <!-- 卡片视图 -->
    <el-row v-if="viewMode === 'card'" :gutter="16">
      <el-col
        v-for="source in filteredSources"
        :key="source.id"
        :xs="24"
        :sm="12"
        :md="8"
        :lg="6"
      >
        <el-card shadow="hover" class="source-card">
          <template #header>
            <div class="card-header">
              <div class="source-header">
                <el-icon :size="24"><FolderOpened /></el-icon>
                <span class="source-name">{{ source.name }}</span>
              </div>
              <el-tag
                :type="getStatusType(source.status)"
                size="small"
                effect="dark"
              >
                {{ getStatusText(source.status) }}
              </el-tag>
            </div>
          </template>

          <div class="source-body">
            <div class="source-info-item">
              <span class="label">类型:</span>
              <el-tag size="small">{{ source.type }}</el-tag>
            </div>
            <div class="source-info-item">
              <span class="label">文件数:</span>
              <span class="value">{{ source.file_count || 0 }}</span>
            </div>
            <div class="source-info-item">
              <span class="label">最后扫描:</span>
              <span class="value">
                {{ source.last_scan_at ? formatTime(source.last_scan_at) : '从未' }}
              </span>
            </div>

            <!-- 错误信息 -->
            <el-alert
              v-if="source.status === 'error' && source.error_message"
              :title="source.error_message"
              type="error"
              :closable="false"
              style="margin-top: 12px"
            />

            <!-- 扫描进度 -->
            <el-progress
              v-if="source.status === 'scanning'"
              :percentage="source.scan_progress || 0"
              style="margin-top: 12px"
            />
          </div>

          <template #footer>
            <div class="source-actions">
              <el-button size="small" :icon="Setting" @click="handleEdit(source)">
                设置
              </el-button>
              <el-button
                size="small"
                type="primary"
                :icon="Refresh"
                :loading="source.status === 'scanning'"
                @click="handleScan(source)"
              >
                扫描
              </el-button>
              <el-dropdown trigger="click">
                <el-button size="small" :icon="More" />
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item :icon="View" @click="handleWatch(source)">
                      {{ source.status === 'watching' ? '停止监控' : '启动监控' }}
                    </el-dropdown-item>
                    <el-dropdown-item :icon="Folder" @click="handleSyncMetadata(source)">
                      同步元数据
                    </el-dropdown-item>
                    <el-dropdown-item :icon="Bell" @click="handleNotify(source)">
                      触发通知
                    </el-dropdown-item>
                    <el-dropdown-item :icon="Delete" divided @click="handleDelete(source)">
                      删除
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-card>
      </el-col>
    </el-row>

    <!-- 列表视图 -->
    <el-table v-else :data="filteredSources" stripe style="width: 100%">
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column prop="type" label="类型" width="120">
        <template #default="{ row }">
          <el-tag size="small">{{ row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small" effect="dark">
            {{ getStatusText(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="file_count" label="文件数" width="100" />
      <el-table-column prop="last_scan_at" label="最后扫描" width="150">
        <template #default="{ row }">
          {{ row.last_scan_at ? formatTime(row.last_scan_at) : '从未' }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button size="small" :icon="Setting" @click="handleEdit(row)" />
          <el-button
            size="small"
            type="primary"
            :icon="Refresh"
            :loading="row.status === 'scanning'"
            @click="handleScan(row)"
          />
          <el-button size="small" :icon="Delete" @click="handleDelete(row)" />
        </template>
      </el-table-column>
    </el-table>

    <!-- 配置抽屉 -->
    <el-drawer
      v-model="drawerVisible"
      :title="drawerTitle"
      size="50%"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="formData"
        :rules="formRules"
        label-width="120px"
      >
        <el-divider content-position="left">基本信息</el-divider>

        <el-form-item label="数据源名称" prop="name">
          <el-input v-model="formData.name" placeholder="请输入数据源名称" />
        </el-form-item>

        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="formData.type" @change="handleTypeChange">
            <el-radio value="local">Local</el-radio>
            <el-radio value="clouddrive2">CloudDrive2</el-radio>
            <el-radio value="openlist">OpenList</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item
          v-if="formData.type !== 'local'"
          label="监控模式"
          prop="monitoring_mode"
        >
          <el-radio-group v-model="formData.monitoring_mode">
            <el-radio value="local">本地挂载模式</el-radio>
            <el-radio value="api">API监控模式</el-radio>
          </el-radio-group>
          <div class="form-help">
            本地挂载模式：通过本地文件系统访问（需要已挂载目录）<br/>
            API监控模式：直接通过 {{ formData.type === 'clouddrive2' ? 'CloudDrive2' : 'OpenList' }} API 获取目录列表，实时性更好
          </div>
        </el-form-item>

        <el-form-item label="启用">
          <el-switch v-model="formData.enabled" />
        </el-form-item>

        <el-divider content-position="left">路径配置</el-divider>

        <el-alert
          type="info"
          :closable="false"
          style="margin-bottom: 20px"
        >
          <template #title>
            <strong>路径配置说明</strong>
          </template>
          <div style="font-size: 13px; line-height: 1.8; margin-top: 8px;">
            <p style="margin: 0 0 12px 0;">
              <strong>• 监控目录</strong>：本项目可访问的实际媒体文件目录，用于扫描视频文件<br/>
              <strong>• 目的目录</strong>：生成的 STRM 文件存放位置，需添加到媒体服务器库<br/>
              <strong>• 媒体路径</strong>：写入 STRM 文件内容中的路径，媒体服务器通过此路径访问源文件
            </p>

            <el-divider style="margin: 12px 0" />

            <div v-if="formData.type === 'local'" style="background: var(--el-fill-color-lighter); padding: 12px; border-radius: 4px;">
              <p style="margin: 0 0 8px 0; font-weight: 600;">Local 类型示例：</p>
              <p style="margin: 0 0 4px 0;">• 监控目录: <code>/mnt/media/movies</code> （本地可访问的源文件目录）</p>
              <p style="margin: 0 0 4px 0;">• 目的目录: <code>/mnt/strm/movies</code> （生成STRM文件的输出目录）</p>
              <p style="margin: 0 0 8px 0;">• 媒体路径: <code>/media/movies</code> （媒体服务器内的路径，Docker容器映射路径或与监控目录一致）</p>
              <p style="margin: 0; font-size: 12px; color: var(--el-text-color-secondary);">
                如媒体服务器是Docker部署且映射了 <code>-v /mnt/media/movies:/media/movies</code>，则媒体路径填容器内路径
              </p>
            </div>

            <div v-else-if="formData.type === 'clouddrive2'" style="background: var(--el-fill-color-lighter); padding: 12px; border-radius: 4px;">
              <p style="margin: 0 0 8px 0; font-weight: 600;">CloudDrive2 类型示例：</p>
              <p style="margin: 0 0 4px 0;">• 监控目录: <code>/mnt/clouddrive/电影</code> （扫描的监控目录）</p>
              <p style="margin: 0 0 4px 0;">• 目的目录: <code>/mnt/strm/电影</code> （STRM输出目录）</p>
              <p style="margin: 0 0 8px 0;">• 媒体路径: <code>/mnt/cd2</code> 或 <code>http://192.168.1.100:19798/dav</code></p>
              <p style="margin: 0; font-size: 12px; color: var(--el-text-color-secondary);">
                媒体路径可填CloudDrive2挂载的本地根路径，或使用WebDAV地址让媒体服务器通过网络访问
              </p>
            </div>

            <div v-else-if="formData.type === 'openlist'" style="background: var(--el-fill-color-lighter); padding: 12px; border-radius: 4px;">
              <p style="margin: 0 0 8px 0; font-weight: 600;">OpenList 类型示例：</p>
              <p style="margin: 0 0 4px 0;">• 监控目录: <code>/mnt/openlist/视频</code> （扫描的监控目录）</p>
              <p style="margin: 0 0 4px 0;">• 目的目录: <code>/mnt/strm/视频</code> （STRM输出目录）</p>
              <p style="margin: 0 0 8px 0;">• 媒体路径: <code>/mnt/openlist</code> 或 <code>http://192.168.1.100:5244/d</code></p>
              <p style="margin: 0; font-size: 12px; color: var(--el-text-color-secondary);">
                媒体路径可填OpenList挂载的本地根路径，或使用HTTP地址让媒体服务器通过网络访问
              </p>
            </div>

            <div v-else style="background: var(--el-fill-color-lighter); padding: 12px; border-radius: 4px;">
              <p style="margin: 0; font-size: 12px; color: var(--el-text-color-secondary);">
                请先选择数据源类型以查看对应的配置示例
              </p>
            </div>
          </div>
        </el-alert>

        <el-form-item label="监控目录" prop="source_prefix">
          <el-autocomplete
            v-model="formData.source_prefix"
            :fetch-suggestions="(queryString, cb) => {
              const list = pathHistory.source_prefix.filter(v => v.includes(queryString || ''))
              cb(list.map(v => ({ value: v })))
            }"
            :trigger-on-focus="true"
            placeholder="/mnt/source"
            style="width: 100%"
          >
            <template #append>
              <el-button :icon="Folder" @click="openDirBrowser('source_prefix')" />
            </template>
          </el-autocomplete>
          <div class="form-help">
            本项目可访问的实际媒体文件目录，用于扫描视频文件（点击文件夹图标可浏览选择目录）
          </div>
        </el-form-item>

        <el-form-item label="目的目录" prop="target_prefix">
          <el-autocomplete
            v-model="formData.target_prefix"
            :fetch-suggestions="(queryString, cb) => {
              const list = pathHistory.target_prefix.filter(v => v.includes(queryString || ''))
              cb(list.map(v => ({ value: v })))
            }"
            :trigger-on-focus="true"
            placeholder="/mnt/target"
            style="width: 100%"
          >
            <template #append>
              <el-button :icon="Folder" @click="openDirBrowser('target_prefix')" />
            </template>
          </el-autocomplete>
          <div class="form-help">
            生成的 STRM 文件存放位置，需要将此目录添加到媒体服务器库中（点击文件夹图标可浏览选择目录）
          </div>
        </el-form-item>

        <el-form-item label="媒体路径" prop="strm_prefix">
          <el-autocomplete
            v-model="formData.strm_prefix"
            :fetch-suggestions="(queryString, cb) => {
              const list = pathHistory.strm_prefix.filter(v => v.includes(queryString || ''))
              cb(list.map(v => ({ value: v })))
            }"
            :trigger-on-focus="true"
            :placeholder="formData.type === 'local' ? '/media' : formData.type === 'clouddrive2' ? '/mnt/cd2 或 http://...' : formData.type === 'openlist' ? '/mnt/openlist 或 http://...' : '/strm'"
            style="width: 100%"
          >
            <template #append>
              <el-button :icon="Folder" @click="selectPath('strm_prefix')" />
            </template>
          </el-autocomplete>
          <div class="form-help">
            写入 STRM 文件内容中的路径前缀，媒体服务器通过此路径访问源文件
            <span v-if="formData.type === 'local'">（Docker容器内路径或与监控目录一致）</span>
            <span v-else-if="formData.type === 'clouddrive2'">（CloudDrive2挂载路径或WebDAV地址）</span>
            <span v-else-if="formData.type === 'openlist'">（OpenList挂载路径或HTTP地址）</span>
          </div>
        </el-form-item>

        <template v-if="formData.type !== 'local' && formData.monitoring_mode === 'api'">
          <el-divider content-position="left">API 连接配置</el-divider>

          <el-alert
            type="info"
            :closable="false"
            style="margin-bottom: 16px"
          >
            <template #title>
              API监控模式需要填写 {{ formData.type === 'clouddrive2' ? 'CloudDrive2' : 'OpenList' }} 服务的连接信息
            </template>
          </el-alert>

          <el-form-item label="主机地址" prop="config.host">
            <el-input
              v-model="formData.config.host"
              placeholder="192.168.1.100"
            />
            <div class="form-help">
              {{ formData.type === 'clouddrive2' ? 'CloudDrive2' : 'OpenList' }} 服务的IP地址或域名
            </div>
          </el-form-item>

          <el-form-item label="端口" prop="config.port">
            <el-input-number
              v-model="formData.config.port"
              :min="1"
              :max="65535"
              style="width: 100%"
            />
            <div class="form-help">
              默认端口: CloudDrive2 为 19798, OpenList 为 5244
            </div>
          </el-form-item>

          <el-form-item label="认证密钥" prop="config.apiKey">
            <el-input
              v-model="formData.config.apiKey"
              type="password"
              show-password
              :placeholder="formData.type === 'clouddrive2' ? 'CloudDrive2 API Key (必填)' : 'OpenList Token (如已设置)'"
            />
            <div class="form-help">
              <span v-if="formData.type === 'clouddrive2'">CloudDrive2 的认证密钥，必填</span>
              <span v-else>OpenList 的认证Token，如果服务器未设置认证可留空</span>
            </div>
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </template>
    </el-drawer>

    <!-- 目录浏览器 -->
    <el-dialog
      v-model="dirBrowserVisible"
      title="选择目录"
      width="600px"
      destroy-on-close
    >
      <div class="dir-browser">
        <!-- 当前路径 -->
        <div class="current-path">
          <el-icon><FolderOpened /></el-icon>
          <span>{{ currentPath || '/' }}</span>
        </div>

        <!-- 目录列表 -->
        <el-scrollbar height="400px">
          <div v-loading="dirLoading" class="dir-list">
            <!-- 返回上级 -->
            <div
              v-if="currentPath && currentPath !== '/'"
              class="dir-item parent"
              @click="goToParent"
            >
              <el-icon><Back /></el-icon>
              <span>../</span>
            </div>

            <!-- 目录列表 -->
            <div
              v-for="dir in directories"
              :key="dir"
              class="dir-item"
              @click="enterDirectory(dir)"
            >
              <el-icon><Folder /></el-icon>
              <span>{{ dir }}</span>
            </div>

            <!-- 空状态 -->
            <el-empty
              v-if="!dirLoading && directories.length === 0"
              description="此目录为空"
              :image-size="80"
            />
          </div>
        </el-scrollbar>
      </div>

      <template #footer>
        <el-button @click="dirBrowserVisible = false">取消</el-button>
        <el-button type="primary" @click="selectCurrentPath">
          选择当前目录
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  getSourceList,
  createSource,
  updateSource,
  deleteSource,
  scanSource,
  startWatch,
  stopWatch,
  syncMetadata,
  triggerNotify
} from '@/api/source'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Plus,
  Grid,
  List,
  FolderOpened,
  Setting,
  Refresh,
  More,
  View,
  Folder,
  Bell,
  Delete,
  Back
} from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

// 视图模式
const viewMode = ref('card')

// 搜索和过滤
const searchText = ref('')
const filterType = ref('')
const filterStatus = ref('')

// 数据源列表
const sourceList = ref([])

// 抽屉
const drawerVisible = ref(false)
const drawerTitle = computed(() => (formData.value.id ? '编辑数据源' : '添加数据源'))

// 表单
const formRef = ref(null)
const formData = ref({
  name: '',
  type: 'local',
  monitoring_mode: 'local',
  enabled: true,
  source_prefix: '',
  target_prefix: '',
  strm_prefix: '',
  config: {
    host: '',
    port: 19798,
    apiKey: ''
  }
})

const formRules = {
  name: [{ required: true, message: '请输入数据源名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  monitoring_mode: [
    {
      validator: (rule, value, callback) => {
        if (formData.value.type === 'local') return callback()
        if (!value) return callback(new Error('请选择监控模式'))
        return callback()
      },
      trigger: 'change'
    }
  ],
  source_prefix: [{ required: true, message: '请输入源路径前缀', trigger: 'blur' }],
  target_prefix: [{ required: true, message: '请输入目标路径前缀', trigger: 'blur' }],
  strm_prefix: [{ required: true, message: '请输入STRM路径前缀', trigger: 'blur' }],
  'config.host': [
    {
      validator: (rule, value, callback) => {
        if (formData.value.type !== 'local' && formData.value.monitoring_mode === 'api') {
          if (!value || value.trim() === '') {
            return callback(new Error('API监控模式下主机地址为必填项'))
          }
        }
        return callback()
      },
      trigger: 'blur'
    }
  ],
  'config.port': [
    {
      validator: (rule, value, callback) => {
        if (formData.value.type !== 'local' && formData.value.monitoring_mode === 'api') {
          if (!value || value < 1 || value > 65535) {
            return callback(new Error('API监控模式下端口为必填项 (1-65535)'))
          }
        }
        return callback()
      },
      trigger: 'change'
    }
  ],
  'config.apiKey': [
    {
      validator: (rule, value, callback) => {
        // CloudDrive2 必填，OpenList 可选
        if (formData.value.type === 'clouddrive2' && formData.value.monitoring_mode === 'api') {
          if (!value || value.trim() === '') {
            return callback(new Error('CloudDrive2 API监控模式下认证密钥为必填项'))
          }
        }
        return callback()
      },
      trigger: 'blur'
    }
  ]
}

// 路径历史记录
const pathHistory = ref({
  source_prefix: JSON.parse(localStorage.getItem('path_history_source') || '[]'),
  target_prefix: JSON.parse(localStorage.getItem('path_history_target') || '[]'),
  strm_prefix: JSON.parse(localStorage.getItem('path_history_strm') || '[]')
})

// 目录浏览器
const dirBrowserVisible = ref(false)
const currentPath = ref('/')
const directories = ref([])
const dirLoading = ref(false)
const currentFieldType = ref('')

// 添加路径到历史记录
const addToPathHistory = (type, path) => {
  if (!path) return
  const history = pathHistory.value[type]
  if (!history.includes(path)) {
    history.unshift(path)
    if (history.length > 10) {
      history.pop()
    }
    localStorage.setItem(`path_history_${type.replace('_prefix', '')}`, JSON.stringify(history))
  }
}

// 过滤后的数据源列表
const filteredSources = computed(() => {
  return sourceList.value.filter(source => {
    if (searchText.value && !source.name.includes(searchText.value)) {
      return false
    }
    if (filterType.value && source.type !== filterType.value) {
      return false
    }
    if (filterStatus.value && source.status !== filterStatus.value) {
      return false
    }
    return true
  })
})

// 加载数据源列表
const loadSources = async () => {
  try {
    const data = await getSourceList()
    sourceList.value = data || []
  } catch (error) {
    console.error('加载数据源失败:', error)
  }
}

// 添加数据源
const handleAdd = () => {
  formData.value = {
    name: '',
    type: 'local',
    monitoring_mode: 'local',
    enabled: true,
    source_prefix: '',
    target_prefix: '',
    strm_prefix: '',
    config: {
      host: '',
      port: 19798,
      apiKey: ''
    }
  }
  drawerVisible.value = true
}

// 编辑数据源
const handleEdit = (source) => {
  // 确保config字段完整，处理旧数据兼容
  // 根据类型智能推断默认端口
  let defaultPort = 19798
  if (source?.type === 'openlist') {
    defaultPort = 5244
  } else if (source?.type === 'clouddrive2') {
    defaultPort = 19798
  }

  const config = {
    host: source?.config?.host || '',
    port: source?.config?.port || defaultPort,
    apiKey: source?.config?.apiKey || ''
  }

  formData.value = {
    ...source,
    monitoring_mode: source?.monitoring_mode || 'local',  // 兼容旧数据
    config
  }
  drawerVisible.value = true
}

// 类型切换时调整默认端口和监控模式
const handleTypeChange = (newType) => {
  // 切换为local时,重置监控模式
  if (newType === 'local') {
    formData.value.monitoring_mode = 'local'
    return
  }

  // 非local类型,如果用户还没修改过端口(使用的是其他类型的默认值),则更新为新类型的默认端口
  const currentPort = formData.value.config.port
  if (newType === 'clouddrive2') {
    // 如果当前是OpenList的默认端口或未设置,切换为CloudDrive2默认端口
    if (!currentPort || currentPort === 5244) {
      formData.value.config.port = 19798
    }
  } else if (newType === 'openlist') {
    // 如果当前是CloudDrive2的默认端口或未设置,切换为OpenList默认端口
    if (!currentPort || currentPort === 19798) {
      formData.value.config.port = 5244
    }
  }
}

// 打开目录浏览器
const openDirBrowser = async (fieldType) => {
  currentFieldType.value = fieldType
  currentPath.value = formData.value[fieldType] || '/'
  dirBrowserVisible.value = true
  await loadDirectories(currentPath.value)
}

// 加载目录列表
const loadDirectories = async (path) => {
  try {
    dirLoading.value = true

    // 构建请求参数
    const params = new URLSearchParams()
    params.set('path', path)

    // 根据类型和监控模式决定请求方式
    const type = formData.value.type || 'local'
    const mode = formData.value.monitoring_mode || 'local'

    params.set('mode', mode)
    params.set('type', type)

    // API监控模式需要提供连接配置
    if (type !== 'local' && mode === 'api') {
      const { host, port, apiKey } = formData.value.config || {}

      // 验证必填字段
      if (!host || !host.trim()) {
        ElMessage.warning('API监控模式需要填写主机地址')
        directories.value = []
        return
      }
      if (!port || port < 1 || port > 65535) {
        ElMessage.warning('API监控模式需要填写有效端口')
        directories.value = []
        return
      }
      if (type === 'clouddrive2' && (!apiKey || !apiKey.trim())) {
        ElMessage.warning('CloudDrive2 API监控模式需要填写认证密钥')
        directories.value = []
        return
      }

      params.set('host', host)
      params.set('port', String(port))
      if (apiKey && apiKey.trim()) {
        params.set('apiKey', apiKey)
      }
    }

    const response = await fetch(`/api/files/directories?${params.toString()}`)
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}))
      throw new Error(errorData.error || '加载目录失败')
    }

    const data = await response.json()
    directories.value = data.directories || []
  } catch (error) {
    console.error('加载目录失败:', error)
    ElMessage.error('加载目录失败：' + error.message)
    directories.value = []
  } finally {
    dirLoading.value = false
  }
}

// 进入目录
const enterDirectory = async (dirName) => {
  const newPath = currentPath.value === '/'
    ? `/${dirName}`
    : `${currentPath.value}/${dirName}`
  currentPath.value = newPath
  await loadDirectories(newPath)
}

// 返回上级目录
const goToParent = async () => {
  const parentPath = currentPath.value.substring(0, currentPath.value.lastIndexOf('/'))
  currentPath.value = parentPath || '/'
  await loadDirectories(currentPath.value)
}

// 选择当前路径
const selectCurrentPath = () => {
  formData.value[currentFieldType.value] = currentPath.value
  dirBrowserVisible.value = false
}

// 选择路径（媒体路径使用输入框）
const selectPath = (type) => {
  // 简单的路径输入对话框
  ElMessageBox.prompt('请输入路径', '选择路径', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputValue: formData.value[type],
    inputPattern: /.+/,
    inputErrorMessage: '路径不能为空'
  }).then(({ value }) => {
    formData.value[type] = value
  }).catch(() => {
    // 用户取消
  })
}

// 保存数据源
const handleSave = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    try {
      // 保存路径历史
      addToPathHistory('source_prefix', formData.value.source_prefix)
      addToPathHistory('target_prefix', formData.value.target_prefix)
      addToPathHistory('strm_prefix', formData.value.strm_prefix)

      if (formData.value.id) {
        await updateSource(formData.value.id, formData.value)
        ElMessage.success('更新成功')
      } else {
        await createSource(formData.value)
        ElMessage.success('创建成功')
      }

      drawerVisible.value = false
      loadSources()
    } catch (error) {
      console.error('保存失败:', error)
      ElMessage.error(error.response?.data?.error || '保存失败')
    }
  })
}

// 删除数据源
const handleDelete = async (source) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除数据源 "${source.name}" 吗？`,
      '删除确认',
      {
        type: 'warning'
      }
    )

    await deleteSource(source.id)
    ElMessage.success('删除成功')
    loadSources()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除失败:', error)
    }
  }
}

// 扫描数据源
const handleScan = async (source) => {
  try {
    await scanSource(source.id)
    ElMessage.success('扫描任务已提交')
    loadSources()
  } catch (error) {
    console.error('扫描失败:', error)
  }
}

// 监控数据源
const handleWatch = async (source) => {
  try {
    if (source.status === 'watching') {
      await stopWatch(source.id)
      ElMessage.success('已停止监控')
    } else {
      await startWatch(source.id)
      ElMessage.success('已启动监控')
    }
    loadSources()
  } catch (error) {
    console.error('监控操作失败:', error)
  }
}

// 同步元数据
const handleSyncMetadata = async (source) => {
  try {
    await syncMetadata(source.id)
    ElMessage.success('元数据同步任务已提交')
  } catch (error) {
    console.error('同步失败:', error)
  }
}

// 触发通知
const handleNotify = async (source) => {
  try {
    await triggerNotify(source.id)
    ElMessage.success('通知已触发')
  } catch (error) {
    console.error('触发通知失败:', error)
  }
}

// 获取状态类型
const getStatusType = (status) => {
  const typeMap = {
    idle: 'info',
    scanning: 'primary',
    watching: 'success',
    error: 'danger'
  }
  return typeMap[status] || 'info'
}

// 获取状态文本
const getStatusText = (status) => {
  const textMap = {
    idle: '空闲',
    scanning: '扫描中',
    watching: '监控中',
    error: '错误'
  }
  return textMap[status] || status
}

// 格式化时间
const formatTime = (time) => {
  return dayjs(time).fromNow()
}

// 组件挂载时加载数据
let refreshInterval = null

onMounted(() => {
  loadSources()

  // 每10秒自动刷新
  refreshInterval = setInterval(loadSources, 10000)
})

// 组件卸载时清理定时器
onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }
})
</script>

<style scoped lang="scss">
.sources-page {
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
    padding: 16px;
    background: var(--el-bg-color);
    border-radius: 4px;
  }

  .form-help {
    margin-top: 4px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    line-height: 1.5;
  }

  .source-card {
    margin-bottom: 16px;
    height: 100%;

    .card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;

      .source-header {
        display: flex;
        align-items: center;
        gap: 8px;

        .source-name {
          font-weight: 500;
        }
      }
    }

    .source-body {
      .source-info-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 8px 0;
        border-bottom: 1px solid var(--el-border-color-lighter);

        &:last-child {
          border-bottom: none;
        }

        .label {
          color: var(--el-text-color-secondary);
          font-size: 14px;
        }

        .value {
          font-weight: 500;
        }
      }
    }

    .source-actions {
      display: flex;
      gap: 8px;
    }
  }

  .dir-browser {
    .current-path {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 12px;
      background: var(--el-fill-color-light);
      border-radius: 4px;
      margin-bottom: 12px;
      font-family: 'Consolas', 'Monaco', monospace;
      font-size: 13px;
      color: var(--el-text-color-primary);
    }

    .dir-list {
      min-height: 400px;

      .dir-item {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 12px;
        cursor: pointer;
        border-radius: 4px;
        transition: background 0.2s;

        &:hover {
          background: var(--el-fill-color-light);
        }

        &.parent {
          color: var(--el-text-color-secondary);
        }

        span {
          font-size: 14px;
        }
      }
    }
  }
}
</style>
