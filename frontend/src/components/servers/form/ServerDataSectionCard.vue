<template>
  <el-card class="mb-16 border-[var(--el-border-color-lighter)]" shadow="never">
    <template #header>
      <div class="flex items-center justify-between gap-12">
        <div class="text-16 font-semibold text-[var(--el-text-color-primary)]">{{ section.label }}</div>
        <div v-if="section.id === 'auth' && showTestButton" class="inline-flex items-center gap-8">
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

    <div v-if="section.layout === 'row'" class="w-full">
      <el-row :gutter="12">
        <el-col
          v-for="field in getVisibleFields(section.fields)"
          :key="field.name"
          :span="field.col_span || 12"
        >
          <el-form-item
            v-if="field.type !== 'hidden'"
            :label="field.label"
            :prop="field.name"
            class="compact-field"
          >
            <template #label>
              <div class="flex items-center gap-8 w-full">
                <span>{{ field.label }}</span>
                <span
                  v-if="field.help"
                  class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[320px] text-right inline-flex items-center gap-4"
                >
                  {{ field.help }}
                </span>
              </div>
            </template>
            <el-input
              v-if="isPathField(field)"
              v-model="dynamicModel[field.name]"
              :placeholder="field.placeholder"
            >
              <template #suffix>
                <el-button link :icon="FolderOpened" @click="openPathDialog(field)" />
              </template>
            </el-input>
            <el-input
              v-else-if="isTextField(field)"
              v-model="dynamicModel[field.name]"
              :placeholder="field.placeholder"
              clearable
            />
            <el-input
              v-else-if="field.type === 'password'"
              v-model="dynamicModel[field.name]"
              type="password"
              show-password
              :placeholder="field.placeholder"
              clearable
            />
            <el-input
              v-else-if="field.type === 'number'"
              v-model.number="dynamicModel[field.name]"
              :placeholder="field.placeholder"
              type="number"
              :min="field.min ?? 1"
              :max="field.max ?? 65535"
              class="input-short"
            />
            <el-select
              v-else-if="field.type === 'select'"
              v-model="dynamicModel[field.name]"
              placeholder="请选择"
              class="w-full"
            >
              <el-option
                v-for="option in field.options || []"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              />
            </el-select>
            <el-radio-group
              v-else-if="field.type === 'radio'"
              v-model="dynamicModel[field.name]"
            >
              <el-radio
                v-for="option in field.options || []"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </el-radio>
            </el-radio-group>
            <el-input
              v-else
              v-model="dynamicModel[field.name]"
              :placeholder="field.placeholder"
              clearable
            />
          </el-form-item>
        </el-col>
      </el-row>
    </div>

    <template v-else>
      <el-form-item
        v-for="field in getVisibleFields(section.fields)"
        :key="field.name"
        v-show="field.type !== 'hidden'"
        :label="field.label"
        :prop="field.name"
        class="compact-field"
      >
        <template #label>
          <div class="flex items-center gap-8 w-full">
            <span>{{ field.label }}</span>
            <span
              v-if="field.help"
              class="ml-auto text-12 text-[var(--el-text-color-secondary)] leading-5 max-w-[320px] text-right inline-flex items-center gap-4"
            >
              {{ field.help }}
            </span>
          </div>
        </template>
        <el-input
          v-if="isPathField(field)"
          v-model="dynamicModel[field.name]"
          :placeholder="field.placeholder"
        >
          <template #suffix>
            <el-button link :icon="FolderOpened" @click="openPathDialog(field)" />
          </template>
        </el-input>
        <el-input
          v-else-if="isTextField(field)"
          v-model="dynamicModel[field.name]"
          :placeholder="field.placeholder"
          clearable
        />
        <el-input
          v-else-if="field.type === 'password'"
          v-model="dynamicModel[field.name]"
          type="password"
          show-password
          :placeholder="field.placeholder"
          clearable
        />
        <el-input
          v-else-if="field.type === 'number'"
          v-model.number="dynamicModel[field.name]"
          :placeholder="field.placeholder"
          type="number"
          :min="field.min ?? 1"
          :max="field.max ?? 65535"
          class="input-short"
        />
        <el-select
          v-else-if="field.type === 'select'"
          v-model="dynamicModel[field.name]"
          placeholder="请选择"
          class="w-full"
        >
          <el-option
            v-for="option in field.options || []"
            :key="option.value"
            :label="option.label"
            :value="option.value"
          />
        </el-select>
        <el-radio-group
          v-else-if="field.type === 'radio'"
          v-model="dynamicModel[field.name]"
        >
          <el-radio
            v-for="option in field.options || []"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </el-radio>
        </el-radio-group>
        <el-input
          v-else
          v-model="dynamicModel[field.name]"
          :placeholder="field.placeholder"
          clearable
        />
      </el-form-item>
    </template>
  </el-card>
</template>

<script setup>
import CircleCheckFilled from '~icons/ep/circle-check-filled'
import CircleCloseFilled from '~icons/ep/circle-close-filled'
import FolderOpened from '~icons/ep/folder-opened'
import Link from '~icons/ep/link'
import Loading from '~icons/ep/loading'

defineProps({
  section: {
    type: Object,
    required: true
  },
  dynamicModel: {
    type: Object,
    required: true
  },
  getVisibleFields: {
    type: Function,
    required: true
  },
  isPathField: {
    type: Function,
    required: true
  },
  isTextField: {
    type: Function,
    required: true
  },
  openPathDialog: {
    type: Function,
    required: true
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
