import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { confirmDialog } from '@/composables/useConfirmDialog'

/**
 * 服务器批量操作 Composable
 *
 * 策略：
 * - 选择状态不跨分页持久化（切页清空）
 * - 选择状态不跨tab持久化（切tab清空）
 * - 批量操作限制：最多5个服务器
 */
export function useServerBatch() {
  // 选择状态
  const selectedIds = ref(new Set())
  const batchMode = ref(false)

  // 是否全选（基于当前列表）
  const isAllSelected = computed(() => {
    return (list) => {
      if (!list || list.length === 0) return false
      return list.every(server => selectedIds.value.has(server.id))
    }
  })

  // 已选数量
  const selectedCount = computed(() => selectedIds.value.size)

  // 切换单个选择
  const toggleSelect = (server) => {
    if (selectedIds.value.has(server.id)) {
      selectedIds.value.delete(server.id)
    } else {
      selectedIds.value.add(server.id)
    }
  }

  // 全选/取消全选
  const toggleSelectAll = (list, value) => {
    if (value) {
      list.forEach(server => selectedIds.value.add(server.id))
    } else {
      selectedIds.value.clear()
    }
  }

  // 清空选择
  const clearSelection = () => {
    selectedIds.value.clear()
    batchMode.value = false
  }

  // 检查是否选中
  const isSelected = (server) => {
    return selectedIds.value.has(server.id)
  }

  // 批量操作前的验证
  const validateBatchOperation = () => {
    if (selectedIds.value.size === 0) {
      ElMessage.warning('请至少选择一个服务器')
      return false
    }
    if (selectedIds.value.size > 5) {
      ElMessage.warning('批量操作最多支持5个服务器，请减少选择')
      return false
    }
    return true
  }

  const buildTogglePayload = (server, enabled) => {
    const options = server.options ?? '{}'
    return {
      name: server.name ?? '',
      type: server.type ?? '',
      host: server.host ?? '',
      port: server.port ?? 0,
      api_key: server.api_key ?? server.apiKey ?? '',
      options,
      enabled,
      download_rate_per_sec: server.download_rate_per_sec ?? server.downloadRatePerSec,
      api_rate: server.api_rate ?? server.apiRate,
      api_retry_max: server.api_retry_max ?? server.apiRetryMax,
      api_retry_interval_sec: server.api_retry_interval_sec ?? server.apiRetryIntervalSec
    }
  }

  // 批量启用
  const handleBatchEnable = async (serverList, updateServerFn, refreshListFn) => {
    if (!validateBatchOperation()) return

    try {
      const selectedServers = serverList.filter(s => selectedIds.value.has(s.id))
      const confirmed = await confirmDialog({
        title: '批量启用',
        message: '将对以下服务器执行“启用”操作：',
        type: 'info',
        items: selectedServers.map(s => s.name || `ID:${s.id}`),
        confirmText: '确认启用',
        cancelText: '取消'
      })
      if (!confirmed) return

      const results = await Promise.allSettled(
        selectedServers.map(server => updateServerFn(server.id, buildTogglePayload(server, true)))
      )

      const successCount = results.filter(r => r.status === 'fulfilled').length
      const failCount = results.filter(r => r.status === 'rejected').length

      if (failCount === 0) {
        ElMessage.success(`成功启用 ${successCount} 个服务器`)
      } else {
        ElMessage.warning(`启用完成：成功 ${successCount} 个，失败 ${failCount} 个`)
      }

      clearSelection()
      await refreshListFn()
    } catch (error) {
      if (error !== 'cancel') {
        console.error('批量启用失败:', error)
      }
    }
  }

  // 批量禁用
  const handleBatchDisable = async (serverList, updateServerFn, refreshListFn) => {
    if (!validateBatchOperation()) return

    try {
      const selectedServers = serverList.filter(s => selectedIds.value.has(s.id))
      const confirmed = await confirmDialog({
        title: '批量禁用',
        message: '将对以下服务器执行“禁用”操作：',
        type: 'warning',
        items: selectedServers.map(s => s.name || `ID:${s.id}`),
        confirmText: '确认禁用',
        cancelText: '取消'
      })
      if (!confirmed) return

      const results = await Promise.allSettled(
        selectedServers.map(server => updateServerFn(server.id, buildTogglePayload(server, false)))
      )

      const successCount = results.filter(r => r.status === 'fulfilled').length
      const failCount = results.filter(r => r.status === 'rejected').length

      if (failCount === 0) {
        ElMessage.success(`成功禁用 ${successCount} 个服务器`)
      } else {
        ElMessage.warning(`禁用完成：成功 ${successCount} 个，失败 ${failCount} 个`)
      }

      clearSelection()
      await refreshListFn()
    } catch (error) {
      if (error !== 'cancel') {
        console.error('批量禁用失败:', error)
      }
    }
  }

  // 批量删除
  const handleBatchDelete = async (serverList, deleteServerFn, refreshListFn) => {
    if (!validateBatchOperation()) return

    try {
      const selectedServers = serverList.filter(s => selectedIds.value.has(s.id))
      const confirmed = await confirmDialog({
        title: '批量删除',
        message: '该操作不可恢复，将删除以下服务器：',
        type: 'error',
        items: selectedServers.map(s => s.name || `ID:${s.id}`),
        confirmText: '确认删除',
        cancelText: '取消'
      })
      if (!confirmed) return

      const results = await Promise.allSettled(
        selectedServers.map(server => deleteServerFn(server.id, server.type))
      )

      const successCount = results.filter(r => r.status === 'fulfilled').length
      const failCount = results.filter(r => r.status === 'rejected').length

      if (failCount === 0) {
        ElMessage.success(`成功删除 ${successCount} 个服务器`)
      } else {
        ElMessage.warning(`删除完成：成功 ${successCount} 个，失败 ${failCount} 个`)
      }

      clearSelection()
      await refreshListFn()
    } catch (error) {
      if (error !== 'cancel') {
        console.error('批量删除失败:', error)
      }
    }
  }

  return {
    // 状态
    selectedIds,
    batchMode,
    selectedCount,
    isAllSelected,

    // 方法
    toggleSelect,
    toggleSelectAll,
    clearSelection,
    isSelected,
    handleBatchEnable,
    handleBatchDisable,
    handleBatchDelete
  }
}
