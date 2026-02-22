const ERROR_CODES = {
  UNKNOWN: 'UNKNOWN',
  NETWORK: 'NETWORK',
  TIMEOUT: 'TIMEOUT',
  UNAUTHORIZED: 'UNAUTHORIZED',
  FORBIDDEN: 'FORBIDDEN',
  NOT_FOUND: 'NOT_FOUND',
  VALIDATION: 'VALIDATION',
  SERVER: 'SERVER',
  CANCELED: 'CANCELED'
}

class AppError extends Error {
  constructor(message, options = {}) {
    super(message || '请求失败')
    this.name = 'AppError'
    this.code = options.code || ERROR_CODES.UNKNOWN
    this.status = options.status
    this.data = options.data
    this.retryable = Boolean(options.retryable)
    this.cause = options.cause
  }
}

const shownMessages = new Set()
let notifier = null

export const ErrorCode = ERROR_CODES
export { AppError }

export const setErrorNotifier = (notify) => {
  notifier = typeof notify === 'function' ? notify : null
}

export const isCanceledError = (error) => {
  return (
    error?.code === 'ERR_CANCELED' ||
    error?.name === 'CanceledError' ||
    error?.message === 'canceled'
  )
}

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

const resolveHttpMessage = (status, data) => {
  const isObject = typeof data === 'object' && data !== null
  if (status === 400 || status === 422) {
    return (
      formatValidationErrors(data) ||
      (isObject ? data?.message : '') ||
      (isObject ? data?.error : '') ||
      '请求参数错误'
    )
  }
  if (status === 404) return '请求的资源不存在'
  if (status === 401) return '未授权访问'
  if (status === 403) return '访问被拒绝'
  if (status >= 500) {
    return (isObject ? data?.message : '') || (isObject ? data?.error : '') || '服务器错误'
  }
  return (isObject ? data?.message : '') || (isObject ? data?.error : '') || '请求失败'
}

const resolveHttpCode = (status) => {
  if (status === 400 || status === 422) return ERROR_CODES.VALIDATION
  if (status === 401) return ERROR_CODES.UNAUTHORIZED
  if (status === 403) return ERROR_CODES.FORBIDDEN
  if (status === 404) return ERROR_CODES.NOT_FOUND
  if (status >= 500) return ERROR_CODES.SERVER
  return ERROR_CODES.UNKNOWN
}

export const normalizeError = (error, options = {}) => {
  if (error instanceof AppError) return error
  if (isCanceledError(error)) {
    return new AppError('请求已取消', { code: ERROR_CODES.CANCELED, cause: error })
  }

  if (error?.response) {
    const status = error.response.status
    const data = error.response.data
    const message = resolveHttpMessage(status, data) || options.defaultMessage || '请求失败'
    const retryable = status === 408 || status === 429 || status >= 500
    return new AppError(message, { code: resolveHttpCode(status), status, data, retryable, cause: error })
  }

  if (error?.code === 'ECONNABORTED' || /timeout/i.test(error?.message || '')) {
    return new AppError('请求超时', { code: ERROR_CODES.TIMEOUT, retryable: true, cause: error })
  }

  if (error?.request) {
    return new AppError('网络连接失败', { code: ERROR_CODES.NETWORK, retryable: true, cause: error })
  }

  return new AppError(options.defaultMessage || error?.message || '请求失败', { cause: error })
}

const notifyError = (message, options = {}) => {
  if (!message) return
  const dedupe = options.dedupe !== false
  if (dedupe) {
    if (shownMessages.has(message)) return
    shownMessages.add(message)
    const ttl = Number.isFinite(options.dedupeMs) ? options.dedupeMs : 3000
    setTimeout(() => shownMessages.delete(message), ttl)
  }
  if (notifier) {
    notifier(message)
    return
  }
  console.warn(message)
}

export const handleApiError = (error, options = {}) => {
  const appError = normalizeError(error, { defaultMessage: options.defaultMessage })
  if (!options.silent && appError.code !== ERROR_CODES.CANCELED) {
    notifyError(appError.message)
  }
  if (typeof options.onError === 'function') {
    options.onError(appError)
  }
  return appError
}

export const installGlobalErrorHandlers = (app, options = {}) => {
  if (options.notify) {
    setErrorNotifier(options.notify)
  }
  const handler = (err, vm, info) => {
    const appError = normalizeError(err, { defaultMessage: options.defaultMessage })
    if (!options.silent && appError.code !== ERROR_CODES.CANCELED) {
      notifyError(appError.message)
    }
    if (typeof options.onError === 'function') {
      options.onError(appError, { vm, info })
    }
  }

  if (app && app.config) {
    app.config.errorHandler = handler
  }

  if (options.captureGlobal === false) return

  if (typeof window !== 'undefined') {
    window.addEventListener('error', (event) => {
      handler(event?.error || event?.message || new Error('未知错误'), null, 'window.error')
    })
    window.addEventListener('unhandledrejection', (event) => {
      handler(event?.reason || new Error('未处理的 Promise 错误'), null, 'window.unhandledrejection')
    })
  }
}
