<template>
  <div v-if="shouldShow" :class="['list-pagination', { 'page-pagination': sticky }]">
    <el-pagination
      :current-page="page"
      :page-size="pageSize"
      :total="total"
      :page-sizes="pageSizes"
      :layout="layout"
      :background="background"
      @current-change="handlePageChange"
      @size-change="handleSizeChange"
    />
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
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
    default: () => [10, 20, 50, 100]
  },
  layout: {
    type: String,
    default: 'total, sizes, prev, pager, next, jumper'
  },
  sticky: {
    type: Boolean,
    default: true
  },
  hideOnSinglePage: {
    type: Boolean,
    default: false
  },
  background: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['update:page', 'update:pageSize', 'change'])

// 是否显示分页器
const shouldShow = computed(() => {
  if (!props.hideOnSinglePage) return true
  return props.total > props.pageSize
})

// 页码变更
const handlePageChange = (newPage) => {
  emit('update:page', newPage)
  emit('change', { page: newPage, pageSize: props.pageSize })
}

// 页大小变更
const handleSizeChange = (newSize) => {
  emit('update:pageSize', newSize)
  emit('change', { page: props.page, pageSize: newSize })
}
</script>

<style scoped lang="scss">
.list-pagination {
  display: flex;
  justify-content: flex-end;
}

.list-pagination:not(.page-pagination) {
  margin-top: var(--space-16);
}
</style>
