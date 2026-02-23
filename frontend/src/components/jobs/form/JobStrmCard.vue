<template>
  <el-card class="job-card mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">STRM 配置</div>
    </template>
    <el-row :gutter="20" class="items-start">
      <el-col :xs="24" :md="12">
        <el-form-item label="STRM 模式" class="compact-field">
          <template #label>
            <div class="flex items-center gap-8 w-full">
              <span class="whitespace-nowrap">STRM 模式</span>
          <span class="label-sep">   </span>
              <span
                v-if="forceRemoteOnly"
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
      </el-col>
      <el-col :xs="24" :md="12">
        <el-form-item label="媒体文件大小阈值" class="compact-field">
          <template #label>
            <div class="flex items-center gap-8 w-full">
              <span class="whitespace-nowrap">媒体文件大小阈值</span>
              <span class="label-sep">   </span>
              <span class="label-help">小于阈值将直接复制</span>
            </div>
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
            <el-table
              :data="formData.strm_replace_rules"
              :show-header="true"
              empty-text="暂无规则"
              border
              class="w-full strm-rule-table"
            >
              <el-table-column label="原始字符串 (From)">
                <template #default="{ row }">
                  <el-input v-model="row.from" placeholder="原始字符串" class="w-full" />
                </template>
              </el-table-column>
              <el-table-column label="替换字符串 (To)">
                <template #default="{ row }">
                  <el-input v-model="row.to" placeholder="替换字符串" class="w-full" />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="160px" fixed="right" align="center">
                <template #default="{ $index }">
                  <el-button
                    link
                    :icon="ArrowUp"
                    :disabled="$index === 0"
                    @click="moveStrmRule($index, -1)"
                  />
                  <el-button
                    link
                    :icon="ArrowDown"
                    :disabled="$index === formData.strm_replace_rules.length - 1"
                    @click="moveStrmRule($index, 1)"
                  />
                  <el-button link :icon="Delete" @click="removeStrmRule($index)" />
                </template>
              </el-table-column>
            </el-table>
            <el-button size="small" type="primary" plain @click="addStrmRule">添加规则</el-button>
          </div>
        </el-form-item>
      </el-col>
    </el-row>

    <el-form-item label="媒体文件后缀" class="compact-field">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span class="whitespace-nowrap">媒体文件后缀</span>
          <span class="label-sep">  </span>
          <span class="label-help ml-auto leading-5 max-w-[360px] text-right inline-flex items-center gap-4">媒体文件扩展名白名单</span>
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
          <span class="label-sep">  </span>
          <span class="label-help ml-auto leading-5 max-w-[360px] text-right inline-flex items-center gap-4">元数据文件扩展名白名单</span>
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

:deep(.el-form-item) {
  margin-bottom: 18px;
}

:deep(.el-form-item__label) {
  font-size: 14px;
  color: var(--el-text-color-regular);
  font-weight: 500;
  line-height: 1.2;
}

:deep(.strm-rule-table .el-table__header th) {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-regular);
}

:deep(.strm-rule-table .el-table__body td) {
  padding: 8px 10px;
}

:deep(.input-short) {
  width: 120px;
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
</style>

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
