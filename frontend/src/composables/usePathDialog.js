import { computed, ref } from 'vue'

const normalizePath = (value) => {
  if (!value) return '/'
  let path = String(value).trim()
  if (!path.startsWith('/')) path = `/${path}`
  path = path.replace(/\/+$/, '')
  return path === '' ? '/' : path
}

const joinPath = (base, name) => {
  if (!base || base === '/') return `/${name}`
  return `${base.replace(/\/+$/, '')}/${name}`
}

export { normalizePath, joinPath }

export const usePathDialog = (options = {}) => {
  if (typeof options.loader !== 'function') {
    throw new Error('usePathDialog: options.loader must be a function')
  }

  const onError = options.onError || (() => {})
  const pageSize = Number.isFinite(options.pageSize) && options.pageSize > 0 ? options.pageSize : 200

  const visible = ref(false)
  const mode = ref(options.initialMode || 'single')
  const root = ref(options.initialRoot || '/')
  const path = ref(options.initialPath || root.value)
  const rows = ref([])
  const loading = ref(false)
  const hasMore = ref(false)
  const offset = ref(0)
  const extra = ref({})
  const selectedName = ref('')
  const selectedNames = ref([])
  const refreshKey = ref(0) // 用于强制刷新表格

  const atRoot = computed(() => normalizePath(path.value) === normalizePath(root.value))

  const clearSelection = () => {
    selectedName.value = ''
    selectedNames.value = []
  }

  const buildRowPath = (name) => {
    if (!name) return normalizePath(path.value || '/')
    return normalizePath(joinPath(path.value || '/', name))
  }

  // 确保 path 不超出 root 范围
  const clampToRoot = (p) => {
    const norm = normalizePath(p)
    const rootNorm = normalizePath(root.value)
    if (rootNorm === '/') return norm
    return norm.startsWith(rootNorm) ? norm : rootNorm
  }

  const load = async (nextPath, append = false) => {
    loading.value = true
    try {
      const safePath = clampToRoot(nextPath || root.value)
      const response = await options.loader({
        path: safePath,
        limit: pageSize,
        offset: append ? offset.value : 0,
        ...extra.value
      })
      const nextPathValue = clampToRoot(response?.path || safePath)
      const nextRows = (response?.directories || []).map(name => ({ name }))
      if (append) {
        rows.value = rows.value.concat(nextRows)
      } else {
        rows.value = nextRows
        refreshKey.value++ // 刷新时递增 key，强制表格重新渲染
      }
      path.value = nextPathValue
      hasMore.value = Boolean(response?.truncated)
      offset.value = append ? offset.value + nextRows.length : nextRows.length
    } catch (error) {
      onError(error)
    } finally {
      loading.value = false
    }
  }

  const open = async (params = {}) => {
    mode.value = params.mode || mode.value
    root.value = params.root || root.value
    extra.value = params.extra || {}
    const initialPath = clampToRoot(params.path || root.value)
    path.value = initialPath
    offset.value = 0
    hasMore.value = false
    if (mode.value !== 'multi') {
      clearSelection()
    }
    visible.value = true
    await load(path.value)
  }

  const close = () => {
    visible.value = false
  }

  const goUp = async () => {
    if (atRoot.value) return
    const current = normalizePath(path.value)
    const rootNorm = normalizePath(root.value)
    const next = current.split('/').slice(0, -1).join('/') || '/'
    const safeNext = rootNorm !== '/' && !next.startsWith(rootNorm) ? rootNorm : next
    await load(safeNext)
  }

  const goRoot = async () => {
    await load(root.value || '/')
  }

  const jump = async (nextPath) => {
    const target = clampToRoot(nextPath || root.value)
    await load(target)
  }

  const enterDirectory = async (name) => {
    if (!name) return
    offset.value = 0
    hasMore.value = false
    await load(joinPath(path.value, name))
  }

  const loadMore = async () => {
    if (loading.value || !hasMore.value) return
    await load(path.value, true)
  }

  const selectRow = (name) => {
    selectedName.value = buildRowPath(name)
  }

  const toggleRow = (name) => {
    const rowPath = buildRowPath(name)
    const set = new Set(selectedNames.value.map(item => normalizePath(item)))
    if (set.has(rowPath)) {
      set.delete(rowPath)
    } else {
      set.add(rowPath)
    }
    selectedNames.value = Array.from(set)
  }

  const getSelectedSingle = () => {
    return normalizePath(selectedName.value || path.value || root.value || '/')
  }

  const getSelectedMulti = () => {
    return (selectedNames.value || []).filter(Boolean).map(item => normalizePath(item))
  }

  return {
    visible,
    mode,
    root,
    path,
    rows,
    loading,
    hasMore,
    selectedName,
    selectedNames,
    atRoot,
    refreshKey,
    open,
    close,
    load,
    loadMore,
    goUp,
    goRoot,
    jump,
    enterDirectory,
    selectRow,
    toggleRow,
    getSelectedSingle,
    getSelectedMulti,
    clearSelection
  }
}
