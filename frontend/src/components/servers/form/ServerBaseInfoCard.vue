<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="flex items-center justify-between gap-12">
        <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">基础信息</div>
        <div class="inline-flex items-center gap-8">
          <el-switch v-model="formData.enabled" />
        </div>
      </div>
    </template>
    <el-row :gutter="12" class="items-start">
      <el-col :xs="24" :md="12">
        <el-form-item label="服务器名称" prop="name" class="compact-field">
          <el-input
            v-model="formData.name"
            placeholder="Emby"
            clearable
            @input="handleNameInput"
          />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-form-item label="服务器类型" prop="type" class="compact-field">
          <div class="flex items-center gap-8 w-full flex-nowrap">
            <el-select
              v-model="formData.type"
              filterable
              placeholder="选择服务器类型"
              class="flex-1 min-w-0"
              @change="handleTypeChange"
            >
              <el-option
                v-for="option in serverTypeOptions"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              >
                <span>{{ option.label }}</span>
                <span class="float-right text-13 text-[var(--el-text-color-secondary)] ml-16">
                  {{ option.description }}
                </span>
              </el-option>
            </el-select>
          </div>
        </el-form-item>
      </el-col>
    </el-row>
  </el-card>
</template>

<script setup>
defineProps({
  formData: {
    type: Object,
    required: true
  },
  serverTypeOptions: {
    type: Array,
    default: () => []
  },
  handleNameInput: {
    type: Function,
    required: true
  },
  handleTypeChange: {
    type: Function,
    required: true
  }
})
</script>
