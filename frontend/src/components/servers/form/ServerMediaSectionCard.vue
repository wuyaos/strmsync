<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="flex items-center justify-between gap-12">
        <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">连接信息</div>
        <div v-if="showTestButton" class="inline-flex items-center gap-8">
          <div
            v-if="testStatus !== 'idle'"
            class="inline-flex items-center gap-8 text-14 px-8"
            :class="[
              testStatus === 'running' ? 'text-[var(--el-color-primary)]' : '',
              testStatus === 'success' ? 'text-[var(--el-color-success)]' : '',
              testStatus === 'error' ? 'text-[var(--el-color-danger)]' : ''
            ]"
          >
            <el-icon v-if="testStatus === 'running'" class="is-loading">
              <Loading />
            </el-icon>
            <el-icon v-else-if="testStatus === 'success'">
              <CircleCheckFilled />
            </el-icon>
            <el-icon v-else-if="testStatus === 'error'">
              <CircleCloseFilled />
            </el-icon>
            <span>{{ testStatusText }}</span>
          </div>
          <el-tooltip
            :disabled="hasId"
            content="未保存的服务器将进行临时测试"
            placement="top"
          >
            <span>
              <el-button
                :icon="Link"
                :loading="testing"
                :disabled="!canTest"
                @click="handleTestConnection"
              >
                测试
              </el-button>
            </span>
          </el-tooltip>
        </div>
      </div>
    </template>

    <div class="w-full">
      <el-row :gutter="12">
        <el-col :span="14">
          <el-form-item label="主机地址" prop="host" class="compact-field">
            <el-input
              v-model="formData.host"
              :placeholder="hostPlaceholder"
              clearable
            />
          </el-form-item>
        </el-col>
        <el-col :span="10">
          <el-form-item label="端口号" prop="port" class="compact-field">
            <el-input
              v-model.number="formData.port"
              type="number"
              :min="1"
              :max="65535"
              :step="1"
              class="input-short"
            />
          </el-form-item>
        </el-col>
      </el-row>
    </div>

    <el-form-item v-if="needsApiKey" :label="apiKeyLabel" prop="api_key" class="compact-field">
      <template #label>
        <div class="flex items-center gap-8 w-full">
          <span>{{ apiKeyLabel }}</span>
          <span
            v-if="serverTypeHint"
            class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[320px] text-right inline-flex items-center gap-4"
          >
            {{ serverTypeHint }}
          </span>
        </div>
      </template>
      <el-input
        v-model="formData.api_key"
        type="password"
        show-password
        :placeholder="`请输入${apiKeyLabel}`"
        clearable
      />
    </el-form-item>
  </el-card>
</template>

<script setup>
import CircleCheckFilled from '~icons/ep/circle-check-filled'
import CircleCloseFilled from '~icons/ep/circle-close-filled'
import Link from '~icons/ep/link'
import Loading from '~icons/ep/loading'

defineProps({
  formData: {
    type: Object,
    required: true
  },
  hostPlaceholder: {
    type: String,
    default: ''
  },
  needsApiKey: {
    type: Boolean,
    default: false
  },
  apiKeyLabel: {
    type: String,
    default: 'API Key'
  },
  serverTypeHint: {
    type: String,
    default: ''
  },
  showTestButton: {
    type: Boolean,
    default: false
  },
  testStatus: {
    type: String,
    default: 'idle'
  },
  testStatusText: {
    type: String,
    default: ''
  },
  testing: {
    type: Boolean,
    default: false
  },
  canTest: {
    type: Boolean,
    default: false
  },
  hasId: {
    type: Boolean,
    default: false
  },
  handleTestConnection: {
    type: Function,
    required: true
  }
})
</script>
