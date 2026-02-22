<template>
  <div class="server-list-panel">
    <div class="servers-toolbar">
      <el-checkbox
        :model-value="isAllSelected(serverList)"
        :indeterminate="selectedCount > 0 && !isAllSelected(serverList)"
        @change="(val) => toggleSelectAll(serverList, val)"
      />
      <el-select
        :model-value="filters.serverType"
        placeholder="服务器类型"
        clearable
        class="w-280"
        @update:model-value="emitUpdate('serverType', $event)"
        @change="emit('search')"
      >
        <el-option label="全部" value="" />
        <el-option
          v-for="item in serverTypeOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </el-select>
      <el-input
        :model-value="filters.keyword"
        placeholder="搜索名称/主机"
        :prefix-icon="Search"
        clearable
        class="toolbar-search"
        @update:model-value="emitUpdate('keyword', $event)"
        @keyup.enter="emit('search')"
      />
      <el-button :disabled="selectedCount === 0" :icon="Check" @click="emit('batch-enable')">
        启用
      </el-button>
      <el-button :disabled="selectedCount === 0" :icon="Close" @click="emit('batch-disable')">
        禁用
      </el-button>
      <el-button
        type="danger"
        :disabled="selectedCount === 0"
        :icon="Delete"
        @click="emit('batch-delete')"
      >
        删除
      </el-button>
      <el-button type="primary" :icon="Plus" @click="emit('add')">
        创建
      </el-button>
    </div>

    <div class="server-card-grid-wrapper">
      <ServerCardGrid
        :server-list="serverList"
        :loading="loading"
        :batch-mode="batchMode"
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
      />
    </div>

    <ListPagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      :page-sizes="pageSizes"
      :affix="affix"
      @update:page="emit('update:page', $event)"
      @update:pageSize="emit('update:pageSize', $event)"
      @change="emit('change', $event)"
    />
  </div>
</template>

<script setup>
import Check from '~icons/ep/check'
import Close from '~icons/ep/close'
import Delete from '~icons/ep/delete'
import Plus from '~icons/ep/plus'
import Search from '~icons/ep/search'
import ListPagination from '@/components/common/ListPagination.vue'
import ServerCardGrid from '@/components/servers/ServerCardGrid.vue'

const props = defineProps({
  filters: {
    type: Object,
    required: true
  },
  serverTypeOptions: {
    type: Array,
    default: () => []
  },
  serverList: {
    type: Array,
    default: () => []
  },
  loading: {
    type: Boolean,
    default: false
  },
  batchMode: {
    type: Boolean,
    default: false
  },
  selectedCount: {
    type: Number,
    default: 0
  },
  isAllSelected: {
    type: Function,
    required: true
  },
  toggleSelectAll: {
    type: Function,
    required: true
  },
  isSelected: {
    type: Function,
    required: true
  },
  toggleSelect: {
    type: Function,
    required: true
  },
  handleCardClick: {
    type: Function,
    required: true
  },
  getServerIconUrl: {
    type: Function,
    required: true
  },
  getServerIcon: {
    type: Function,
    required: true
  },
  getConnectionStatus: {
    type: Function,
    required: true
  },
  formatTime: {
    type: Function,
    required: true
  },
  handleTest: {
    type: Function,
    required: true
  },
  handleEdit: {
    type: Function,
    required: true
  },
  handleDelete: {
    type: Function,
    required: true
  },
  handleToggleEnabled: {
    type: Function,
    required: true
  },
  page: {
    type: Number,
    required: true
  },
  pageSize: {
    type: Number,
    required: true
  },
  total: {
    type: Number,
    required: true
  },
  pageSizes: {
    type: Array,
    default: () => [12, 24, 48]
  },
  affix: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits([
  'update:filters',
  'search',
  'batch-enable',
  'batch-disable',
  'batch-delete',
  'add',
  'update:page',
  'update:pageSize',
  'change'
])

const emitUpdate = (key, value) => {
  emit('update:filters', { ...props.filters, [key]: value })
}
</script>

<style scoped lang="scss">
.server-list-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
  min-height: 0;
}

.servers-toolbar {
  display: grid;
  grid-template-columns: auto auto minmax(320px, 1fr) auto auto auto auto;
  column-gap: 12px;
  row-gap: 12px;
  align-items: center;
  padding: 16px;
  background: var(--el-bg-color);
  border-radius: 0;

  .toolbar-search {
    width: 100%;
    flex: 1;
    min-width: 320px;
    max-width: none;
  }
}

.server-card-grid-wrapper {
  background: var(--el-bg-color-page);
  border-radius: 0;
  padding: 16px;
}

:deep(.w-280) {
  width: 280px;
}
</style>
