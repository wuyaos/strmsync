import request from './request'

// 获取日志列表
export function getLogList(params) {
  return request({
    url: '/logs',
    method: 'get',
    params
  })
}

// 清理日志
export function cleanupLogs(data) {
  return request({
    url: '/logs/cleanup',
    method: 'post',
    data
  })
}
