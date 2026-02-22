import axios from 'axios'
import { ElMessage } from 'element-plus'

// 格式化字段验证错误信息
const formatValidationErrors = (data) => {
  if (!data || typeof data !== 'object') return ''
  const errors = data.errors
  if (!errors || typeof errors !== 'object') return ''

  const parts = []
  for (const [field, messages] of Object.entries(errors)) {
    if (Array.isArray(messages)) {
      for (const msg of messages) {
        if (msg) parts.push(`${field}: ${msg}`)
      }
      continue
    }
    if (typeof messages === 'string' && messages) {
      parts.push(`${field}: ${messages}`)
    }
  }
  return parts.join('；')
}

// 创建 axios 实例
const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

// 请求拦截器
request.interceptors.request.use(
  config => {
    // 可以在这里添加 token
    return config
  },
  error => {
    console.error('请求错误:', error)
    return Promise.reject(error)
  }
)

// 响应拦截器
request.interceptors.response.use(
  response => {
    return response.data
  },
  error => {
    console.error('响应错误:', error)

    if (error?.config?.silent) {
      return Promise.reject(error)
    }

    let message = '请求失败'
    if (error.response) {
      const data = error.response.data
      const isObject = typeof data === 'object' && data !== null
      switch (error.response.status) {
        case 400:
        case 422:
          message =
            formatValidationErrors(data) ||
            (isObject ? data?.message : '') ||
            (isObject ? data?.error : '') ||
            '请求参数错误'
          break
        case 404:
          message = '请求的资源不存在'
          break
        case 500:
          message = (isObject ? data?.message : '') || (isObject ? data?.error : '') || '服务器错误'
          break
        default:
          message = (isObject ? data?.message : '') || (isObject ? data?.error : '') || '请求失败'
      }
    } else if (error.request) {
      message = '网络连接失败'
    }

    ElMessage.error(message)
    return Promise.reject(error)
  }
)

export default request
