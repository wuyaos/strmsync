import { onMounted, ref } from "vue"
import { ElMessage } from "element-plus"
import { getSettings, updateSettings } from "@/api/settings"
import { useSystemInfo } from "@/composables/useSystemInfo"

export const useSettingsPage = () => {
  const activeTab = ref("global")

  const predefineColors = ref([
    "#409EFF",
    "#67C23A",
    "#E6A23C",
    "#F56C6C",
    "#909399",
    "#ff4500",
    "#ff8c00",
    "#ffd700",
    "#90ee90",
    "#00ced1",
    "#1e90ff",
    "#c71585"
  ])

  const createDefaultSettings = () => ({
    global: {
      concurrency: 20,
      batchSize: 500,
      download_rate_per_sec: 10,
      api_rate: 5,
      api_concurrency: 3,
      api_retry_max: 3,
      api_retry_interval_sec: 60
    },
    ui: {
      auto_refresh_interval_ms: 2000
    },
    theme: {
      mode: "light",
      primaryColor: "#409EFF",
      compact: false
    },
    notification: {
      position: "top-right",
      duration: 3,
      showIcon: true,
      sound: false
    }
  })

  const settings = ref(createDefaultSettings())
  const { frontendVersion, loadSystemInfo } = useSystemInfo()

  const loadSettings = async () => {
    const defaults = createDefaultSettings()
    try {
      const data = await getSettings()
      const resolved = data?.settings || data
      if (resolved) {
        const resolvedScanner = resolved.scanner || {}
        const resolvedRate = resolved.rate || resolved.qos || {}
        const resolvedGlobal = resolved.global || {}
        const resolvedUI = resolved.ui || {}
        settings.value = {
          global: {
            concurrency: resolvedGlobal.concurrency ?? resolvedScanner.concurrency ?? defaults.global.concurrency,
            batchSize: resolvedGlobal.batchSize ?? resolvedScanner.batchSize ?? defaults.global.batchSize,
            download_rate_per_sec: resolvedGlobal.download_rate_per_sec ?? resolvedRate.download_rate_per_sec ?? defaults.global.download_rate_per_sec,
            api_rate: resolvedGlobal.api_rate ?? resolvedRate.api_rate ?? defaults.global.api_rate,
            api_concurrency: resolvedGlobal.api_concurrency ?? resolvedRate.api_concurrency ?? defaults.global.api_concurrency,
            api_retry_max: resolvedGlobal.api_retry_max ?? resolvedRate.api_retry_max ?? defaults.global.api_retry_max,
            api_retry_interval_sec: resolvedGlobal.api_retry_interval_sec ?? resolvedRate.api_retry_interval_sec ?? defaults.global.api_retry_interval_sec
          },
          ui: {
            auto_refresh_interval_ms: resolvedUI.auto_refresh_interval_ms ?? defaults.ui.auto_refresh_interval_ms
          },
          theme: { ...defaults.theme, ...(resolved.theme || {}) },
          notification: { ...defaults.notification, ...(resolved.notification || {}) }
        }
      } else {
        settings.value = defaults
      }
    } catch (error) {
      console.error("加载设置失败:", error)
      settings.value = defaults
    }
  }

  const handleSave = async () => {
    try {
      const payload = {
        ...settings.value,
        scanner: {
          concurrency: settings.value?.global?.concurrency ?? 20,
          batchSize: settings.value?.global?.batchSize ?? 500
        },
        rate: {
          download_rate_per_sec: settings.value?.global?.download_rate_per_sec ?? 10,
          api_rate: settings.value?.global?.api_rate ?? 5,
          api_concurrency: settings.value?.global?.api_concurrency ?? 3,
          api_retry_max: settings.value?.global?.api_retry_max ?? 3,
          api_retry_interval_sec: settings.value?.global?.api_retry_interval_sec ?? 60
        }
      }
      delete payload.global
      await updateSettings(payload)
      ElMessage.success("设置已保存")
    } catch (error) {
      console.error("保存设置失败:", error)
      ElMessage.error(error.response?.data?.error || "保存失败")
    }
  }

  onMounted(() => {
    loadSettings()
    loadSystemInfo()
  })

  return {
    activeTab,
    predefineColors,
    settings,
    frontendVersion,
    handleSave
  }
}
