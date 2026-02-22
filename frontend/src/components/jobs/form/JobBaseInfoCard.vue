<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">基本信息</div>
    </template>
    <el-row :gutter="20" class="items-start">
      <el-col :xs="24" :md="8">
        <el-form-item label="任务名称" prop="name">
          <el-input v-model="formData.name" placeholder="每周电影同步" />
        </el-form-item>
      </el-col>
      <el-col :xs="24" :md="8">
        <el-form-item label="数据服务器" prop="data_server_id">
          <el-select
            v-model="formData.data_server_id"
            placeholder="选择数据服务器"
            class="w-full"
            @change="handleServerChange"
          >
            <el-option
              v-for="server in dataServerOptions"
              :key="server.id"
              :label="server.label"
              :value="server.id"
            />
          </el-select>
        </el-form-item>
      </el-col>
      <el-col :xs="24" :md="8">
        <el-form-item prop="media_server_id">
          <template #label>
            <span>媒体服务器（可选）</span>
            <el-tooltip content="用于后续媒体库联动，不影响同步流程" placement="top">
              <el-icon class="ml-4 text-[var(--el-text-color-secondary)]"><InfoFilled /></el-icon>
            </el-tooltip>
          </template>
          <el-select
            v-model="formData.media_server_id"
            placeholder="可选：选择媒体服务器"
            clearable
            class="w-full"
          >
            <el-option
              v-for="server in mediaServerOptions"
              :key="server.id"
              :label="server.label"
              :value="server.id"
            />
          </el-select>
        </el-form-item>
      </el-col>
    </el-row>
  </el-card>
</template>

<script setup>
import InfoFilled from '~icons/ep/info-filled'

defineProps({
  formData: {
    type: Object,
    required: true
  },
  dataServerOptions: {
    type: Array,
    default: () => []
  },
  mediaServerOptions: {
    type: Array,
    default: () => []
  },
  handleServerChange: {
    type: Function,
    required: true
  }
})
</script>
