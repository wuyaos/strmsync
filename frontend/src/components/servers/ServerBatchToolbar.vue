<template>
  <div v-show="batchMode" class="batch-toolbar">
    <el-checkbox
      :model-value="isAllSelected(serverList)"
      :indeterminate="selectedCount > 0 && !isAllSelected(serverList)"
      @change="(val) => toggleSelectAll(serverList, val)"
    >
      全选
    </el-checkbox>
    <span class="text-14 text-[var(--el-text-color-regular)] font-medium">已选择 {{ selectedCount }} 项</span>
    <div class="flex-1"></div>
    <el-button
      :disabled="selectedCount === 0"
      @click="handleBatchEnable(serverList, updateServer, reload)"
    >
      批量启用
    </el-button>
    <el-button
      :disabled="selectedCount === 0"
      @click="handleBatchDisable(serverList, updateServer, reload)"
    >
      批量禁用
    </el-button>
    <el-button
      type="danger"
      :disabled="selectedCount === 0"
      @click="handleBatchDelete(serverList, deleteServer, reload)"
    >
      批量删除
    </el-button>
  </div>
</template>

<script setup>
defineProps({
  batchMode: {
    type: Boolean,
    default: false
  },
  serverList: {
    type: Array,
    default: () => []
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
  handleBatchEnable: {
    type: Function,
    required: true
  },
  handleBatchDisable: {
    type: Function,
    required: true
  },
  handleBatchDelete: {
    type: Function,
    required: true
  },
  updateServer: {
    type: Function,
    required: true
  },
  deleteServer: {
    type: Function,
    required: true
  },
  reload: {
    type: Function,
    required: true
  }
})
</script>

<style scoped lang="scss">
.batch-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  column-gap: 12px;
  row-gap: 8px;
  padding: 12px 16px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-left: 4px solid var(--el-color-primary);
  border-radius: 6px;
  box-shadow: var(--el-box-shadow-lighter);
}
</style>
