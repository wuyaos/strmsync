/**
 * 系统信息组合式函数
 * 提供版本号等全局可复用数据
 */
import { ref } from 'vue'
import { getHealth } from '@/api/system'

const frontendVersion = ref(import.meta.env.VITE_APP_VERSION || 'unknown')
const backendVersion = ref('unknown')
const loading = ref(false)
let loaded = false

export const useSystemInfo = () => {
  const loadSystemInfo = async (force = false) => {
    if (loading.value || (loaded && !force)) return
    loading.value = true
    try {
      const data = await getHealth()
      backendVersion.value = data?.version || 'unknown'
      if (data?.frontend_version) {
        frontendVersion.value = data.frontend_version
      }
      loaded = true
    } catch (error) {
      console.error('加载系统信息失败:', error)
    } finally {
      loading.value = false
    }
  }

  return {
    frontendVersion,
    backendVersion,
    loadSystemInfo
  }
}
