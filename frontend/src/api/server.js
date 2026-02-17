/**
 * 服务器配置API
 * 包括数据服务器和媒体服务器的配置管理
 */
import request from './request'

/**
 * 获取数据服务器配置
 * @returns {Promise} 数据服务器配置
 */
export function getDataServer() {
  return request({
    url: '/servers/data',
    method: 'get'
  })
}

/**
 * 更新数据服务器配置
 * @param {Object} data 配置数据
 * @returns {Promise}
 */
export function updateDataServer(data) {
  return request({
    url: '/servers/data',
    method: 'put',
    data
  })
}

/**
 * 获取媒体服务器配置
 * @returns {Promise} 媒体服务器配置
 */
export function getMediaServer() {
  return request({
    url: '/servers/media',
    method: 'get'
  })
}

/**
 * 更新媒体服务器配置
 * @param {Object} data 配置数据
 * @returns {Promise}
 */
export function updateMediaServer(data) {
  return request({
    url: '/servers/media',
    method: 'put',
    data
  })
}
