<template>
  <el-table
    v-loading="loading"
    :data="jobList"
    stripe
    class="w-full"
    row-key="id"
    :reserve-selection="true"
    @selection-change="(rows) => emit('selection-change', rows)"
  >
    <el-table-column type="selection" width="48" />
    <el-table-column prop="enabled" label="状态" width="80" sortable :sort-by="(row) => (row.enabled ? 1 : 0)">
      <template #default="{ row }">
        <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
          {{ row.enabled ? '启用' : '禁用' }}
        </el-tag>
      </template>
    </el-table-column>
    <el-table-column prop="name" label="名称" width="140" sortable />
    <el-table-column label="数据服务器" width="160" sortable :sort-by="(row) => getServerName(row, 'data')">
      <template #default="{ row }">
        {{ getServerName(row, 'data') }}
      </template>
    </el-table-column>
    <el-table-column label="媒体服务器" width="160" sortable :sort-by="(row) => getServerName(row, 'media')">
      <template #default="{ row }">
        {{ getServerName(row, 'media') }}
      </template>
    </el-table-column>
    <el-table-column prop="cron" label="调度配置" width="160" sortable :sort-by="(row) => row.cron || ''">
      <template #default="{ row }">
        <el-tag size="small" type="info">{{ row.cron || '-' }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column prop="cron" label="定时同步" width="110" sortable :sort-by="(row) => (row.cron ? 1 : 0)">
      <template #default="{ row }">
        <el-tag size="small" :type="row.cron ? 'success' : 'info'">
          {{ row.cron ? '启用' : '关闭' }}
        </el-tag>
      </template>
    </el-table-column>
    <el-table-column label="同步策略" width="120" sortable :sort-by="(row) => getSyncStrategy(row)">
      <template #default="{ row }">
        <el-tag size="small" type="info">{{ getSyncStrategy(row) }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="STRM 模式" width="140" sortable :sort-by="(row) => getStrmMode(row)">
      <template #default="{ row }">
        <el-tag size="small" type="info">{{ getStrmMode(row) }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column prop="source_path" label="媒体目录" width="180" sortable :sort-by="(row) => getMediaDir(row)">
      <template #default="{ row }">
        {{ getMediaDir(row) }}
      </template>
    </el-table-column>
    <el-table-column label="操作" width="220" fixed="right">
      <template #default="{ row }">
        <el-tooltip content="立即执行" placement="top">
          <el-button
            size="default"
            :icon="VideoPlay"
            class="action-button-large"
            :loading="actionLoading?.trigger?.[row.id]"
            :disabled="isRowActionPending(row.id)"
            @click="emit('trigger', row)"
          />
        </el-tooltip>
        <el-tooltip content="编辑任务" placement="top">
          <el-button
            size="default"
            :icon="Edit"
            class="action-button-large"
            :disabled="isRowActionPending(row.id)"
            @click="emit('edit', row)"
          />
        </el-tooltip>
        <el-tooltip :content="row.enabled ? '禁用任务' : '启用任务'" placement="top">
          <el-button
            size="default"
            :icon="getToggleIcon(row)"
            class="action-button-large"
            :loading="actionLoading?.toggle?.[row.id]"
            :disabled="isRowActionPending(row.id)"
            @click="emit('toggle', row)"
          />
        </el-tooltip>
        <el-tooltip content="删除任务" placement="top">
          <el-button
            size="default"
            type="danger"
            :icon="Delete"
            class="action-button-large"
            :disabled="isRowActionPending(row.id)"
            @click="emit('delete', row)"
          />
        </el-tooltip>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup>
import Delete from '~icons/ep/delete'
import Edit from '~icons/ep/edit'
import VideoPlay from '~icons/ep/video-play'

defineProps({
  jobList: {
    type: Array,
    default: () => []
  },
  loading: {
    type: Boolean,
    default: false
  },
  actionLoading: {
    type: Object,
    default: () => ({ trigger: {}, toggle: {} })
  },
  isRowActionPending: {
    type: Function,
    required: true
  },
  getServerName: {
    type: Function,
    required: true
  },
  getSyncStrategy: {
    type: Function,
    required: true
  },
  getStrmMode: {
    type: Function,
    required: true
  },
  getMediaDir: {
    type: Function,
    required: true
  },
  getToggleIcon: {
    type: Function,
    required: true
  }
})

const emit = defineEmits(['trigger', 'edit', 'toggle', 'delete', 'selection-change'])
</script>

<style scoped lang="scss">
.action-button-large {
  font-size: 14px;
  padding: 6px 8px;
}
</style>
