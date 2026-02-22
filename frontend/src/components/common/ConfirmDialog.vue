<template>
  <el-dialog
    :model-value="modelValue"
    :title="title"
    width="440px"
    center
    class="confirm-dialog"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    @close="handleCancel"
  >
    <div class="confirm-body">
      <div v-if="message" class="message">{{ message }}</div>
      <div v-if="hasItems" class="items">
        <div class="items-title">涉及对象：</div>
        <ul>
          <li v-for="item in previewItems" :key="item">{{ item }}</li>
        </ul>
        <div v-if="hasMore" class="items-more">等 {{ items.length }} 项</div>
      </div>
    </div>
    <template #footer>
      <el-button @click="handleCancel">{{ cancelText }}</el-button>
      <el-button :type="buttonType" @click="handleConfirm">{{ confirmText }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  title: {
    type: String,
    default: '确认操作'
  },
  message: {
    type: String,
    default: ''
  },
  items: {
    type: Array,
    default: () => []
  },
  maxItems: {
    type: Number,
    default: 6
  },
  confirmText: {
    type: String,
    default: '确认'
  },
  cancelText: {
    type: String,
    default: '取消'
  },
  type: {
    type: String,
    default: 'info'
  }
})

const emit = defineEmits(['update:modelValue', 'confirm', 'cancel'])

const hasItems = computed(() => Array.isArray(props.items) && props.items.length > 0)
const previewItems = computed(() => props.items.slice(0, props.maxItems))
const hasMore = computed(() => props.items.length > props.maxItems)
const buttonType = computed(() => (props.type === 'error' ? 'danger' : props.type))

const handleConfirm = () => {
  emit('update:modelValue', false)
  emit('confirm')
}

const handleCancel = () => {
  emit('update:modelValue', false)
  emit('cancel')
}
</script>

<style scoped lang="scss">
.confirm-dialog {
  :deep(.el-dialog__header) {
    padding: 16px 20px 0;
    margin-right: 0;
  }

  :deep(.el-dialog__title) {
    font-size: 16px;
    font-weight: 600;
    color: var(--el-text-color-primary);
  }

  :deep(.el-dialog__body) {
    padding: 12px 20px 16px;
  }

  :deep(.el-dialog__footer) {
    padding: 0 20px 16px;
  }
}

.confirm-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  color: var(--el-text-color-regular);
}

.message {
  font-size: 14px;
}

.items {
  background: var(--el-fill-color-light);
  border-radius: 6px;
  padding: 10px 12px;
  font-size: 13px;

  .items-title {
    font-weight: 600;
    margin-bottom: 6px;
    color: var(--el-text-color-primary);
  }

  ul {
    margin: 0;
    padding-left: 16px;
  }

  li {
    line-height: 1.6;
  }

  .items-more {
    margin-top: 6px;
    color: var(--el-text-color-secondary);
  }
}
</style>
