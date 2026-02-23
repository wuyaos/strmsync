<template>
  <el-card class="job-card mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">清除功能</div>
    </template>
    <el-checkbox-group v-model="formData.cleanup_opts" class="cleanup-grid">
      <div
        v-for="item in cleanupOptions"
        :key="item.value"
        class="cleanup-card"
      >
        <el-checkbox :value="item.value">{{ item.label }}</el-checkbox>
        <div class="text-12 text-[var(--el-text-color-secondary)] leading-6 mt-4 flex items-start gap-4">
          {{ item.help }}
        </div>
      </div>
    </el-checkbox-group>
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

.cleanup-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
  margin-bottom: 8px;
}

.cleanup-card {
  padding: 10px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--el-fill-color-blank);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.cleanup-card:hover {
  border-color: var(--el-color-primary-light-7);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-8);
}
</style>

<script setup>
defineProps({
  formData: {
    type: Object,
    required: true
  },
  cleanupOptions: {
    type: Array,
    default: () => []
  }
})
</script>
