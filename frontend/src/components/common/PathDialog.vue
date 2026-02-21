<template>
  <el-dialog
    v-model="visible"
    :title="title"
    width="640px"
    destroy-on-close
    :close-on-click-modal="false"
  >
    <div v-loading="loading" class="path-dialog">
      <div class="path-header">
        <span class="path-label">当前路径</span>
        <el-input
          v-model="pathInput"
          class="path-input"
          placeholder="/"
          @keyup.enter="handleJump"
        />
      </div>

      <el-table
        :data="rows"
        stripe
        highlight-current-row
        class="w-full mt-12"
        max-height="400px"
        @row-dblclick="(row) => emit('enter', row.name)"
      >
        <el-table-column width="52">
          <template #default="{ row }">
            <el-checkbox
              :model-value="isRowSelected(row.name)"
              @change="() => handleToggle(row.name)"
            />
          </template>
        </el-table-column>
        <el-table-column label="目录名称" prop="name">
          <template #default="{ row }">
            <div class="dir-name" :class="{ 'is-selected': isRowSelected(row.name) }">
              <el-icon><FolderOpened /></el-icon>
              <span>{{ row.name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column width="72" align="center">
          <template #default="{ row }">
            <el-button link :icon="Right" @click="emit('enter', row.name)" />
          </template>
        </el-table-column>
      </el-table>

      <div v-if="rows.length === 0" class="empty-hint">
        当前目录下没有子目录
      </div>

      <div class="path-actions">
        <div class="path-actions-left">
          <el-button :icon="ArrowLeft" :disabled="atRoot" @click="emit('up')">
            返回上级
          </el-button>
          <el-button :icon="HomeFilled" :disabled="atRoot" @click="emit('to-root')">
            根目录
          </el-button>
        </div>
        <el-button v-if="hasMore" :loading="loading" @click="emit('load-more')">
          加载更多
        </el-button>
        <el-button type="primary" @click="emit('confirm')">确认</el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { ArrowLeft, FolderOpened, HomeFilled, Right } from '@element-plus/icons-vue'
import { joinPath, normalizePath } from '@/composables/usePathDialog'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, default: '选择目录' },
  mode: { type: String, default: 'single' },
  path: { type: String, default: '/' },
  rows: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  hasMore: { type: Boolean, default: false },
  selectedName: { type: String, default: '' },
  selectedNames: { type: Array, default: () => [] },
  atRoot: { type: Boolean, default: false }
})

const emit = defineEmits([
  'update:modelValue',
  'up',
  'to-root',
  'select',
  'toggle',
  'enter',
  'jump',
  'load-more',
  'confirm'
])

const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const isMulti = computed(() => props.mode === 'multi')

const pathInput = ref(props.path || '/')

watch(
  () => props.path,
  (value) => {
    pathInput.value = value || '/'
  }
)

const normalizedSelectedName = computed(() => {
  if (!props.selectedName) return ''
  return normalizePath(props.selectedName)
})

const normalizedSelectedSet = computed(() => {
  return new Set((props.selectedNames || []).filter(Boolean).map(item => normalizePath(item)))
})

const getRowPath = (name) => {
  return normalizePath(joinPath(props.path || '/', name))
}

const isRowSelected = (name) => {
  const rowPath = getRowPath(name)
  if (isMulti.value) {
    return normalizedSelectedSet.value.has(rowPath)
  }
  return normalizedSelectedName.value === rowPath
}

const handleToggle = (name) => {
  if (isMulti.value) {
    emit('toggle', name)
    return
  }
  emit('select', name)
}

const handleJump = () => {
  emit('jump', pathInput.value)
}
</script>

<style scoped lang="scss">
.path-dialog {
  .path-header {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    padding: var(--space-8) 0;
  }

  .path-label {
    color: var(--el-text-color-secondary);
    font-size: var(--font-13);
    white-space: nowrap;
  }

  .path-input {
    flex: 1;
    font-family: 'Courier New', monospace;
  }

  .dir-name {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    cursor: pointer;

    .el-icon {
      color: var(--el-color-warning);
      font-size: 18px;
    }

    &.is-selected {
      font-weight: 600;
      color: var(--el-color-primary);

      .el-icon {
        color: var(--el-color-primary);
      }
    }
  }

  .empty-hint {
    text-align: center;
    color: var(--el-text-color-secondary);
    padding: 32px 0;
    font-size: var(--font-14);
  }

  .path-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-8);
    padding-top: var(--space-12);
  }

  .path-actions-left {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
}
</style>
