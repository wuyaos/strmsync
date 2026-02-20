/**
 * API响应标准化工具
 * 用于统一处理后端返回的列表数据结构
 */

/**
 * 标准化列表响应数据
 * 兼容多种后端返回格式，统一转换为 { list, total } 结构
 *
 * @param {Object|Array} response - API响应对象或数组
 * @returns {{ list: Array, total: number }} 标准化后的列表数据
 *
 * @example
 * // 后端返回 { data: { items: [...], total: 100 } }
 * const { list, total } = normalizeListResponse(response)
 *
 * // 后端返回 { list: [...], total: 50 }
 * const { list, total } = normalizeListResponse(response)
 *
 * // 后端直接返回数组 [...]
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

  // 提取list字段，支持多种命名方式
  const list =
    response?.data?.items ||
    response?.data?.list ||
    response?.items ||
    response?.list ||
    response?.servers ||
    response?.data ||
    []

  // 确保list是数组类型
  const normalizedList = Array.isArray(list) ? list : []

  // 提取total字段，支持多种结构
  const total =
    response?.data?.total ??
    response?.total ??
    response?.meta?.total ??
    response?.pagination?.total ??
    null

  // 确保total是数字类型，避免分页组件异常
  // 只有在total存在时才解析，否则使用list长度作为回退
  // 使用 Number.isFinite 避免将合法的 0 当作 falsy 处理
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
