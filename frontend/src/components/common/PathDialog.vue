<template>
  <el-dialog
    v-model="visible"
    :title="title"
    width="640px"
    destroy-on-close
    :close-on-click-modal="false"
  >
    <div v-loading="loading">
      <div class="flex items-center gap-8 py-8">
        <span class="text-13 text-[var(--el-text-color-secondary)] whitespace-nowrap">当前路径</span>
        <el-input
          v-model="pathInput"
          class="flex-1 font-mono"
          placeholder="/"
          @keyup.enter="handleJump"
        />
      </div>

      <el-table
        :key="refreshKey"
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
            <div
              class="flex items-center gap-8 cursor-pointer"
              :class="isRowSelected(row.name) ? 'font-semibold text-[var(--el-color-primary)]' : ''"
            >
              <el-icon
                :class="[
                  'text-[18px]',
                  isRowSelected(row.name) ? 'text-[var(--el-color-primary)]' : 'text-[var(--el-color-warning)]'
                ]"
              >
                <FolderOpened />
              </el-icon>
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

      <div v-if="rows.length === 0" class="text-center text-14 text-[var(--el-text-color-secondary)] py-24">
        当前目录下没有子目录
      </div>

      <div class="flex items-center justify-between gap-8 pt-12">
        <div class="flex items-center gap-8">
          <el-button :icon="ArrowLeft" :disabled="atRoot" @click="emit('up')">
            返回上级
          </el-button>
          <el-button :icon="HomeFilled" :disabled="atRoot" @click="emit('to-root')">
            根目录
          </el-button>
          <el-button :icon="Refresh" :loading="loading" @click="emit('refresh')">
            刷新
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
import ArrowLeft from '~icons/ep/arrow-left'
import FolderOpened from '~icons/ep/folder-opened'
import HomeFilled from '~icons/ep/home-filled'
import Refresh from '~icons/ep/refresh'
import Right from '~icons/ep/right'
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
  atRoot: { type: Boolean, default: false },
  refreshKey: { type: Number, default: 0 }
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
  'refresh',
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
