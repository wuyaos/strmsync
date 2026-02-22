<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">同步功能</div>
    </template>

    <el-form-item label="定时同步">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span class="whitespace-nowrap">定时同步</span>
          <span class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4">启用后按 Cron 规则定时执行同步</span>
        </div>
      </template>
      <div class="flex items-center gap-12">
        <el-switch v-model="formData.schedule_enabled" />
      </div>
    </el-form-item>

    <el-form-item v-if="formData.schedule_enabled" label="Cron 表达式" prop="cron" class="compact-field">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span class="whitespace-nowrap">Cron 表达式</span>
          <span class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4">
            格式：分 时 日 月 周（例如：0 */6 * * * 表示每 6 小时执行一次）
          </span>
        </div>
      </template>
      <el-input v-model="formData.cron" placeholder="0 */6 * * *" />
    </el-form-item>

    <div class="text-14 font-semibold text-[var(--el-text-color-regular)] my-12">同步策略</div>
    <div class="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-12 mb-8">
      <div
        v-for="item in syncOptionGroups.general"
        :key="item.value"
        class="p-8 px-12 border border-[var(--el-border-color-lighter)] rounded-6 bg-[var(--el-fill-color-blank)]"
      >
        <el-radio v-model="syncStrategyModel" :value="item.value">{{ item.label }}</el-radio>
        <div class="text-12 text-[var(--el-text-color-secondary)] leading-6 mt-4 flex items-start gap-4">
          {{ item.help }}
        </div>
      </div>
    </div>
    <el-form-item v-if="currentServerHasApi" label="远程列表优先">
      <div class="flex items-center gap-12">
        <el-switch v-model="formData.prefer_remote_list" :disabled="forceRemoteOnly" />
        <span class="text-12 text-[var(--el-text-color-secondary)] leading-6 mt-4 flex items-start gap-4">
          优先使用远程 API 获取文件列表（更快），本地默认关闭
          <span v-if="forceRemoteOnly">（OpenList 未配置访问目录时强制开启）</span>
        </span>
      </div>
    </el-form-item>

    <div class="text-14 font-semibold text-[var(--el-text-color-regular)] my-12">元数据选项</div>
    <div class="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-12 mb-8">
      <div
        v-for="item in syncOptionGroups.meta"
        :key="item.value"
        class="p-8 px-12 border border-[var(--el-border-color-lighter)] rounded-6 bg-[var(--el-fill-color-blank)]"
      >
        <el-radio v-model="metaStrategyModel" :value="item.value">{{ item.label }}</el-radio>
        <div class="text-12 text-[var(--el-text-color-secondary)] leading-6 mt-4 flex items-start gap-4">
          {{ item.help }}
        </div>
      </div>
    </div>

    <el-row :gutter="20" class="items-start">
      <el-col :xs="24" :md="12">
        <el-form-item label="元数据模式" class="inline-field">
          <el-radio-group v-model="formData.metadata_mode">
            <el-radio value="copy" :disabled="forceRemoteOnly">复制文件</el-radio>
            <el-radio value="download" :disabled="currentServerIsLocal">下载文件</el-radio>
            <el-radio value="none" :disabled="forceRemoteOnly">不处理</el-radio>
          </el-radio-group>
          <div
            v-if="currentServerIsLocal && formData.metadata_mode === 'download'"
            class="text-12 text-[var(--el-color-warning)] mt-4 flex items-start gap-4 basis-full"
          >
            <el-icon class="text-[var(--el-color-warning)] mt-4"><WarningFilled /></el-icon>
            Local 模式不支持下载，已自动切换为复制模式
          </div>
        </el-form-item>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-form-item label="同步线程数" class="inline-field">
          <el-input v-model="formData.thread_count" class="input-short" type="number" min="1" max="16" step="1" />
        </el-form-item>
      </el-col>
    </el-row>
  </el-card>
</template>

<script setup>
import { computed } from 'vue'
import WarningFilled from '~icons/ep/warning-filled'

const props = defineProps({
  formData: {
    type: Object,
    required: true
  },
  syncOptionGroups: {
    type: Object,
    required: true
  },
  syncStrategy: {
    type: String,
    required: true
  },
  metaStrategy: {
    type: String,
    required: true
  },
  currentServerHasApi: {
    type: Boolean,
    default: false
  },
  currentServerIsLocal: {
    type: Boolean,
    default: false
  },
  forceRemoteOnly: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:syncStrategy', 'update:metaStrategy'])

const syncStrategyModel = computed({
  get: () => props.syncStrategy,
  set: (value) => emit('update:syncStrategy', value)
})

const metaStrategyModel = computed({
  get: () => props.metaStrategy,
  set: (value) => emit('update:metaStrategy', value)
})
</script>
