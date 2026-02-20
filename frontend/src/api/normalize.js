/**
 * API响应标准化工具
 * 用于统一处理后端返回的列表数据结构
 */

/**
 * 标准化列表响应数据
 * 统一转换为 { list, total } 结构
 *
 * @param {Object|Array} response - API响应对象或数组
 * @returns {{ list: Array, total: number }} 标准化后的列表数据
 *
 * @example
 * // 后端返回 { servers: [...], total: 50 }
 * const { list, total } = normalizeListResponse(response)
 */
export function normalizeListResponse(response) {
  // 处理直接返回数组的情况
  if (Array.isArray(response)) {
    return {
      list: response,
      total: response.length
    }
  }

  const list =
    response?.servers ||
    response?.jobs ||
    response?.runs ||
    response?.logs ||
    []

  // 确保list是数组类型
  const normalizedList = Array.isArray(list) ? list : []

  const total = response?.total ?? null

  // 确保total是数字类型，避免分页组件异常
  let normalizedTotal
  if (total !== null && total !== undefined) {
    const parsedTotal = Number(total)
    normalizedTotal = Number.isFinite(parsedTotal) ? parsedTotal : normalizedList.length
  } else {
    normalizedTotal = normalizedList.length
  }

  return {
    list: normalizedList,
    total: normalizedTotal
  }
}
