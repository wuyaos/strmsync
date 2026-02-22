<template>
  <div class="server-toolbar-wrapper">
    <!-- Tabs -->
    <el-tabs :model-value="activeTab" @tab-change="$emit('update:activeTab', $event)">
      <el-tab-pane label="数据服务器" name="data" />
      <el-tab-pane label="媒体服务器" name="media" />
    </el-tabs>

    <!-- 工具栏 -->
    <div class="flex items-center gap-12 my-12 mb-16 px-16 py-12 bg-[var(--el-bg-color)] rounded-4">
      <el-input
        :model-value="keyword"
        placeholder="搜索名称/主机"
        :prefix-icon="Search"
        clearable
        class="w-320"
        @update:model-value="$emit('update:keyword', $event)"
        @keyup.enter="$emit('search')"
      />
      <div class="flex-1"></div>
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
    <div
      v-show="batchMode"
      class="flex items-center gap-12 mb-16 px-16 py-12 bg-[var(--el-color-warning-light-9)] border border-[var(--el-color-warning-light-7)] rounded-4"
    >
      <el-checkbox
        :model-value="allSelected"
        :indeterminate="selectedCount > 0 && !allSelected"
        @change="$emit('select-all', $event)"
      >
        全选
      </el-checkbox>
      <span class="text-14 text-[var(--el-text-color-regular)] font-medium">已选择 {{ selectedCount }} 项</span>
      <div class="flex-1"></div>
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
import Plus from '~icons/ep/plus'
import Search from '~icons/ep/search'

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
