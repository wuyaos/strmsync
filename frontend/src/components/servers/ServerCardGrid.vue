<template>
  <div
    v-loading="loading"
    class="server-card-grid"
    role="listbox"
    :aria-multiselectable="batchMode"
    aria-label="服务器列表"
  >
    <el-card
      v-for="server in serverList"
      :key="server.id"
      shadow="hover"
      class="server-card"
      :class="{ 'is-selected': isSelected(server), 'is-batch-mode': batchMode }"
      tabindex="0"
      :aria-selected="isSelected(server)"
      role="option"
      :aria-label="`${server.name} - ${server.type} - ${server.enabled ? '已启用' : '已禁用'}`"
      @click="handleCardClick(server)"
      @keydown.enter="handleCardClick(server)"
      @keydown.space.prevent="handleCardClick(server)"
    >
      <div class="card-header">
        <div class="checkbox-wrapper">
          <el-checkbox
            :model-value="isSelected(server)"
            @click.stop
            @change="toggleSelect(server)"
          />
        </div>
        <div class="server-icon">
          <img
            v-if="getServerIconUrl(server)"
            :src="getServerIconUrl(server)"
            :alt="`${server.name} 图标`"
            class="server-icon-image"
          />
          <el-icon v-else :size="20">
            <component :is="getServerIcon(server.type)" />
          </el-icon>
        </div>
        <span class="server-name">{{ server.name }}</span>
        <span
          v-if="server.type !== 'local'"
          class="status-dot"
          :class="getConnectionStatus(server)"
          :title="server.enabled ? '已启用' : '已禁用'"
          :aria-label="server.enabled ? '已启用' : '已禁用'"
          role="status"
        />
      </div>
      <div class="card-body">
        <div class="info-row">
          <span class="label">类型：</span>
          <el-tag size="small">{{ server.type }}</el-tag>
        </div>
        <div class="info-row">
          <span class="label">地址：</span>
          <span class="value">{{ server.host || '-' }}{{ server.port ? ':' + server.port : '' }}</span>
        </div>
        <div class="info-row">
          <span class="label">状态：</span>
          <el-switch
            :model-value="server.enabled"
            size="small"
            @click.stop
            @change="(value) => handleToggleEnabled(server, value)"
          />
        </div>
        <div class="info-row">
          <span class="label">创建：</span>
          <span class="value">{{ server.created_at ? formatTime(server.created_at) : '-' }}</span>
        </div>
        <div class="card-divider"></div>
        <div class="card-actions">
          <el-button
            size="default"
            text
            :icon="Link"
            :disabled="server.type === 'local'"
            class="action-button"
            @click.stop="handleTest(server)"
          >
            测试
          </el-button>
          <el-button
            size="default"
            text
            :icon="Edit"
            class="action-button"
            @click.stop="handleEdit(server)"
          >
            编辑
          </el-button>
          <el-button
            size="default"
            text
            type="danger"
            :icon="Delete"
            class="action-button"
            @click.stop="handleDelete(server)"
          >
            删除
          </el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import Delete from '~icons/ep/delete'
import Edit from '~icons/ep/edit'
import Link from '~icons/ep/link'

defineProps({
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
  }
})
</script>

<style scoped lang="scss">
.server-card-grid {
  display: grid;
  gap: 16px;
  min-height: 200px;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
}

.server-card {
  transition: all 0.2s ease;
  cursor: pointer;
  border: 2px solid transparent;

  &:hover {
    transform: translateY(-2px);
    border-color: var(--el-color-primary-light-5);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }

  &:focus {
    outline: 2px solid var(--el-color-primary);
    outline-offset: 2px;
  }

  &.is-selected {
    border-color: var(--el-color-primary);
    background-color: var(--el-color-primary-light-9);
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--el-border-color-lighter);

    .checkbox-wrapper {
      opacity: 1;
      width: 24px;
      overflow: hidden;
      transition: all 0.2s ease;

      :deep(.el-checkbox) {
        pointer-events: auto;
      }
    }

    .server-icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 22px;
      height: 22px;
      color: var(--el-color-primary);
      flex-shrink: 0;
    }

    .server-icon-image {
      width: 20px;
      height: 20px;
      border-radius: 4px;
      object-fit: contain;
    }

    .server-name {
      font-weight: 500;
      font-size: 15px;
      color: var(--el-text-color-primary);
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .status-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      flex-shrink: 0;

      &.status-unknown {
        background-color: var(--el-color-info);
      }

      &.status-success,
      &.status-green {
        background-color: var(--el-color-success);
      }

      &.status-error,
      &.status-red {
        background-color: var(--el-color-danger);
      }

      &.status-disabled {
        background-color: var(--el-color-info-light-5);
      }
    }
  }

  .card-body {
    padding-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;

    .card-divider {
      height: 1px;
      background: var(--el-border-color-lighter);
      margin-top: 4px;
    }

    .info-row {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;

      .label {
        color: var(--el-text-color-secondary);
        min-width: 40px;
      }

      .value {
        color: var(--el-text-color-primary);
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }


    .card-actions {
      min-height: 36px;
      margin-top: 6px;
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 8px;
      align-items: center;
      justify-items: center;
    }

    .action-button {
      font-size: 14px;
      line-height: 1;
      padding: 6px 0;
      min-width: 64px;
      justify-content: center;
    }
  }

  &:hover .checkbox-wrapper,
  &.is-batch-mode .checkbox-wrapper,
  &.is-selected .checkbox-wrapper,
  &:focus .checkbox-wrapper {
    opacity: 1;
    width: 24px;
  }
}
</style>
