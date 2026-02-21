/**
 * 任务运行记录API
 * 执行历史与状态监控
 */
import request from './request'

/**
 * 获取运行记录列表
 * @param {Object} params 查询参数
 * @param {number} [params.jobId] 任务ID过滤
 * @param {string} [params.status] 运行状态过滤
 * @param {string} [params.from] 开始时间（ISO格式）
 * @param {string} [params.to] 结束时间（ISO格式）
 * @param {number} [params.page] 页码
 * @param {number} [params.pageSize] 每页数量
 * @returns {Promise<{data: Array, meta: Object}>} 运行记录列表及分页信息
 */
export function getRunList(params) {
  return request({
    url: '/runs',
    method: 'get',
    params
  })
}

/**
 * 获取运行记录详情
 * @param {string|number} id 运行记录ID
 * @returns {Promise<Object>} 运行记录详情（含日志和错误信息）
 */
export function getRun(id) {
  return request({
    url: `/runs/${id}`,
    method: 'get'
  })
}

/**
 * 获取运行记录事件明细
 * @param {string|number} id 运行记录ID
 * @param {Object} params 查询参数
 * @param {number} [params.page] 页码
 * @param {number} [params.pageSize] 每页数量
 * @param {string} [params.kind] 事件类型
 * @param {string} [params.op] 操作类型
 * @param {string} [params.status] 事件状态
 * @returns {Promise<Object>} 事件列表及分页信息
 */
export function getRunEvents(id, params) {
  return request({
    url: `/runs/${id}/events`,
    method: 'get',
    params
  })
}

/**
 * 取消正在运行的任务
 * @param {string|number} id 运行记录ID
 * @returns {Promise<void>}
 */
export function cancelRun(id) {
  return request({
    url: `/runs/${id}/cancel`,
    method: 'post'
  })
}

/**
 * 获取运行统计信息
 * @param {Object} params 查询参数
 * @param {number} [params.jobId] 任务ID
 * @param {string} [params.from] 开始时间
 * @param {string} [params.to] 结束时间
 * @returns {Promise<Object>} 统计信息（成功/失败数、平均耗时等）
 */
export function getRunStats(params) {
  return request({
    url: '/runs/stats',
    method: 'get',
    params
  })
}
