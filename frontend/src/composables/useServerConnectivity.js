import { onMounted, onUnmounted, reactive, ref, unref, watch } from 'vue'
import { testServerSilent } from '@/api/servers'

export const useServerConnectivity = (options) => {
  const serverListRef = options?.serverList
  const intervalMs = options?.intervalMs ?? 10000
  const maxConcurrentTests = options?.maxConcurrentTests ?? 3

  const connectionStatusMap = reactive({})
  const pollingTimer = ref(null)
  const pollingInFlight = ref(false)
  const lastTestAtMap = reactive({})
  const inFlightKeyMap = reactive({})
  const isUnmounted = ref(false)
  const localServerType = 'local'

  const getConnectionStatus = (server) => {
    if (!server?.enabled) return 'status-disabled'
    const cached = connectionStatusMap[server.id]
    if (cached) return 'status-' + cached
    return 'status-unknown'
  }

  const setConnectionStatus = (serverId, status) => {
    if (!serverId) return
    connectionStatusMap[serverId] = status
  }

  const refreshConnectionStatus = async () => {
    if (pollingInFlight.value) return
    const list = unref(serverListRef) || []
    const targets = list.filter(server => server.enabled && server.type !== localServerType)
    if (targets.length === 0) return

    pollingInFlight.value = true
    try {
      if (isUnmounted.value) return
      const now = Date.now()
      const queue = []
      for (const server of targets) {
        const key = String(server.id ?? '')
        if (!key) continue
        const lastAt = lastTestAtMap[key] || 0
        if (now - lastAt < intervalMs - 500) continue
        if (inFlightKeyMap[key]) continue
        queue.push({ key, server })
      }

      const workerCount = Math.min(maxConcurrentTests, queue.length)
      const workers = Array.from({ length: workerCount }, async () => {
        while (queue.length > 0) {
          if (isUnmounted.value) return
          const item = queue.shift()
          if (!item) return
          inFlightKeyMap[item.key] = true
          try {
            const result = await testServerSilent(item.server.id, item.server.type)
            if (isUnmounted.value) return
            const ok = result === true
            const status = ok ? 'success' : 'error'
            connectionStatusMap[item.key] = status
          } catch (error) {
            if (isUnmounted.value) return
            connectionStatusMap[item.key] = 'error'
            if (import.meta?.env?.DEV) {
              console.debug('服务器连通性检测失败:', error)
            }
          } finally {
            lastTestAtMap[item.key] = Date.now()
            delete inFlightKeyMap[item.key]
          }
        }
      })

      await Promise.all(workers)
    } finally {
      pollingInFlight.value = false
    }
  }

  const handleListChange = (newList) => {
    const validIds = new Set(newList.map(s => String(s.id)))
    for (const id in connectionStatusMap) {
      if (!validIds.has(String(id))) {
        delete connectionStatusMap[id]
        delete lastTestAtMap[id]
        delete inFlightKeyMap[id]
      }
    }
    for (const server of newList) {
      if (!server.enabled || server.type === localServerType) {
        const key = String(server.id ?? '')
        if (!key) continue
        delete connectionStatusMap[key]
        delete lastTestAtMap[key]
        delete inFlightKeyMap[key]
      }
    }
  }

  if (serverListRef) {
    watch(() => unref(serverListRef), (newList) => {
      if (Array.isArray(newList)) {
        handleListChange(newList)
      }
    }, { deep: true })
  }

  onMounted(() => {
    refreshConnectionStatus()
    pollingTimer.value = setInterval(() => {
      refreshConnectionStatus()
    }, intervalMs)
  })

  onUnmounted(() => {
    isUnmounted.value = true
    if (pollingTimer.value) {
      clearInterval(pollingTimer.value)
      pollingTimer.value = null
    }
  })

  return {
    connectionStatusMap,
    getConnectionStatus,
    setConnectionStatus,
    refreshConnectionStatus
  }
}
