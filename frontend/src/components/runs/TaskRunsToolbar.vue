<template>
  <div class="runs-toolbar">
    <div class="filters">
      <el-select
        :model-value="filters.status"
        placeholder="状态"
        clearable
        class="w-160"
        @update:model-value="emitUpdate('status', $event)"
        @change="emit('search')"
      >
        <el-option label="成功" value="completed" />
        <el-option label="失败" value="failed" />
        <el-option label="运行中" value="running" />
        <el-option label="已取消" value="cancelled" />
      </el-select>
      <el-select
        :model-value="filters.jobId"
        placeholder="任务"
        clearable
        filterable
        class="w-160"
        @update:model-value="emitUpdate('jobId', $event)"
        @change="emit('search')"
      >
        <el-option
          v-for="job in jobOptions"
          :key="job.id"
          :label="job.name"
          :value="job.id"
        />
      </el-select>
      <el-date-picker
        :model-value="filters.timeRange"
        type="daterange"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        value-format="YYYY-MM-DD"
        class="toolbar-range"
        :unlink-panels="true"
        popper-class="single-range-panel"
        @update:model-value="emitUpdate('timeRange', $event)"
        @change="emit('search')"
      />
    </div>
    <div class="actions">
      <el-button type="danger" :disabled="selectedCount === 0" :icon="Delete" @click="emit('batch-delete')">
        删除
      </el-button>
    </div>
  </div>
</template>

<script setup>
import Delete from '~icons/ep/delete'
const props = defineProps({
  filters: {
    type: Object,
    required: true
  },
  jobOptions: {
    type: Array,
    default: () => []
  },
  selectedCount: {
    type: Number,
    default: 0
  }
})

const emit = defineEmits(['update:filters', 'search', 'batch-delete'])

const emitUpdate = (key, value) => {
  emit('update:filters', { ...props.filters, [key]: value })
}
</script>

<style scoped lang="scss">
.runs-toolbar {
  display: grid;
  grid-template-columns: minmax(320px, 1fr) auto;
  column-gap: 12px;
  row-gap: 12px;
  align-items: center;
  margin-bottom: 16px;
  padding: 16px;
  background: var(--el-bg-color);
  border-radius: 4px;

  .filters {
    display: flex;
    align-items: center;
    justify-content: flex-start;
    gap: 12px;
    min-width: 0;
  }

  .toolbar-range {
    flex: 0 0 220px;
    max-width: 220px;
    min-width: 220px;
  }

  :deep(.toolbar-range.el-date-editor) {
    width: 220px !important;
    flex: 0 0 220px;
  }

  :deep(.w-160) {
    width: 160px;
  }

  :deep(.single-range-panel .el-date-range-picker__content.is-right) {
    display: none;
  }

  :deep(.single-range-panel .el-date-range-picker__content.is-left) {
    border-right: 0;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }

}
</style>
