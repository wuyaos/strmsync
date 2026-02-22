<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">STRM 配置</div>
    </template>
    <el-form-item label="STRM 模式" class="compact-field">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span class="whitespace-nowrap">STRM 模式</span>
          <span
            v-if="!currentServerSupportsUrl"
            class="ml-auto text-12 text-[var(--el-color-warning)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4"
          >
            当前数据服务器不支持 URL 访问模式
          </span>
          <span
            v-else-if="forceRemoteOnly"
            class="ml-auto text-12 text-[var(--el-color-warning)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4"
          >
            OpenList 未配置访问目录时仅支持远程 URL
          </span>
        </div>
      </template>
      <el-radio-group v-model="formData.strm_mode" :disabled="!currentServerSupportsUrl">
        <el-radio value="local" :disabled="forceRemoteOnly">本地路径</el-radio>
        <el-radio value="url">远程 URL</el-radio>
      </el-radio-group>
    </el-form-item>
    <el-row :gutter="20" class="items-start">
      <el-col :xs="24" :md="12">
        <el-form-item>
          <template #label>
            <span class="text-14 font-medium text-[var(--el-text-color-regular)]">媒体文件大小阈值</span>
            <el-tooltip content="小于此大小的文件将直接复制/下载，而不是生成 STRM" placement="top">
              <el-icon class="ml-4 text-[var(--el-text-color-secondary)]"><InfoFilled /></el-icon>
            </el-tooltip>
          </template>
          <div class="flex items-center gap-8">
            <el-input v-model="formData.min_file_size" class="input-short" type="number" min="0" step="100" />
            <span class="text-[var(--el-text-color-regular)]">MB</span>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="items-start">
      <el-col :xs="24" :md="24">
        <el-form-item>
          <template #label>
            <span class="text-14 font-medium text-[var(--el-text-color-regular)]">STRM 替换规则</span>
            <el-tooltip content="按顺序依次替换，支持路径与URL" placement="top">
              <el-icon class="ml-4 text-[var(--el-text-color-secondary)]"><InfoFilled /></el-icon>
            </el-tooltip>
          </template>
          <div class="flex flex-col gap-8 w-full">
            <div v-if="!formData.strm_replace_rules?.length" class="text-12 text-[var(--el-text-color-secondary)] leading-6 mt-4 flex items-start gap-4">暂无规则</div>
            <div
              v-for="(rule, index) in formData.strm_replace_rules"
              :key="index"
              class="grid grid-cols-[1fr_1fr_auto] gap-8 items-center"
            >
              <el-input v-model="rule.from" placeholder="原始字符串" class="w-full" />
              <el-input v-model="rule.to" placeholder="替换字符串" class="w-full" />
              <div class="flex items-center gap-4">
                <el-button
                  link
                  :icon="ArrowUp"
                  :disabled="index === 0"
                  @click="moveStrmRule(index, -1)"
                />
                <el-button
                  link
                  :icon="ArrowDown"
                  :disabled="index === formData.strm_replace_rules.length - 1"
                  @click="moveStrmRule(index, 1)"
                />
                <el-button link :icon="Delete" @click="removeStrmRule(index)" />
              </div>
            </div>
            <el-button size="small" type="primary" plain @click="addStrmRule">添加规则</el-button>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <el-form-item label="媒体文件后缀" class="compact-field">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span class="whitespace-nowrap">媒体文件后缀</span>
          <span class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4">媒体文件扩展名白名单</span>
        </div>
      </template>
      <el-select
        v-model="formData.media_exts"
        multiple
        filterable
        allow-create
        default-first-option
        :loading="extsLoading"
        placeholder="添加后缀（.mkv, .mp4）"
        class="w-full"
      />
    </el-form-item>

    <el-form-item label="元数据后缀" class="compact-field">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span class="whitespace-nowrap">元数据后缀</span>
          <span class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4">元数据文件扩展名白名单</span>
        </div>
      </template>
      <el-select
        v-model="formData.meta_exts"
        multiple
        filterable
        allow-create
        default-first-option
        :loading="extsLoading"
        placeholder="添加后缀（.nfo, .jpg）"
        class="w-full"
      />
    </el-form-item>
  </el-card>
</template>

<script setup>
import ArrowDown from '~icons/ep/arrow-down'
import ArrowUp from '~icons/ep/arrow-up'
import Delete from '~icons/ep/delete'
import InfoFilled from '~icons/ep/info-filled'

defineProps({
  formData: {
    type: Object,
    required: true
  },
  currentServerSupportsUrl: {
    type: Boolean,
    default: false
  },
  forceRemoteOnly: {
    type: Boolean,
    default: false
  },
  extsLoading: {
    type: Boolean,
    default: false
  },
  addStrmRule: {
    type: Function,
    required: true
  },
  removeStrmRule: {
    type: Function,
    required: true
  },
  moveStrmRule: {
    type: Function,
    required: true
  }
})
</script>
