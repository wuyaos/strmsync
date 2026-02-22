<template>
  <el-row :gutter="32" class="dashboard-row">
    <el-col :xs="24" :md="24">
      <el-card shadow="hover">
        <template #header>
          <div class="card-header">
            <span>最近运行记录</span>
            <el-button text @click="$router.push('/runs')">
              查看全部 <el-icon><ArrowRight /></el-icon>
            </el-button>
          </div>
        </template>

        <el-empty v-if="runList.length === 0" description="暂无运行记录" />

        <el-timeline v-else>
          <el-timeline-item
            v-for="run in runList"
            :key="run.id"
            :type="getRunType(run.status)"
          >
            <div class="task-item">
              <div class="task-name">{{ run.job_name || run.job?.name || '未命名任务' }}</div>
              <div class="task-meta">
                {{ formatTime(run.started_at || run.created_at) }}
              </div>
            </div>
          </el-timeline-item>
        </el-timeline>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup>
import ArrowRight from '~icons/ep/arrow-right'

defineProps({
  runList: {
    type: Array,
    default: () => []
  },
  getRunType: {
    type: Function,
    required: true
  },
  formatTime: {
    type: Function,
    required: true
  }
})
</script>

<style scoped lang="scss">
.dashboard-row {
  row-gap: 12px;
}

.task-item {
  .task-name {
    font-weight: 500;
    margin-bottom: 4px;
  }

  .task-meta {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
}
</style>
