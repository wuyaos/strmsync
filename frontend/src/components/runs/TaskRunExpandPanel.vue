<template>
  <div class="run-expand-panel">
    <div class="section-title">任务配置</div>
    <div class="section-body">
      <div class="config-card">
        <div v-if="compactRows.length" class="config-compact-grid">
          <div
            v-for="item in compactRows"
            :key="item.label"
            class="config-item compact"
          >
            <span class="label">{{ item.label }}</span>
            <span class="value">{{ item.value }}</span>
          </div>
        </div>
        <div v-if="normalRows.length" class="config-list">
          <div
            v-for="item in normalRows"
            :key="item.label"
            class="config-item"
          >
            <span class="label">{{ item.label }}</span>
            <span
              class="value"
              :class="[
                item.mono ? 'font-mono' : '',
                item.singleLine ? 'truncate' : ''
              ]"
              :title="item.title || ''"
            >
              <template v-if="item.pre">
                <pre class="m-0 whitespace-pre-wrap max-h-[160px] overflow-auto">{{ item.value }}</pre>
              </template>
              <template v-else>{{ item.value }}</template>
            </span>
          </div>
        </div>
        <div class="divider"></div>
        <div class="config-list">
          <div
            v-for="item in getSummaryRows(row)"
            :key="item.label"
            class="config-item"
          >
            <span class="label">{{ item.label }}</span>
            <span class="value break-all">{{ item.value }}</span>
          </div>
        </div>
        <div class="divider"></div>
        <div class="config-list">
          <div class="config-item">
            <span class="label">错误信息</span>
            <span class="value break-all">
              {{ row.error_message || row.error || '无' }}
            </span>
          </div>
        </div>
      </div>
    </div>
    <div class="section-title section-title--spaced">执行情况</div>
    <div class="section-body">
      <RunExecutionDetail :run-id="row.id" />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import RunExecutionDetail from '@/components/runs/RunExecutionDetail.vue'

const props = defineProps({
  row: {
    type: Object,
    required: true
  },
  getJobConfigRows: {
    type: Function,
    required: true
  },
  getSummaryRows: {
    type: Function,
    required: true
  }
})

const configRows = computed(() => props.getJobConfigRows(props.row) || [])
const compactRows = computed(() => configRows.value.filter((item) => item.compact))
const normalRows = computed(() => configRows.value.filter((item) => !item.compact))
</script>

<style scoped lang="scss">
.run-expand-panel {
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 6px;

  .section-title {
    font-weight: 600;
    margin-bottom: 10px;
    color: var(--el-text-color-primary);

    &.section-title--spaced {
      margin-top: 16px;
    }
  }

  .section-body {
    color: var(--el-text-color-regular);
  }

  .config-card {
    padding: 10px 12px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
    background: var(--el-fill-color-blank);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .config-compact-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 8px 16px;
  }

  .config-list {
    display: grid;
    gap: 8px;
  }

  .config-item {
    display: grid;
    grid-template-columns: 110px 1fr;
    gap: 8px;
    align-items: start;

    &.compact {
      grid-template-columns: 90px 1fr;
    }

    .label {
      color: var(--el-text-color-secondary);
    }

    .value {
      break-all: break-all;
    }
  }

  .divider {
    height: 1px;
    background: var(--el-border-color-lighter);
    margin: 10px 0;
  }
}
</style>
