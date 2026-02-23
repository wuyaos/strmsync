<template>
  <el-card class="job-card mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">目录配置</div>
    </template>
    <div class="path-meta mb-12">
      <div class="text-12 text-[var(--el-text-color-secondary)] mb-8">路径信息</div>
      <div class="path-grid">
        <div class="path-item">
          <span class="path-label">访问目录</span>
          <span class="path-value">
            {{ accessPathText }}
          </span>
        </div>
        <div class="path-item">
          <span class="path-label">挂载目录</span>
          <span class="path-value">
            {{ mountPathText }}
          </span>
        </div>
        <div class="path-item">
          <span class="path-label">远程根目录</span>
          <span class="path-value">
            {{ remoteRootText }}
          </span>
        </div>
      </div>
    </div>
    <el-row :gutter="20" class="items-start">
      <el-col :xs="24" :md="12">
        <el-form-item label="媒体目录" prop="media_dir" class="compact-field">
          <template #label>
            <div class="flex items-center gap-8 w-full">
              <span class="whitespace-nowrap">媒体目录</span>
              <span class="label-sep"> </span>
              <span class="label-help ml-auto leading-5 max-w-[360px] text-right inline-flex items-center gap-4">
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
              <span class="label-sep"> </span>
              <span class="label-help ml-auto leading-5 max-w-[360px] text-right inline-flex items-center gap-4">用于 CD2/OpenList 的远程 API 根路径</span>
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
              <span class="label-sep"> </span>
              <span class="label-help ml-auto leading-5 max-w-[360px] text-right inline-flex items-center gap-4">用于存放生成的 STRM 文件和元数据</span>
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

    <el-form-item label="排除目录" prop="exclude_dirs" class="compact-field mt-20">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span class="whitespace-nowrap">排除目录</span>
          <span class="label-sep"> </span>
          <span class="label-help ml-auto leading-5 max-w-[360px] text-right inline-flex items-center gap-4">可多选，支持手动输入(用\",\"隔开)</span>
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

<style scoped lang="scss">
.job-card {
  --el-card-padding: 20px;
}

:deep(.el-card__header) {
  padding: var(--el-card-padding);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

:deep(.el-card__body) {
  padding: var(--el-card-padding);
}

.path-meta {
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
}

.path-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 8px 16px;
}

.path-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.path-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.path-value {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 13px;
  color: var(--el-text-color-primary);
  word-break: break-all;
}

.label-sep {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.label-help {
  margin-left: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

:deep(.el-form-item) {
  margin-bottom: 16px;
}

:deep(.el-form-item__label) {
  font-size: 14px;
  color: var(--el-text-color-regular);
  font-weight: 500;
  line-height: 1.2;
}

:deep(.el-form-item__label .ml-auto) {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  font-weight: normal;
}
</style>

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
