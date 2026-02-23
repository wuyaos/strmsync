<template>
  <el-card class="job-card mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">同步功能</div>
    </template>

    <!-- Partition 1: Schedule -->
    <div class="section-title">定时设置</div>
    <el-divider class="section-divider" />
    <el-row :gutter="20" class="items-start schedule-row">
      <el-col :xs="24" :sm="12" :md="8">
        <el-form-item label="定时同步" class="schedule-item">
          <template #label>
            <div class="flex items-center gap-8 w-full">
              <span class="whitespace-nowrap">定时同步</span>
              <span class="label-sep">   </span>
            </div>
          </template>
          <div class="flex items-center gap-12">
            <el-switch v-model="formData.schedule_enabled" />
          </div>
        </el-form-item>
      </el-col>
      <el-col :xs="24" :sm="12" :md="8">
        <el-form-item label="Cron 表达式" prop="cron" class="compact-field cron-item">
          <template #label>
            <div class="flex items-center gap-8 w-full">
              <span class="whitespace-nowrap">Cron 表达式</span>
              <span class="label-sep">   </span>
            </div>
          </template>
          <el-input v-model="formData.cron" placeholder="0 */6 * * *" />
        </el-form-item>
      </el-col>
    </el-row>

    <!-- Partition 2: Sync Strategy -->
    <div class="section-title">同步策略</div>
    <el-divider class="section-divider" />
    <el-row :gutter="20">
      <el-col v-for="item in syncOptionGroups.general" :key="item.value" :xs="24" :sm="12" :md="8">
        <div class="option-item">
          <el-radio v-model="syncStrategyModel" :value="item.value">{{ item.label }}</el-radio>
          <div class="option-help">{{ item.help }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- Partition 3: Meta Strategy -->
    <div class="section-title">元数据更新策略</div>
    <el-divider class="section-divider" />
    <el-row :gutter="20">
      <el-col v-for="item in syncOptionGroups.meta" :key="item.value" :xs="24" :sm="12" :md="8">
        <div class="option-item">
          <el-radio v-model="metaStrategyModel" :value="item.value">{{ item.label }}</el-radio>
          <div class="option-help">{{ item.help }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- Partition 3: Metadata Mode -->
    <div class="section-title">元数据模式</div>
    <el-divider class="section-divider" />
    <el-row :gutter="20" class="items-start">
      <el-col :xs="24" :sm="12" :md="8">
        <div class="option-item">
          <el-radio v-model="formData.metadata_mode" value="download" :disabled="currentServerIsLocal">
            API 下载
          </el-radio>
          <div class="option-help">优先 API 下载，失败回退复制</div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="12" :md="8">
        <div class="option-item">
          <el-radio v-model="formData.metadata_mode" value="copy" :disabled="forceRemoteOnly">
            系统复制
          </el-radio>
          <div class="option-help">从挂载盘直接复制</div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="12" :md="8">
        <div class="option-item">
          <el-radio v-model="formData.metadata_mode" value="none" :disabled="forceRemoteOnly">
            不处理
          </el-radio>
          <div class="option-help">跳过所有元数据文件</div>
        </div>
      </el-col>
    </el-row>

    <!-- Partition 5: Cleanup -->
    <div class="section-title">清除策略</div>
    <el-divider class="section-divider" />
    <el-checkbox-group v-model="formData.cleanup_opts">
      <el-row :gutter="20">
        <el-col v-for="item in cleanupOptions" :key="item.value" :xs="24" :sm="12" :md="8">
          <div class="option-item">
            <el-checkbox :value="item.value">{{ item.label }}</el-checkbox>
            <div class="option-help">{{ item.help }}</div>
          </div>
        </el-col>
      </el-row>
    </el-checkbox-group>
  </el-card>
</template>

<style scoped lang="scss">
.job-card {
  --el-card-padding: 24px 20px;
  --option-min-width: 200px;

  :deep(.el-card__header) {
    padding: var(--el-card-padding);
    border-bottom: 1px solid var(--el-border-color-lighter);
  }
}

.section-title {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-regular);
}

.section-divider {
  margin: 0 0 12px;
}

.option-item {
  width: 100%;
  padding: 4px 0;
}

.option-item :deep(.el-radio),
.option-item :deep(.el-checkbox) {
  min-width: var(--option-min-width);
}

.option-help {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
  padding-left: 24px;
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

.schedule-item {
  margin-top: 12px;
}

.schedule-row :deep(.el-form-item__label) {
  align-items: flex-start;
}

.cron-item :deep(.el-form-item__label) {
  align-items: center;
}

.cron-help {
  margin: 12px 0 0 0;
}

.el-form-item {
  margin-bottom: 18px;

  :deep(.el-form-item__label) {
    font-size: 14px;
    color: var(--el-text-color-regular);
    font-weight: 500;
    display: flex;
    align-items: center; /* Align label and asterisk vertically */
    line-height: 1; /* Consistent line height */
    margin-bottom: 4px; /* Space below label */
  }

  :deep(.el-item.is-required .el-form-item__label:before) {
    align-self: center; /* Vertically center the asterisk */
    margin-right: 4px;
  }

  /* Style for help text within form items (e.g., div.text-12) */
  .text-12 {
    font-size: 12px;
    color: var(--el-text-color-secondary);
    line-height: 1.5;
  }
}
</style>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  formData: {
    type: Object,
    required: true
  },
  syncOptionGroups: {
    type: Object,
    required: true
  },
  cleanupOptions: {
    type: Array,
    default: () => []
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
