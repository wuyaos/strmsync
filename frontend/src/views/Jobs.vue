<template>
  <div class="jobs-page">
    <div class="toolbar">
      <el-select
        v-model="filters.status"
        placeholder="状态"
        clearable
        style="width: 140px"
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
        style="width: 260px"
        @keyup.enter="handleSearch"
      />
      <div style="flex: 1"></div>
      <el-button type="primary" :icon="Plus" @click="handleAdd">
        新增任务
      </el-button>
    </div>

    <el-table v-loading="loading" :data="jobList" stripe style="width: 100%">
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
      width="640px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="formData"
        :rules="formRules"
        label-width="110px"
      >
        <el-form-item label="名称" prop="name">
          <el-input v-model="formData.name" placeholder="请输入任务名称" />
        </el-form-item>

        <el-form-item label="数据服务器" prop="data_server_id">
          <el-select
            v-model="formData.data_server_id"
            filterable
            placeholder="选择数据服务器"
            style="width: 100%"
          >
            <el-option
              v-for="server in dataServers"
              :key="server.id"
              :label="`${server.name} (${server.type})`"
              :value="server.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="媒体服务器" prop="media_server_id">
          <el-select
            v-model="formData.media_server_id"
            filterable
            placeholder="选择媒体服务器"
            style="width: 100%"
          >
            <el-option
              v-for="server in mediaServers"
              :key="server.id"
              :label="`${server.name} (${server.type})`"
              :value="server.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="调度配置" prop="cron">
          <el-input v-model="formData.cron" placeholder="例如 0 */6 * * *" />
          <div class="form-help">使用 Cron 表达式配置定时执行</div>
        </el-form-item>

        <el-form-item label="启用状态" prop="enabled">
          <el-switch v-model="formData.enabled" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">
          保存
        </el-button>
      </template>
    </el-dialog>
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
  getJobList,
  triggerJob,
  updateJob
} from '@/api/jobs'
import { getServerList } from '@/api/servers'
import { normalizeListResponse } from '@/api/normalize'

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
const formRef = ref(null)
const formData = reactive({
  id: null,
  name: '',
  data_server_id: '',
  media_server_id: '',
  cron: '',
  enabled: true
})

const dialogTitle = computed(() => (isEdit.value ? '编辑任务' : '新增任务'))

const formRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  data_server_id: [{ required: true, message: '请选择数据服务器', trigger: 'change' }],
  media_server_id: [{ required: true, message: '请选择媒体服务器', trigger: 'change' }],
  cron: [{ required: true, message: '请输入 Cron 表达式', trigger: 'blur' }]
}

const formatTime = (time) => {
  return dayjs(time).fromNow()
}

// 判断某一行是否有操作正在进行
const isRowActionPending = (rowId) => {
  return actionLoading.trigger[rowId] || actionLoading.toggle[rowId]
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

const handlePageChange = () => {
  loadJobs()
}

const handleSizeChange = () => {
  pagination.page = 1
  loadJobs()
}

const resetForm = () => {
  formData.id = null
  formData.name = ''
  formData.data_server_id = ''
  formData.media_server_id = ''
  formData.cron = ''
  formData.enabled = true
}

const handleAdd = () => {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

const handleEdit = (row) => {
  isEdit.value = true
  formData.id = row.id
  formData.name = row.name
  formData.data_server_id = row.data_server_id || row.data_server?.id || ''
  formData.media_server_id = row.media_server_id || row.media_server?.id || ''
  formData.cron = row.cron || ''
  formData.enabled = row.enabled !== false
  dialogVisible.value = true
}

const handleSave = async () => {
  try {
    await formRef.value?.validate()
    saving.value = true

    const payload = {
      name: formData.name,
      data_server_id: formData.data_server_id,
      media_server_id: formData.media_server_id,
      cron: formData.cron,
      enabled: formData.enabled
    }

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
.jobs-page {
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
    padding: 12px 16px;
    background: var(--el-bg-color);
    border-radius: 4px;
  }

  .pagination {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .form-help {
    margin-top: 4px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
}
</style>
