import request from './request'

// 获取数据源列表
export function getSourceList(params) {
  return request({
    url: '/sources',
    method: 'get',
    params
  })
}

// 获取数据源详情
export function getSource(id) {
  return request({
    url: `/sources/${id}`,
    method: 'get'
  })
}

// 创建数据源
export function createSource(data) {
  // 序列化config和options字段
  const payload = {
    ...data,
    config: typeof data.config === 'object' ? JSON.stringify(data.config) : data.config,
    options: typeof data.options === 'object' ? JSON.stringify(data.options) : data.options
  }
  return request({
    url: '/sources',
    method: 'post',
    data: payload
  })
}

// 更新数据源
export function updateSource(id, data) {
  // 序列化config和options字段
  const payload = {
    ...data,
    config: typeof data.config === 'object' ? JSON.stringify(data.config) : data.config,
    options: typeof data.options === 'object' ? JSON.stringify(data.options) : data.options
  }
  return request({
    url: `/sources/${id}`,
    method: 'put',
    data: payload
  })
}

// 删除数据源
export function deleteSource(id) {
  return request({
    url: `/sources/${id}`,
    method: 'delete'
  })
}

// 测试数据源连接
export function testSource(id) {
  return request({
    url: `/sources/${id}/test`,
    method: 'post'
  })
}

// 触发扫描
export function scanSource(id) {
  return request({
    url: `/sources/${id}/scan`,
    method: 'post'
  })
}

// 启动文件监控
export function startWatch(id) {
  return request({
    url: `/sources/${id}/watch/start`,
    method: 'post'
  })
}

// 停止文件监控
export function stopWatch(id) {
  return request({
    url: `/sources/${id}/watch/stop`,
    method: 'post'
  })
}

// 获取监控状态
export function getWatchStatus(id) {
  return request({
    url: `/sources/${id}/watch/status`,
    method: 'get'
  })
}

// 同步元数据
export function syncMetadata(id) {
  return request({
    url: `/sources/${id}/metadata/sync`,
    method: 'post'
  })
}

// 同步指定路径的元数据
export function syncMetadataPath(id, data) {
  return request({
    url: `/sources/${id}/metadata/sync/path`,
    method: 'post',
    data
  })
}

// 触发通知
export function triggerNotify(id) {
  return request({
    url: `/sources/${id}/notify/refresh`,
    method: 'post'
  })
}
