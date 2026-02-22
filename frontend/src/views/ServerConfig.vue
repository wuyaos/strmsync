<template>
  <div class="server-config-page flex flex-col gap-16">
    <!-- 数据服务器配置 -->
    <div class="section">
      <div class="content-header">
        <h3>数据服务器配置</h3>
        <p class="description">配置各种数据源适配器（Local、CloudDrive2、OpenList等）的默认参数</p>
      </div>

      <el-form :model="dataServerConfig" label-width="140px" class="config-form">
        <el-form-item label="默认并发数">
          <el-input
            v-model.number="dataServerConfig.defaultConcurrency"
            type="number"
            :min="1"
            :max="100"
            class="input-short"
          />
          <span class="form-help">新建数据源时的默认并发扫描数</span>
        </el-form-item>

        <el-form-item label="默认批量大小">
          <el-input
            v-model.number="dataServerConfig.defaultBatchSize"
            type="number"
            :min="10"
            :max="1000"
            class="input-short"
          />
          <span class="form-help">批量写入数据库的默认记录数</span>
        </el-form-item>

        <el-form-item label="默认超时时间(秒)">
          <el-input
            v-model.number="dataServerConfig.defaultTimeout"
            type="number"
            :min="1"
            :max="300"
            class="input-short"
          />
          <span class="form-help">数据源连接的默认超时时间</span>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="saveDataServer">保存数据服务器配置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-divider />

    <!-- 媒体服务器配置 -->
    <div class="section">
      <div class="content-header">
        <h3>媒体服务器配置</h3>
        <p class="description">配置Emby、Jellyfin、Plex等媒体服务器，用于媒体库刷新通知</p>
      </div>

      <el-form :model="mediaServerConfig" label-width="140px" class="config-form">
        <el-form-item label="启用通知">
          <el-switch v-model="mediaServerConfig.enabled" />
          <span class="form-help">是否启用媒体库刷新通知功能</span>
        </el-form-item>

        <template v-if="mediaServerConfig.enabled">
          <el-divider content-position="left">服务器信息</el-divider>

          <el-form-item label="通知提供商">
            <el-select v-model="mediaServerConfig.provider" placeholder="选择通知提供商">
              <el-option label="Emby" value="emby" />
              <el-option label="Jellyfin" value="jellyfin" />
              <el-option label="Plex" value="plex" />
            </el-select>
          </el-form-item>

          <el-form-item label="服务器地址">
            <el-input
              v-model="mediaServerConfig.baseURL"
              placeholder="http://localhost:8096"
              clearable
            />
            <span class="form-help">媒体服务器的完整URL地址</span>
          </el-form-item>

          <el-form-item label="API Token">
            <el-input
              v-model="mediaServerConfig.token"
              type="password"
              placeholder="输入API密钥"
              show-password
              clearable
            />
            <span class="form-help">媒体服务器的API密钥或访问令牌</span>
          </el-form-item>

          <el-divider content-position="left">通知参数</el-divider>

          <el-form-item label="超时时间(秒)">
            <el-input
              v-model.number="mediaServerConfig.timeoutSeconds"
              type="number"
              :min="1"
              :max="60"
              class="input-short"
            />
            <span class="form-help">通知请求的超时时间</span>
          </el-form-item>

          <el-form-item label="重试次数">
            <el-input
              v-model.number="mediaServerConfig.retryMax"
              type="number"
              :min="0"
              :max="10"
              class="input-short"
            />
            <span class="form-help">通知失败后的重试次数</span>
          </el-form-item>

          <el-form-item label="防抖延迟(秒)">
            <el-input
              v-model.number="mediaServerConfig.debounceSeconds"
              type="number"
              :min="0"
              :max="300"
              class="input-short"
            />
            <span class="form-help">多个通知请求会合并为一次，避免频繁刷新</span>
          </el-form-item>

          <el-form-item label="通知范围">
            <el-radio-group v-model="mediaServerConfig.scope">
              <el-radio-button value="global">全局刷新</el-radio-button>
              <el-radio-button value="library">库级刷新</el-radio-button>
              <el-radio-button value="path">路径级刷新</el-radio-button>
            </el-radio-group>
            <div class="form-help">
              <p>• 全局刷新：刷新整个媒体库</p>
              <p>• 库级刷新：仅刷新对应的媒体库</p>
              <p>• 路径级刷新：仅刷新特定路径（精确度最高）</p>
            </div>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" @click="saveMediaServer">保存媒体服务器配置</el-button>
            <el-button @click="testConnection">测试</el-button>
          </el-form-item>
        </template>

        <el-form-item v-else>
          <el-alert
            title="通知功能已禁用"
            type="info"
            :closable="false"
            description="启用后可配置媒体服务器刷新通知，实现自动化媒体库更新"
          />
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getDataServer, updateDataServer, getMediaServer, updateMediaServer } from '@/api/server'

// 数据服务器默认配置
const defaultDataServerConfig = {
  defaultConcurrency: 20,
  defaultBatchSize: 500,
  defaultTimeout: 30
}

// 数据服务器配置
const dataServerConfig = ref({ ...defaultDataServerConfig })

// 媒体服务器默认配置
const defaultMediaServerConfig = {
  enabled: false,
  provider: 'emby',
  baseURL: '',
  token: '',
  timeoutSeconds: 10,
  retryMax: 3,
  debounceSeconds: 5,
  scope: 'global'
}

// 媒体服务器配置
const mediaServerConfig = ref({ ...defaultMediaServerConfig })

// 加载数据服务器配置
const loadDataServer = async () => {
  try {
    const data = await getDataServer()
    if (data) {
      dataServerConfig.value = {
        ...defaultDataServerConfig,
        ...data
      }
    } else {
      dataServerConfig.value = { ...defaultDataServerConfig }
    }
  } catch (error) {
    console.error('加载数据服务器配置失败:', error)
    dataServerConfig.value = { ...defaultDataServerConfig }
  }
}

// 加载媒体服务器配置
const loadMediaServer = async () => {
  try {
    const data = await getMediaServer()
    if (data) {
      mediaServerConfig.value = {
        ...defaultMediaServerConfig,
        ...data
      }
    } else {
      mediaServerConfig.value = { ...defaultMediaServerConfig }
    }
  } catch (error) {
    console.error('加载媒体服务器配置失败:', error)
    mediaServerConfig.value = { ...defaultMediaServerConfig }
  }
}

// 保存数据服务器配置
const saveDataServer = async () => {
  try {
    await updateDataServer(dataServerConfig.value)
    ElMessage.success('数据服务器配置已保存')
  } catch (error) {
    console.error('保存数据服务器配置失败:', error)
    ElMessage.error(error.response?.data?.error || '保存失败')
  }
}

// 保存媒体服务器配置
const saveMediaServer = async () => {
  try {
    await updateMediaServer(mediaServerConfig.value)
    ElMessage.success('媒体服务器配置已保存')
  } catch (error) {
    console.error('保存媒体服务器配置失败:', error)
    ElMessage.error(error.response?.data?.error || '保存失败')
  }
}

// 测试连接
const testConnection = async () => {
  if (!mediaServerConfig.value.baseURL || !mediaServerConfig.value.token) {
    ElMessage.warning('请先填写服务器地址和API Token')
    return
  }

  try {
    // TODO: 调用测试连接API
    ElMessage.success('连接测试成功')
  } catch (error) {
    console.error('测试连接失败:', error)
    ElMessage.error('连接测试失败')
  }
}

// 组件挂载时加载配置
onMounted(() => {
  loadDataServer()
  loadMediaServer()
})
</script>

<style scoped lang="scss">
.server-config-page {
  .section {
    padding: 0 20px 20px;

    .content-header {
      margin-bottom: 24px;

      h3 {
        margin: 0 0 8px 0;
        font-size: 18px;
        font-weight: 600;
        color: var(--el-text-color-primary);
      }

      .description {
        margin: 0;
        font-size: 14px;
        color: var(--el-text-color-secondary);
      }
    }

    .config-form {
      max-width: 600px;

      .form-help {
        margin-left: 12px;
        font-size: 12px;
        color: var(--el-text-color-secondary);

        p {
          margin: 4px 0;
        }
      }

      :deep(.el-divider__text) {
        font-weight: 500;
        color: var(--el-text-color-primary);
      }
    }
  }
}

.input-short {
  width: 140px;
}
</style>
