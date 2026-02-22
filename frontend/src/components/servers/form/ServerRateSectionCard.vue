<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">高级选项</div>
    </template>
    <el-collapse>
      <el-collapse-item title="接口速率配置（可选）" name="rate">
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="下载队列每秒处理数量" class="compact-field">
              <template #label>
                <div class="flex items-center gap-8 w-full">
                  <span>下载队列每秒处理数量</span>
                  <span
                    class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[320px] text-right inline-flex items-center gap-4"
                  >
                    0 表示使用系统设置
                  </span>
                </div>
              </template>
              <el-input
                v-model.number="formData.download_rate_per_sec"
                type="number"
                :min="0"
                :max="1000"
                class="input-short"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="接口速率(每秒请求数)" class="compact-field">
              <template #label>
                <div class="flex items-center gap-8 w-full">
                  <span>接口速率(每秒请求数)</span>
                  <span
                    class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[320px] text-right inline-flex items-center gap-4"
                  >
                    0 表示使用系统设置
                  </span>
                </div>
              </template>
              <el-input
                v-model.number="formData.api_rate"
                type="number"
                :min="0"
                :max="1000"
                class="input-short"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="接口重试次数" class="compact-field">
              <template #label>
                <div class="flex items-center gap-8 w-full">
                  <span>接口重试次数</span>
                  <span
                    class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[320px] text-right inline-flex items-center gap-4"
                  >
                    0 表示使用系统设置
                  </span>
                </div>
              </template>
              <el-input
                v-model.number="formData.api_retry_max"
                type="number"
                :min="0"
                :max="10"
                class="input-short"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="接口重试间隔(秒)" class="compact-field">
              <template #label>
                <div class="flex items-center gap-8 w-full">
                  <span>接口重试间隔(秒)</span>
                  <span
                    class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[320px] text-right inline-flex items-center gap-4"
                  >
                    0 表示使用系统设置
                  </span>
                </div>
              </template>
              <el-input
                v-model.number="formData.api_retry_interval_sec"
                type="number"
                :min="0"
                :max="60"
                class="input-short"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-collapse-item>
    </el-collapse>
  </el-card>
</template>

<script setup>
defineProps({
  formData: {
    type: Object,
    required: true
  }
})
</script>
