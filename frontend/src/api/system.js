/**
 * 系统信息API
 * 包含健康检查、版本信息等
 */
import request from './request'

/**
 * 获取系统健康状态
 * @returns {Promise} 系统信息
 */
export function getHealth() {
  return request({
    url: '/health',
    method: 'get'
  })
}
