<template>
  <div class="servers-page flex flex-col gap-16 p-20 min-h-[calc(100vh-80px)]">
    <div class="page-header">
      <div>
        <h1 class="page-title">服务器管理</h1>
        <p class="page-description">创建/管理数据与媒体服务器</p>
      </div>
    </div>
    <el-tabs
      v-model="activeTab"
      type="border-card"
      class="servers-tabs"
      @tab-change="handleTabChange"
    >
      <el-tab-pane label="数据服务器" name="data">
        <ServerListPanel
          v-if="activeTab === 'data'"
          v-model:page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :filters="filters"
          :server-type-options="serverTypeOptions"
          :server-list="serverList"
          :loading="loading"
          :batch-mode="batchMode"
          :selected-count="selectedCount"
          :is-all-selected="isAllSelected"
          :toggle-select-all="toggleSelectAll"
          :is-selected="isSelected"
          :toggle-select="toggleSelect"
          :handle-card-click="handleCardClick"
          :get-server-icon-url="getServerIconUrl"
          :get-server-icon="getServerIcon"
          :get-connection-status="getConnectionStatus"
          :format-time="formatTime"
          :handle-test="handleTest"
          :handle-edit="handleEdit"
          :handle-delete="handleDelete"
          :handle-toggle-enabled="handleToggleEnabled"
          @update:filters="updateFilters"
          @search="handleSearch"
          @batch-enable="handleBatchEnableAction"
          @batch-disable="handleBatchDisableAction"
          @batch-delete="handleBatchDeleteAction"
          @add="handleAdd"
          @change="loadServers"
        />
      </el-tab-pane>
      <el-tab-pane label="媒体服务器" name="media">
        <ServerListPanel
          v-if="activeTab === 'media'"
          v-model:page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :filters="filters"
          :server-type-options="serverTypeOptions"
          :server-list="serverList"
          :loading="loading"
          :batch-mode="batchMode"
          :selected-count="selectedCount"
          :is-all-selected="isAllSelected"
          :toggle-select-all="toggleSelectAll"
          :is-selected="isSelected"
          :toggle-select="toggleSelect"
          :handle-card-click="handleCardClick"
          :get-server-icon-url="getServerIconUrl"
          :get-server-icon="getServerIcon"
          :get-connection-status="getConnectionStatus"
          :format-time="formatTime"
          :handle-test="handleTest"
          :handle-edit="handleEdit"
          :handle-delete="handleDelete"
          :handle-toggle-enabled="handleToggleEnabled"
          @update:filters="updateFilters"
          @search="handleSearch"
          @batch-enable="handleBatchEnableAction"
          @batch-disable="handleBatchDisableAction"
          @batch-delete="handleBatchDeleteAction"
          @add="handleAdd"
          @change="loadServers"
        />
      </el-tab-pane>
    </el-tabs>
    <ServerFormDialog
      v-model="dialogVisible"
      :mode="activeTab"
      :editing-server="editingServer"
      :data-type-defs="dataServerTypeDefs"
      :media-type-options="mediaServerTypeOptions"
      :server-list="serverList"
      @saved="handleFormSaved"
    />
  </div>
</template>

<script setup>
import { useServersPage } from '@/composables/useServersPage'
import ServerFormDialog from '@/components/servers/ServerFormDialog.vue'
import ServerListPanel from '@/components/servers/ServerListPanel.vue'
const {
  activeTab,
  loading,
  serverList,
  filters,
  pagination,
  dialogVisible,
  editingServer,
  dataServerTypeDefs,
  mediaServerTypeOptions,
  serverTypeOptions,
  batchMode,
  selectedCount,
  isAllSelected,
  toggleSelect,
  toggleSelectAll,
  isSelected,
  handleBatchEnable,
  handleBatchDisable,
  handleBatchDelete,
  formatTime,
  loadServers,
  handleSearch,
  handleTabChange,
  handleAdd,
  handleEdit,
  handleFormSaved,
  handleDelete,
  handleTest,
  updateServer,
  deleteServer,
  getServerIcon,
  getServerIconUrl,
  getConnectionStatus,
  handleCardClick,
  handleToggleEnabled
} = useServersPage({ enableConnectivity: false })

const updateFilters = (next) => {
  Object.assign(filters, next || {})
}

const handleBatchEnableAction = () => {
  handleBatchEnable(serverList, updateServer, loadServers)
}

const handleBatchDisableAction = () => {
  handleBatchDisable(serverList, updateServer, loadServers)
}

const handleBatchDeleteAction = () => {
  handleBatchDelete(serverList, deleteServer, loadServers)
}
</script>

<style scoped lang="scss">
.servers-tabs {
  :deep(.el-tabs__header) {
    margin-bottom: 0;
  }

  :deep(.el-tabs__content) {
    padding: 0;
  }
}
</style>
