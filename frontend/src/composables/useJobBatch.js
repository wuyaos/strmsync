import { ElMessage } from 'element-plus'
import { confirmDialog } from '@/composables/useConfirmDialog'
import { enableJob, disableJob, triggerJob, deleteJob } from '@/api/jobs'

export const useJobBatch = ({ jobList, loadJobs }) => {
  const resolveJobNames = (ids) => {
    const map = new Map(jobList.value.map((job) => [job.id, job.name || `ID:${job.id}`]))
    return ids.map((id) => map.get(id) || `ID:${id}`)
  }

  const handleBatchEnable = async (ids) => {
    if (!Array.isArray(ids) || ids.length === 0) return
    const confirmed = await confirmDialog({
      title: '批量启用',
      message: `将启用选中的 ${ids.length} 个任务，是否继续？`,
      type: 'info',
      items: resolveJobNames(ids),
      confirmText: '确认启用',
      cancelText: '取消'
    })
    if (!confirmed) return
    const results = await Promise.allSettled(ids.map((id) => enableJob(id)))
    const failCount = results.filter((r) => r.status === 'rejected').length
    if (failCount === 0) {
      ElMessage.success(`成功启用 ${ids.length} 个任务`)
    } else {
      ElMessage.warning(`启用完成：成功 ${ids.length - failCount} 个，失败 ${failCount} 个`)
    }
    loadJobs()
  }

  const handleBatchDisable = async (ids) => {
    if (!Array.isArray(ids) || ids.length === 0) return
    const confirmed = await confirmDialog({
      title: '批量禁用',
      message: `将禁用选中的 ${ids.length} 个任务，是否继续？`,
      type: 'warning',
      items: resolveJobNames(ids),
      confirmText: '确认禁用',
      cancelText: '取消'
    })
    if (!confirmed) return
    const results = await Promise.allSettled(ids.map((id) => disableJob(id)))
    const failCount = results.filter((r) => r.status === 'rejected').length
    if (failCount === 0) {
      ElMessage.success(`成功禁用 ${ids.length} 个任务`)
    } else {
      ElMessage.warning(`禁用完成：成功 ${ids.length - failCount} 个，失败 ${failCount} 个`)
    }
    loadJobs()
  }

  const handleBatchRun = async (ids) => {
    if (!Array.isArray(ids) || ids.length === 0) return
    const confirmed = await confirmDialog({
      title: '批量运行',
      message: `将运行选中的 ${ids.length} 个任务，是否继续？`,
      type: 'info',
      items: resolveJobNames(ids),
      confirmText: '确认运行',
      cancelText: '取消'
    })
    if (!confirmed) return
    const results = await Promise.allSettled(ids.map((id) => triggerJob(id)))
    const failCount = results.filter((r) => r.status === 'rejected').length
    if (failCount === 0) {
      ElMessage.success(`成功运行 ${ids.length} 个任务`)
    } else {
      ElMessage.warning(`运行完成：成功 ${ids.length - failCount} 个，失败 ${failCount} 个`)
    }
    loadJobs()
  }

  const handleBatchDelete = async (ids) => {
    if (!Array.isArray(ids) || ids.length === 0) return
    const confirmed = await confirmDialog({
      title: '批量删除',
      message: `将删除选中的 ${ids.length} 个任务，且无法恢复，是否继续？`,
      type: 'error',
      items: resolveJobNames(ids),
      confirmText: '确认删除',
      cancelText: '取消'
    })
    if (!confirmed) return
    const results = await Promise.allSettled(ids.map((id) => deleteJob(id)))
    const failCount = results.filter((r) => r.status === 'rejected').length
    if (failCount === 0) {
      ElMessage.success(`成功删除 ${ids.length} 个任务`)
    } else {
      ElMessage.warning(`删除完成：成功 ${ids.length - failCount} 个，失败 ${failCount} 个`)
    }
    loadJobs()
  }

  return {
    handleBatchEnable,
    handleBatchDisable,
    handleBatchRun,
    handleBatchDelete
  }
}
