<template>
  <div class="jobs-page flex flex-col min-h-[calc(100vh-80px)]">
    <div class="page-header">
      <div>
        <h1 class="page-title">任务配置</h1>
        <p class="page-description">配置同步任务与执行策略</p>
      </div>
    </div>
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
        <el-select
          v-model="filters.dataServerId"
          placeholder="数据服务器"
          clearable
          class="w-200"
          @change="handleSearch"
        >
          <el-option label="全部" value="" />
          <el-option
            v-for="server in dataServerOptions"
            :key="server.id"
            :label="server.label"
            :value="server.id"
          />
        </el-select>
        <el-select
          v-model="filters.strmMode"
          placeholder="STRM模式"
          clearable
          class="w-140"
          @change="handleSearch"
        >
          <el-option label="本地路径" value="local" />
          <el-option label="远程 URL" value="url" />
        </el-select>
        <el-input
          v-model="filters.keyword"
          placeholder="搜索任务名称"
          :prefix-icon="Search"
          clearable
          class="toolbar-search"
          @keyup.enter="handleSearch"
        />
      </template>
      <template #actions>
        <el-button
          :disabled="selectedJobIds.length === 0"
          :icon="VideoPlay"
          @click="handleBatchRun(selectedJobIds)"
        >
          运行
        </el-button>
        <el-button
          :disabled="selectedJobIds.length === 0"
          :icon="Check"
          @click="handleBatchEnable(selectedJobIds)"
        >
          启用
        </el-button>
        <el-button
          :disabled="selectedJobIds.length === 0"
          :icon="Close"
          @click="handleBatchDisable(selectedJobIds)"
        >
          禁用
        </el-button>
        <el-button
          type="danger"
          :disabled="selectedJobIds.length === 0"
          :icon="Delete"
          @click="handleBatchDelete(selectedJobIds)"
        >
          删除
        </el-button>
        <el-button type="primary" :icon="Plus" @click="handleAdd">
          创建
        </el-button>
      </template>
    </FilterToolbar>

    <JobsTable
      :job-list="jobList"
      :loading="loading"
      :action-loading="actionLoading"
      :is-row-action-pending="isRowActionPending"
      :get-server-name="getServerName"
      :get-sync-strategy="getSyncStrategy"
      :get-strm-mode="getStrmMode"
      :get-media-dir="getMediaDir"
      :get-toggle-icon="getToggleIcon"
      @selection-change="handleSelectionChange"
      @trigger="handleTrigger"
      @edit="handleEdit"
      @toggle="handleToggle"
      @delete="handleDelete"
    />

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
      :current-server-is-local="currentServerIsLocal"
      :show-media-dir-warning="showMediaDirWarning"
      :force-remote-only="currentServerRemoteOnly"
      :media-dir-disabled="mediaDirDisabled"
      :data-server-access-path="currentServer?.accessPath || ''"
      :data-server-mount-path="currentServer?.mountPath || ''"
      :data-server-remote-root="currentServer?.remoteRoot || ''"
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
      @toggle="handlePathToggle"
      @refresh="() => pathDlg.load(pathDlg.path.value)"
      @load-more="pathDlg.loadMore"
      @confirm="handlePathConfirm"
    />
  </div>
</template>

<script setup>
import Check from '~icons/ep/check'
import Close from '~icons/ep/close'
import CircleCheckFilled from '~icons/ep/circle-check-filled'
import CircleCloseFilled from '~icons/ep/circle-close-filled'
import Delete from '~icons/ep/delete'
import Plus from '~icons/ep/plus'
import Search from '~icons/ep/search'
import VideoPlay from '~icons/ep/video-play'
import { useJobsPage } from '@/composables/useJobsPage'
import { ref } from 'vue'
import ListPagination from '@/components/common/ListPagination.vue'
import FilterToolbar from '@/components/common/FilterToolbar.vue'
import JobsTable from '@/components/jobs/JobsTable.vue'
import JobFormDialog from '@/components/jobs/JobFormDialog.vue'
import PathDialog from '@/components/common/PathDialog.vue'
const jobsState = useJobsPage()
const {
  loading,
  saving,
  jobList,
  filters,
  pagination,
  dialogVisible,
  dialogTitle,
  formData,
  formRules,
  extsLoading,
  dataServerOptions,
  mediaServerOptions,
  currentServerHasApi,
  currentServerSupportsUrl,
  currentServerIsLocal,
  currentServerRemoteOnly,
  showMediaDirWarning,
  mediaDirDisabled,
  excludeDirsText,
  pathDlg,
  handleSearch,
  handleAdd,
  handleEdit,
  handleSave,
  handleDelete,
  handleBatchRun,
  handleBatchEnable,
  handleBatchDisable,
  handleBatchDelete,
  handleToggle,
  handleTrigger,
  handleServerChange,
  getServerName,
  getSyncStrategy,
  getStrmMode,
  isRowActionPending,
  getMediaDir,
  openPathDialog,
  handlePathSelect,
  handlePathToggle,
  handlePathConfirm
} = jobsState

const actionLoading = jobsState.actionLoading || { trigger: {}, toggle: {} }
const loadJobs = jobsState.loadJobs || (() => {})
const currentServer = jobsState.currentServer || null
const selectedJobIds = ref([])

const getToggleIcon = (row) => {
  return row?.enabled ? CircleCloseFilled : CircleCheckFilled
}

const handleSelectionChange = (rows) => {
  selectedJobIds.value = rows.map((row) => row.id)
}
</script>
