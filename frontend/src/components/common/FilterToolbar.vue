<template>
  <div class="filter-toolbar">
    <!-- 左侧筛选区 -->
    <div v-if="$slots.filters" class="filters">
      <slot name="filters" />
    </div>

    <!-- 右侧操作区 -->
    <div v-if="$slots.actions" class="actions">
      <slot name="actions" />
    </div>

    <!-- 额外区域（可选） -->
    <slot name="extra" />
  </div>
</template>

<script setup>
defineProps({
  loading: {
    type: Boolean,
    default: false
  },
  sticky: {
    type: Boolean,
    default: false
  },
  dense: {
    type: Boolean,
    default: false
  }
})

defineEmits(['search', 'reset'])
</script>

<style scoped lang="scss">
.filter-toolbar {
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
    gap: 12px;
    min-width: 0;

    :deep(.toolbar-search) {
      flex: 1;
      min-width: 50%;
    }
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }
}
</style>
