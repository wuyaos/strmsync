import axios from 'axios'
import { handleApiError, isCanceledError } from '@/utils/error'

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
    if (axios.isCancel?.(error) || isCanceledError(error)) {
      return Promise.reject(error)
    }
    const appError = handleApiError(error, { silent: error?.config?.silent })
    return Promise.reject(appError)
  }
)

export default request
