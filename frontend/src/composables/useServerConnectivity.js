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
    const targets = list.filter(server => server.enabled && server.type !== 'local')
    if (targets.length === 0) return

    pollingInFlight.value = true
    try {
      const now = Date.now()
      const queue = []
      for (const server of targets) {
        const key = String(server.id ?? '')
        if (!key) continue
        const lastAt = lastTestAtMap[key] || 0
        if (now - lastAt < intervalMs) continue
        if (inFlightKeyMap[key]) continue
        queue.push({ key, server })
      }

      const workers = Array.from({ length: maxConcurrentTests }, async () => {
        while (queue.length > 0) {
          const item = queue.shift()
          if (!item) return
          inFlightKeyMap[item.key] = true
          try {
            const result = await testServerSilent(item.server.id, item.server.type)
            const ok = !!result || result === undefined
            const status = ok ? 'success' : 'error'
            connectionStatusMap[item.server.id] = status
          } catch (error) {
            connectionStatusMap[item.server.id] = 'error'
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
      if (!server.enabled || server.type === 'local') {
        delete connectionStatusMap[server.id]
        delete lastTestAtMap[server.id]
        delete inFlightKeyMap[server.id]
      }
    }
  }

  if (serverListRef) {
    watch(() => unref(serverListRef), (newList) => {
      if (Array.isArray(newList)) {
        handleListChange(newList)
      }
    })
  }

  onMounted(() => {
    pollingTimer.value = setInterval(() => {
      refreshConnectionStatus()
    }, intervalMs)
  })

  onUnmounted(() => {
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
