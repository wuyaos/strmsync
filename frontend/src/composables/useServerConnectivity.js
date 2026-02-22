import { isRef, onActivated, onDeactivated, onMounted, onUnmounted, reactive, unref, watch } from 'vue'
import { testServerSilent } from '@/api/servers'

const STATUS = { DISABLED: 'disabled', SUCCESS: 'success', UNKNOWN: 'unknown', ERROR: 'error' }
const localServerType = 'local'

export const useServerConnectivity = (options) => {
  const serverListRef = options?.serverList
  const intervalMs = Math.max(options?.intervalMs ?? 10000, 2000)
  const maxConcurrentTests = options?.maxConcurrentTests ?? 3
  const requestTimeoutMs = options?.requestTimeoutMs ?? 8000
  const autoStart = options?.autoStart !== false
  const activeRef = options?.isActive

  const connectionStatusMap = reactive({})
  const lastTestAtMap = {}
  const nextAllowedAtMap = {}
  const failureCountMap = {}
  const activeServerKeys = new Set()

  let pollingTimer = null
  let isUnmounted = false
  let isTesting = false
  let listChangeTimer = null
  let isPaused = !autoStart

  const inFlightControllers = new Set()

  const getSafeId = (id) => String(id ?? '')
  const resolveActive = () => {
    if (!activeRef) return true
    if (typeof activeRef === 'function') {
      return activeRef()
    }
    if (isRef(activeRef)) {
      return Boolean(activeRef.value)
    }
    return Boolean(activeRef)
  }

  // 同步当前活跃的服务器 ID，并清理掉已删除或已禁用的状态缓存
  const syncActiveKeys = (list) => {
    activeServerKeys.clear()
    const validKeys = new Set()
    for (const server of list) {
      const key = getSafeId(server.id)
      if (!key) continue
      validKeys.add(key)
      if (server.enabled && server.type !== localServerType) {
        activeServerKeys.add(key)
      } else {
        delete connectionStatusMap[key]
        delete nextAllowedAtMap[key]
        delete failureCountMap[key]
      }
    }
    // 清理已经不存在于列表中的服务器状态
    for (const key of Object.keys(connectionStatusMap)) {
      if (!validKeys.has(key)) {
        delete connectionStatusMap[key]
        delete lastTestAtMap[key]
        delete nextAllowedAtMap[key]
        delete failureCountMap[key]
      }
    }
  }

  const getConnectionStatus = (server) => {
    if (!server?.enabled) return `status-${STATUS.DISABLED}`
    if (server?.type === localServerType) return `status-${STATUS.SUCCESS}`
    const key = getSafeId(server?.id)
    return `status-${connectionStatusMap[key] || STATUS.UNKNOWN}`
  }

  const setConnectionStatus = (serverId, status) => {
    const key = getSafeId(serverId)
    if (key) connectionStatusMap[key] = status
  }

  const runTest = async (server) => {
    const controller = new AbortController()
    inFlightControllers.add(controller)
    const timeoutId = setTimeout(() => controller.abort('TimeoutError'), requestTimeoutMs)

    try {
      const result = await testServerSilent(server.id, server.type, { signal: controller.signal })
      return { success: result === true || result?.success === true }
    } catch (error) {
      return { success: false, error }
    } finally {
      clearTimeout(timeoutId)
      inFlightControllers.delete(controller)
    }
  }

  const resolveBackoffMs = (error, failureCount) => {
    const status = Number(error?.status)
    if (status === 429) {
      return 10 * 60 * 1000
    }
    if (Number.isFinite(status) && status >= 400 && status < 500) {
      return 2 * 60 * 1000
    }
    const base = 10 * 1000
    const max = 5 * 60 * 1000
    const exponent = Math.min(Math.max(failureCount - 1, 0), 6)
    return Math.min(base * Math.pow(2, exponent), max)
  }

  const refreshConnectionStatus = async (force = false) => {
    if (isUnmounted || isPaused || isTesting) return
    if (!resolveActive()) return
    isTesting = true

    try {
      const list = unref(serverListRef) || []
      const now = Date.now()
      const targets = list.filter(s => {
        if (!s.enabled || s.type === localServerType) return false
        const key = getSafeId(s.id)
        if (!key) return false
        const nextAllowedAt = nextAllowedAtMap[key] || 0
        if (!force && now < nextAllowedAt) return false
        // 除非强制刷新，否则在轮询间隔内的不再重复测试
        if (!force && (now - (lastTestAtMap[key] || 0) < intervalMs - 500)) return false
        return true
      })

      // 并发控制池 (Concurrency Pool)
      const executing = new Set()
      for (const server of targets) {
        if (isUnmounted) break
        const key = getSafeId(server.id)
        const p = runTest(server).then(result => {
          if (isUnmounted || !activeServerKeys.has(key)) return
          if (result?.success) {
            connectionStatusMap[key] = STATUS.SUCCESS
            lastTestAtMap[key] = Date.now()
            failureCountMap[key] = 0
            nextAllowedAtMap[key] = 0
            return
          }
          connectionStatusMap[key] = STATUS.ERROR
          lastTestAtMap[key] = Date.now()
          const nextCount = (failureCountMap[key] || 0) + 1
          failureCountMap[key] = nextCount
          nextAllowedAtMap[key] = Date.now() + resolveBackoffMs(result?.error, nextCount)
        }).finally(() => executing.delete(p))

        executing.add(p)
        // 达到最大并发限制时，等待任意一个请求完成
        if (executing.size >= maxConcurrentTests) {
          await Promise.race(executing)
        }
      }
      // 等待剩余请求全部完成
      await Promise.all(executing)
    } finally {
      isTesting = false
    }
  }

  const scheduleNext = () => {
    if (isUnmounted || isPaused) return
    clearTimeout(pollingTimer)
    pollingTimer = setTimeout(async () => {
      await refreshConnectionStatus()
      scheduleNext()
    }, intervalMs)
  }

  if (serverListRef) {
    // 使用签名监听以避免对象引用变化带来的无限循环刷新，仅关注关键字段
    watch(
      () => {
        const list = unref(serverListRef) || []
        return list.map(s => `${s.id}|${s.type}|${s.enabled}|${s.host}|${s.port}`).join(',')
      },
      (newSig, oldSig) => {
        if (newSig !== oldSig) {
          syncActiveKeys(unref(serverListRef) || [])
          if (isPaused) return
          clearTimeout(listChangeTimer)
          listChangeTimer = setTimeout(() => refreshConnectionStatus(true), 300)
        }
      }
    )
  }

  const start = () => {
    if (isUnmounted) return
    if (!resolveActive()) return
    isPaused = false
    if (!serverListRef) return
    syncActiveKeys(unref(serverListRef) || [])
    refreshConnectionStatus().finally(() => scheduleNext())
  }

  const pause = () => {
    isPaused = true
    clearTimeout(pollingTimer)
    clearTimeout(listChangeTimer)
    for (const controller of inFlightControllers) {
      controller.abort('Unmounted')
    }
    inFlightControllers.clear()
    isTesting = false
  }

  onMounted(() => {
    if (autoStart) start()
  })

  onActivated(() => {
    if (autoStart) start()
  })

  onDeactivated(() => {
    pause()
  })

  if (activeRef) {
    watch(
      () => resolveActive(),
      (active) => {
        if (!autoStart) return
        if (active) {
          start()
        } else {
          pause()
        }
      },
      { immediate: true }
    )
  }

  const stop = () => {
    isUnmounted = true
    pause()
  }

  onUnmounted(stop)

  return {
    connectionStatusMap,
    getConnectionStatus,
    setConnectionStatus,
    refreshConnectionStatus,
    stop
  }
}
