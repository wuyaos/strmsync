/**
 * 服务器API
 * 统一管理数据服务器（DataServer）与媒体服务器（MediaServer）
 */
import request from './request'
import { DATA_SERVER_TYPES, MEDIA_SERVER_TYPES } from '@/constants/serverTypes'

/**
 * 序列化 options 字段
 * @param {Object} data 服务器数据
 * @returns {Object} 序列化后的数据
 */
function normalizeOptions(options) {
  if (!options) return {}
  if (typeof options === 'object') return options
  if (typeof options === 'string') {
    try {
      const parsed = JSON.parse(options)
      return parsed && typeof parsed === 'object' ? parsed : {}
    } catch (error) {
      return {}
    }
  }
  return {}
}

function serializeOptions(data) {
  if (!data) return data

  return {
    ...data,
    options: normalizeOptions(data.options)
  }
}

/**
 * 获取服务器列表
 * @param {Object} params 查询参数
 * @param {string} [params.category] 服务器分类（data/media）
 * @param {string} [params.type] 服务器分类（data/media）或类型（local/clouddrive2/emby等）
 * @param {string} [params.serverType] 服务器类型过滤（local/clouddrive2/emby等）
 * @param {string} [params.keyword] 搜索关键词
 * @param {number} [params.page] 页码
 * @param {number} [params.pageSize] 每页数量
 * @returns {Promise<{data: Array, meta: Object}>} 服务器列表及分页信息
 */
export function getServerList(params) {
  const { endpointCategory, queryParams } = resolveServerListParams(params)
  return request({
    url: `/servers/${endpointCategory}`,
    method: 'get',
    params: queryParams
  })
}

/**
 * 获取服务器类型定义列表
 * @param {Object} params 查询参数（可选）
 * @param {string} [params.category] 类型分类（data/media）
 * @returns {Promise<{types: Array}>} 类型定义列表
 */
export function getServerTypes(params) {
  return request({
    url: '/servers/types',
    method: 'get',
    params
  })
}

/**
 * 获取单个服务器类型定义
 * @param {string} type 服务器类型（local/clouddrive2/openlist等）
 * @returns {Promise<{type: Object}>} 类型定义
 */
export function getServerType(type) {
  return request({
    url: `/servers/types/${type}`,
    method: 'get'
  })
}

/**
 * 获取服务器详情
 * @param {string|number} id 服务器ID
 * @param {string} type 服务器类型（local/clouddrive2/emby等）
 * @returns {Promise<Object>} 服务器详情
 */
export function getServer(id, type) {
  const category = inferCategory(type)
  return request({
    url: `/servers/${category}/${id}`,
    method: 'get'
  })
}

/**
 * 根据服务器类型推断分类
 * @param {string} type 服务器类型
 * @returns {string} 分类（data/media）
 */
function inferCategory(type) {
  if (!type) {
    console.warn('[inferCategory] type is empty, defaulting to data')
    return 'data'
  }
  if (DATA_SERVER_TYPES.includes(type)) {
    return 'data'
  }
  if (MEDIA_SERVER_TYPES.includes(type)) {
    return 'media'
  }

  console.error(`[inferCategory] unknown type: ${type}, defaulting to data`)
  return 'data'
}

function resolveServerListParams(params) {
  const { category, serverType, type, ...restParams } = params || {}
  const endpointCategory = category || (type && type !== 'data' && type !== 'media'
    ? inferCategory(type)
    : (type || 'data'))
  if (serverType) {
    restParams.type = serverType
  } else if (type && type !== 'data' && type !== 'media') {
    restParams.type = type
  }
  return { endpointCategory, queryParams: restParams }
}

/**
 * 创建服务器
 * @param {Object} data 服务器数据
 * @param {string} data.type 服务器类型（必填）
 * @param {string} data.name 服务器名称
 * @param {string} data.host 主机地址
 * @param {number} data.port 端口
 * @param {string} data.api_key API密钥
 * @param {Object|string} data.options 扩展选项（JSON对象或字符串）
 * @param {boolean} [data.enabled=true] 是否启用
 * @returns {Promise<Object>} 创建的服务器
 */
export function createServer(data) {
  const payload = serializeOptions(data)
  const category = inferCategory(payload.type)
  return request({
    url: `/servers/${category}`,
    method: 'post',
    data: payload
  })
}

/**
 * 更新服务器
 * @param {string|number} id 服务器ID
 * @param {Object} data 服务器数据
 * @returns {Promise<Object>} 更新后的服务器
 */
export function updateServer(id, data) {
  const payload = serializeOptions(data)
  const category = inferCategory(payload.type)
  return request({
    url: `/servers/${category}/${id}`,
    method: 'put',
    data: payload
  })
}

/**
 * 删除服务器
 * @param {string|number} id 服务器ID
 * @param {string} type 服务器类型（local/clouddrive2/emby等）
 * @returns {Promise<void>}
 */
export function deleteServer(id, type) {
  const category = inferCategory(type)
  return request({
    url: `/servers/${category}/${id}`,
    method: 'delete'
  })
}

/**
 * 测试服务器连接
 * @param {string|number} id 服务器ID
 * @param {string} type 服务器类型（local/clouddrive2/emby等）
 * @returns {Promise<{success: boolean, message: string}>} 测试结果
 */
export function testServer(id, type) {
  const category = inferCategory(type)
  return request({
    url: `/servers/${category}/${id}/test`,
    method: 'post'
  })
}

// 静默测试服务器连接（不弹错误提示）
export function testServerSilent(id, type, options = {}) {
  const category = inferCategory(type)
  return request({
    url: `/servers/${category}/${id}/test`,
    method: 'post',
    silent: true,
    signal: options.signal
  })
}

/**
 * 临时测试服务器连接（未保存）
 * @param {Object} data 服务器数据
 * @returns {Promise<{success: boolean, message: string}>} 测试结果
 */
export function testServerTemp(data) {
  const payload = serializeOptions(data)
  const category = inferCategory(payload?.type)
  return request({
    url: `/servers/${category}/test`,
    method: 'post',
    data: payload
  })
}

/**
 * 列出指定路径下的目录
 * @param {Object} params 查询参数
 * @param {string} params.path 目标路径
 * @param {string} [params.mode] 访问模式（local/api）
 * @param {string} [params.type] 服务器类型（clouddrive2/openlist）
 * @param {string} [params.host] 主机地址（clouddrive2）
 * @param {number|string} [params.port] 端口（clouddrive2）
 * @param {string} [params.apiKey] API密钥（clouddrive2）
 * @param {number|string} [params.serverId] 服务器ID（openlist）
 * @param {string} [params.username] OpenList 用户名（openlist 临时）
 * @param {string} [params.password] OpenList 密码（openlist 临时）
 * @returns {Promise<{path: string, directories: string[]}>} 目录列表
 */
export function listDirectories(params) {
  return request({
    url: '/files/directories',
    method: 'get',
    params
  })
}
