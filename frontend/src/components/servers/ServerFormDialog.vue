<template>
  <el-dialog
    v-model="visible"
    :title="dialogTitle"
    width="680px"
    destroy-on-close
    :close-on-click-modal="false"
    class="task-config-dialog"
  >
    <el-form
      ref="formRef"
      :model="formModel"
      :rules="formRules"
      label-position="top"
      label-width="var(--form-label-width)"
      class="compact-form"
    >
      <!-- 基础信息 -->
      <ServerBaseInfoCard
        :form-data="formData"
        :server-type-options="serverTypeOptions"
        :handle-name-input="handleNameInput"
        :handle-type-change="handleTypeChange"
      />

      <!-- 数据服务器：动态表单 -->
      <template v-if="isDataMode">
        <template v-if="activeTypeDef">
          <template v-for="section in activeTypeDef.sections" :key="section.id">
            <ServerDataSectionCard
              :section="section"
              :dynamic-model="dynamicModel"
              :get-visible-fields="getVisibleFields"
              :is-path-field="isPathField"
              :is-text-field="isTextField"
              :open-path-dialog="openPathDialog"
              :show-test-button="showTestButton"
              :test-status="testStatus"
              :test-status-text="testStatusText"
              :testing="testing"
              :can-test="canTest"
              :has-id="!!formData.id"
              :handle-test-connection="handleTestConnection"
            />
          </template>
        </template>
        <el-alert
          v-else
          title="请选择服务器类型"
          type="info"
          :closable="false"
        />
      </template>

      <!-- 媒体服务器：静态表单 -->
      <template v-else>
        <!-- 连接信息 -->
        <ServerMediaSectionCard
          :form-data="formData"
          :host-placeholder="hostPlaceholder"
          :needs-api-key="needsApiKey"
          :api-key-label="apiKeyLabel"
          :server-type-hint="serverTypeHint"
          :show-test-button="showTestButton"
          :test-status="testStatus"
          :test-status-text="testStatusText"
          :testing="testing"
          :can-test="canTest"
          :has-id="!!formData.id"
          :handle-test-connection="handleTestConnection"
        />
      </template>

      <!-- 高级选项（接口速率配置） -->
      <ServerRateSectionCard v-if="showRate" :form-data="formData" />
    </el-form>

    <template #footer>
      <div class="flex items-center justify-between">
        <div class="flex-none">
          <el-button @click="visible = false">取消</el-button>
        </div>
        <div class="flex items-center justify-end gap-12 flex-1">
          <!-- 创建/更新按钮 -->
          <el-button
            type="primary"
            :loading="saving"
            :icon="isEdit ? Edit : Plus"
            @click="handleSave"
          >
            {{ isEdit ? '更新' : '创建' }}
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>

  <PathDialog
    v-model="pathDlg.visible.value"
    :mode="pathDlg.mode.value"
    :loading="pathDlg.loading.value"
    :path="pathDlg.path.value"
    :rows="pathDlg.rows.value"
    :has-more="pathDlg.hasMore.value"
    :selected-name="pathDlg.selectedName.value"
    :selected-names="pathDlg.selectedNames.value"
    :at-root="pathDlg.atRoot.value"
    :refresh-key="pathDlg.refreshKey.value"
    @up="pathDlg.goUp"
    @to-root="pathDlg.goRoot"
    @jump="pathDlg.jump"
    @enter="(name) => pathDlg.enterDirectory(name)"
    @select="handlePathSelect"
    @refresh="() => pathDlg.load(pathDlg.path.value)"
    @load-more="pathDlg.loadMore"
    @confirm="handlePathConfirm"
  />
</template>

<script setup>
import Edit from '~icons/ep/edit'
import Plus from '~icons/ep/plus'
import PathDialog from '@/components/common/PathDialog.vue'
import ServerBaseInfoCard from '@/components/servers/form/ServerBaseInfoCard.vue'
import ServerDataSectionCard from '@/components/servers/form/ServerDataSectionCard.vue'
import ServerMediaSectionCard from '@/components/servers/form/ServerMediaSectionCard.vue'
import ServerRateSectionCard from '@/components/servers/form/ServerRateSectionCard.vue'
import { useServerFormDialog } from '@/composables/useServerFormDialog'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  mode: { type: String, default: 'data' },
  editingServer: { type: Object, default: null },
  dataTypeDefs: { type: Array, default: () => [] },
  mediaTypeOptions: { type: Array, default: () => [] },
  serverList: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:modelValue', 'saved'])

const {
  visible,
  isEdit,
  isDataMode,
  formRef,
  formData,
  dynamicModel,
  formModel,
  formRules,
  serverTypeOptions,
  dialogTitle,
  activeTypeDef,
  serverTypeHint,
  hostPlaceholder,
  needsApiKey,
  apiKeyLabel,
  showTestButton,
  testStatus,
  testStatusText,
  testing,
  canTest,
  showRate,
  saving,
  openPathDialog,
  handlePathSelect,
  handlePathConfirm,
  handleTestConnection,
  handleSave,
  handleTypeChange,
  handleNameInput,
  getVisibleFields,
  isPathField,
  isTextField,
  pathDlg
} = useServerFormDialog(props, emit)
</script>

<style scoped lang="scss">
.task-config-dialog {
  :deep(.el-form-item__label) {
    font-size: 14px;
    color: var(--el-text-color-regular);
    font-weight: 500;
    display: flex;
    align-items: center;
  }

  :deep(.el-form-item__label::before) {
    margin-right: 4px;
  }

  :deep(.el-form-item) {
    margin-bottom: 12px;
  }

  :deep(.compact-field .el-form-item__content) {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: nowrap;
  }

  :deep(.compact-field .el-input),
  :deep(.compact-field .el-select),
  :deep(.compact-field .el-input-number) {
    flex: 1;
    min-width: 0;
  }

  :deep(.compact-form .el-form-item) {
    position: relative;
  }

  :deep(.compact-form .el-form-item__error) {
    position: absolute;
    right: 0;
    top: 0;
    margin-top: 0;
    padding-top: 0;
    line-height: 1.2;
    font-size: 12px;
    text-align: right;
    max-width: 260px;
  }

  :deep(.no-label .el-form-item__label) {
    display: none;
  }
}

:deep(.el-divider__text) {
  font-weight: 500;
  color: var(--el-text-color-primary);
}
</style>
