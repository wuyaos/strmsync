/**
 * 任务API
 * 任务配置与调度管理
 */
import request from './request'

/**
 * 获取任务列表
 * @param {Object} params 查询参数
 * @param {string} [params.status] 任务状态过滤
 * @param {string} [params.keyword] 搜索关键词
 * @param {number} [params.page] 页码
 * @param {number} [params.pageSize] 每页数量
 * @returns {Promise<{data: Array, meta: Object}>} 任务列表及分页信息
 */
export function getJobList(params) {
  return request({
    url: '/jobs',
    method: 'get',
    params
  })
}

/**
 * 获取任务详情
 * @param {string|number} id 任务ID
 * @returns {Promise<Object>} 任务详情
 */
export function getJob(id) {
  return request({
    url: `/jobs/${id}`,
    method: 'get'
  })
}

/**
 * 创建任务
 * @param {Object} data 任务数据
 * @param {string} data.name 任务名称
 * @param {number} data.data_server_id 数据服务器ID
 * @param {number} data.media_server_id 媒体服务器ID
 * @param {string} data.schedule 调度配置（cron表达式）
 * @param {Object} data.strategy 执行策略
 * @param {boolean} [data.enabled=true] 是否启用
 * @returns {Promise<Object>} 创建的任务
 */
export function createJob(data) {
  return request({
    url: '/jobs',
    method: 'post',
    data
  })
}

/**
 * 更新任务
 * @param {string|number} id 任务ID
 * @param {Object} data 任务数据
 * @returns {Promise<Object>} 更新后的任务
 */
export function updateJob(id, data) {
  return request({
    url: `/jobs/${id}`,
    method: 'put',
    data
  })
}

/**
 * 删除任务
 * @param {string|number} id 任务ID
 * @returns {Promise<void>}
 */
export function deleteJob(id) {
  return request({
    url: `/jobs/${id}`,
    method: 'delete'
  })
}

/**
 * 手动触发任务执行
 * @param {string|number} id 任务ID
 * @returns {Promise<Object>} 触发结果
 */
export function triggerJob(id) {
  return request({
    url: `/jobs/${id}/trigger`,
    method: 'post'
  })
}

/**
 * 启用任务
 * @param {string|number} id 任务ID
 * @returns {Promise<void>}
 */
export function enableJob(id) {
  return request({
    url: `/jobs/${id}/enable`,
    method: 'put'
  })
}

/**
 * 禁用任务
 * @param {string|number} id 任务ID
 * @returns {Promise<void>}
 */
export function disableJob(id) {
  return request({
    url: `/jobs/${id}/disable`,
    method: 'put'
  })
}
