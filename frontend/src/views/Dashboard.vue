<template>
  <div class="dashboard">
    <h1>仪表盘</h1>
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>数据源</span>
            </div>
          </template>
          <div class="stat-item">
            <div class="stat-value">{{ stats.totalSources }}</div>
            <div class="stat-label">总数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>媒体文件</span>
            </div>
          </template>
          <div class="stat-item">
            <div class="stat-value">{{ stats.totalFiles }}</div>
            <div class="stat-label">总数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>STRM 文件</span>
            </div>
          </template>
          <div class="stat-item">
            <div class="stat-value">{{ stats.totalStrm }}</div>
            <div class="stat-label">已生成</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <template #header>
            <div class="card-header">
              <span>运行任务</span>
            </div>
          </template>
          <div class="stat-item">
            <div class="stat-value">{{ stats.runningTasks }}</div>
            <div class="stat-label">进行中</div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const stats = ref({
  totalSources: 0,
  totalFiles: 0,
  totalStrm: 0,
  runningTasks: 0
})

onMounted(() => {
  // TODO: 从 API 获取统计数据
})
</script>

<style scoped>
.dashboard {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: bold;
}

.stat-item {
  text-align: center;
  padding: 20px 0;
}

.stat-value {
  font-size: 36px;
  font-weight: bold;
  color: #409eff;
}

.stat-label {
  margin-top: 10px;
  font-size: 14px;
  color: #909399;
}
</style>
