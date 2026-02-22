<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">清除功能</div>
    </template>
    <div class="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-12 mb-8">
      <div
        v-for="item in cleanupOptions"
        :key="item.value"
        class="p-8 px-12 border border-[var(--el-border-color-lighter)] rounded-6 bg-[var(--el-fill-color-blank)]"
      >
        <el-checkbox v-model="formData.cleanup_opts" :label="item.value">
          {{ item.label }}
        </el-checkbox>
        <div class="text-12 text-[var(--el-text-color-secondary)] leading-6 mt-4 flex items-start gap-4">
          {{ item.help }}
        </div>
      </div>
    </div>
  </el-card>
</template>

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
