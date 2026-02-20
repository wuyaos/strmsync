import { reactive, ref } from 'vue'
import { getServerList } from '@/api/servers'
import { normalizeListResponse } from '@/api/normalize'

/**
 * 服务器列表状态管理 Composable
 * 负责：列表加载、分页、筛选、Tab 切换
 */
export function useServerList(options = {}) {
  const {
    initialTab = 'data',
    initialPageSize = 12,
    onBeforeLoad = null,
    onAfterLoad = null
  } = options

  // 状态
  const activeTab = ref(initialTab)
  const loading = ref(false)
  const serverList = ref([])

  const filters = reactive({
    keyword: ''
  })

  const pagination = reactive({
    page: 1,
    pageSize: initialPageSize,
    total: 0
  })

  // 加载服务器列表
  const loadServers = async () => {
    if (onBeforeLoad) onBeforeLoad()

    loading.value = true
    try {
      const params = {
        type: activeTab.value,
        keyword: filters.keyword,
        page: pagination.page,
        pageSize: pagination.pageSize
      }
      const response = await getServerList(params)
      const { list, total } = normalizeListResponse(response)
      serverList.value = list
      pagination.total = total

      if (onAfterLoad) onAfterLoad(list, total)
    } catch (error) {
      console.error('加载服务器列表失败:', error)
    } finally {
      loading.value = false
    }
  }

  // 刷新列表（保持当前页）
  const refresh = async () => {
    await loadServers()
  }

  // 搜索（重置到第一页）
  const handleSearch = () => {
    pagination.page = 1
    loadServers()
  }

  // Tab 切换（重置筛选和分页）
  const handleTabChange = (tab) => {
    if (tab) activeTab.value = tab
    pagination.page = 1
    filters.keyword = ''
    loadServers()
  }

  // 分页变更
  const handlePageChange = (page) => {
    pagination.page = page
    loadServers()
  }

  // 页大小变更
  const handleSizeChange = (size) => {
    pagination.pageSize = size
    pagination.page = 1
    loadServers()
  }

  return {
    // 状态
    activeTab,
    filters,
    pagination,
    serverList,
    loading,
    // 方法
    loadServers,
    refresh,
    handleSearch,
    handleTabChange,
    handlePageChange,
    handleSizeChange
  }
}
