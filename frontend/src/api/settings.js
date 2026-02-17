/**
 * 系统设置API
 * 包括扫描配置、日志配置、主题设置、通知样式等
 */
import request from './request'

/**
 * 获取系统设置
 * @returns {Promise} 系统设置
 */
export function getSettings() {
  return request({
    url: '/settings',
    method: 'get'
  })
}

/**
 * 更新系统设置
 * @param {Object} data 设置数据
 * @returns {Promise}
 */
export function updateSettings(data) {
  return request({
    url: '/settings',
    method: 'put',
    data
  })
}
