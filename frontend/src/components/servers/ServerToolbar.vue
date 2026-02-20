<template>
  <div class="server-toolbar-wrapper">
    <!-- Tabs -->
    <el-tabs :model-value="activeTab" @tab-change="$emit('update:activeTab', $event)">
      <el-tab-pane label="数据服务器" name="data" />
      <el-tab-pane label="媒体服务器" name="media" />
    </el-tabs>

    <!-- 工具栏 -->
    <div class="toolbar">
      <el-input
        :model-value="keyword"
        placeholder="搜索名称/主机"
        :prefix-icon="Search"
        clearable
        style="width: 320px"
        @update:model-value="$emit('update:keyword', $event)"
        @keyup.enter="$emit('search')"
      />
      <div style="flex: 1"></div>
      <el-button
        :type="batchMode ? 'warning' : 'default'"
        @click="$emit('toggle-batch', !batchMode)"
      >
        {{ batchMode ? '退出批量' : '批量操作' }}
      </el-button>
      <el-button type="primary" :icon="Plus" @click="$emit('add')">
        新增服务器
      </el-button>
    </div>

    <!-- 批量操作工具栏 -->
    <div v-show="batchMode" class="batch-toolbar">
      <el-checkbox
        :model-value="allSelected"
        :indeterminate="selectedCount > 0 && !allSelected"
        @change="$emit('select-all', $event)"
      >
        全选
      </el-checkbox>
      <span class="selected-count">已选择 {{ selectedCount }} 项</span>
      <div style="flex: 1"></div>
      <el-button
        :disabled="selectedCount === 0"
        @click="$emit('batch-enable')"
      >
        批量启用
      </el-button>
      <el-button
        :disabled="selectedCount === 0"
        @click="$emit('batch-disable')"
      >
        批量禁用
      </el-button>
      <el-button
        type="danger"
        :disabled="selectedCount === 0"
        @click="$emit('batch-delete')"
      >
        批量删除
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { Plus, Search } from '@element-plus/icons-vue'

defineProps({
  activeTab: {
    type: String,
    required: true
  },
  keyword: {
    type: String,
    default: ''
  },
  batchMode: {
    type: Boolean,
    default: false
  },
  selectedCount: {
    type: Number,
    default: 0
  },
  allSelected: {
    type: Boolean,
    default: false
  },
  loading: {
    type: Boolean,
    default: false
  }
})

defineEmits([
  'update:activeTab',
  'update:keyword',
  'search',
  'add',
  'toggle-batch',
  'select-all',
  'batch-enable',
  'batch-disable',
  'batch-delete'
])
</script>

<style scoped lang="scss">
.server-toolbar-wrapper {
  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    margin: 12px 0 16px;
    padding: 12px 16px;
    background: var(--el-bg-color);
    border-radius: 4px;
  }

  .batch-toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
    padding: 12px 16px;
    background: var(--el-color-warning-light-9);
    border: 1px solid var(--el-color-warning-light-7);
    border-radius: 4px;

    .selected-count {
      font-size: 14px;
      color: var(--el-text-color-regular);
      font-weight: 500;
    }
  }
}
</style>
