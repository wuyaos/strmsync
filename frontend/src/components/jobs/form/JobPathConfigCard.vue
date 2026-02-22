<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">目录配置</div>
    </template>
    <div class="flex flex-wrap items-center gap-16 text-12 text-[var(--el-text-color-secondary)] mb-12">
      <div>
        <span class="text-[var(--el-text-color-regular)]">访问目录：</span>
        <span class="font-mono text-[var(--el-text-color-primary)]">{{ accessPathText }}</span>
      </div>
      <div>
        <span class="text-[var(--el-text-color-regular)]">挂载目录：</span>
        <span class="font-mono text-[var(--el-text-color-primary)]">{{ mountPathText }}</span>
      </div>
      <div>
        <span class="text-[var(--el-text-color-regular)]">远程根目录：</span>
        <span class="font-mono text-[var(--el-text-color-primary)]">{{ remoteRootText }}</span>
      </div>
    </div>
    <el-row :gutter="20" class="items-start">
      <el-col :xs="24" :md="12">
        <el-form-item label="媒体目录" prop="media_dir" class="compact-field">
          <template #label>
            <div class="flex items-center gap-8 w-full">
              <span class="whitespace-nowrap">媒体目录</span>
              <span class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4">
                <span v-if="mediaDirDisabled">请先选择数据服务器。</span>
                <template v-else>
                  <el-icon v-if="showMediaDirWarning" class="text-[var(--el-color-warning)] mt-4"><WarningFilled /></el-icon>
                  默认为数据服务器的访问目录的根目录
                </template>
              </span>
            </div>
          </template>
          <el-input
            :model-value="mediaDirProxy"
            placeholder="movies"
            :disabled="mediaDirDisabled"
            @update:model-value="emit('update:mediaDirProxy', $event)"
          >
            <template #suffix>
              <el-button link :icon="FolderOpened" :disabled="mediaDirDisabled" @click="openPath('media_dir')" />
            </template>
          </el-input>
        </el-form-item>
      </el-col>
      <el-col v-if="currentServerHasApi" :xs="24" :md="12">
        <el-form-item label="远程根目录" prop="remote_root" class="compact-field">
          <template #label>
            <div class="flex items-center gap-8 w-full">
              <span class="whitespace-nowrap">远程根目录</span>
              <span class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4">用于 CD2/OpenList 的远程 API 根路径</span>
            </div>
          </template>
          <el-input v-model="formData.remote_root" placeholder="/">
            <template #suffix>
              <el-button link :icon="FolderOpened" @click="openPath('remote_root', { forceApi: true })" />
            </template>
          </el-input>
        </el-form-item>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-form-item label="本地输出" prop="local_dir" class="compact-field">
          <template #label>
            <div class="flex items-center gap-8 w-full">
              <span class="whitespace-nowrap">本地输出</span>
              <span class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4">用于存放生成的 STRM 文件和元数据</span>
            </div>
          </template>
          <el-input v-model="formData.local_dir" placeholder="/local/strm/movies">
            <template #suffix>
              <el-button link :icon="FolderOpened" @click="openPath('local_dir')" />
            </template>
          </el-input>
        </el-form-item>
      </el-col>
    </el-row>

    <el-form-item label="排除目录" prop="exclude_dirs" class="compact-field">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span class="whitespace-nowrap">排除目录</span>
          <span class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[360px] text-right inline-flex items-center gap-4">可多选，支持手动输入(用\",\"隔开)</span>
        </div>
      </template>
      <el-input
        :model-value="excludeDirsProxy"
        placeholder="temp, .trash"
        @update:model-value="emit('update:excludeDirsProxy', $event)"
      >
        <template #suffix>
          <el-button link :icon="FolderOpened" @click="openPath('exclude_dirs', { multiple: true })" />
        </template>
      </el-input>
    </el-form-item>
  </el-card>
</template>

<script setup>
import FolderOpened from '~icons/ep/folder-opened'
import WarningFilled from '~icons/ep/warning-filled'

const props = defineProps({
  formData: {
    type: Object,
    required: true
  },
  accessPathText: {
    type: String,
    default: ''
  },
  mountPathText: {
    type: String,
    default: ''
  },
  remoteRootText: {
    type: String,
    default: ''
  },
  mediaDirProxy: {
    type: String,
    default: ''
  },
  excludeDirsProxy: {
    type: String,
    default: ''
  },
  mediaDirDisabled: {
    type: Boolean,
    default: false
  },
  showMediaDirWarning: {
    type: Boolean,
    default: false
  },
  currentServerHasApi: {
    type: Boolean,
    default: false
  },
  openPath: {
    type: Function,
    required: true
  }
})

const emit = defineEmits(['update:mediaDirProxy', 'update:excludeDirsProxy'])
</script>
