<template>
  <div class="filter-toolbar">
    <!-- 左侧筛选区 -->
    <div v-if="$slots.filters" class="toolbar-filters">
      <slot name="filters" />
    </div>

    <!-- 中间占位 -->
    <div class="flex-1" />

    <!-- 右侧操作区 -->
    <div v-if="$slots.actions" class="toolbar-actions">
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
  display: flex;
  align-items: center;
  gap: var(--toolbar-gap);
  margin-bottom: var(--space-16);
  padding: var(--space-16);
  background: var(--el-bg-color);
  border-radius: var(--card-radius);

  .toolbar-filters {
    display: flex;
    align-items: center;
    gap: var(--toolbar-gap);
  }

  .toolbar-actions {
    display: flex;
    align-items: center;
    gap: var(--toolbar-gap);
  }
}
</style>
