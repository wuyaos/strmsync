// Package handlers 提供HTTP处理器和辅助函数
package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ================== 错误响应结构 ==================

// FieldError 字段级验证错误
type FieldError struct {
	Field   string `json:"field"`   // 字段名称
	Message string `json:"message"` // 错误消息
}

// ErrorResponse 统一的错误响应格式
type ErrorResponse struct {
	Code        string       `json:"code"`                  // 错误代码（如：invalid_request）
	Message     string       `json:"message"`               // 错误消息
	FieldErrors []FieldError `json:"field_errors,omitempty"` // 字段级错误（可选）
}

// respondError 返回统一格式的错误响应
func respondError(c *gin.Context, status int, code, message string, fieldErrors []FieldError) {
	c.JSON(status, ErrorResponse{
		Code:        code,
		Message:     message,
		FieldErrors: fieldErrors,
	})
}

// respondValidationError 返回参数验证错误（400）
func respondValidationError(c *gin.Context, fieldErrors []FieldError) {
	respondError(c, http.StatusBadRequest, "invalid_request", "请求参数无效", fieldErrors)
}

// ================== 分页参数 ==================

// Pagination 分页参数结构
type Pagination struct {
	Page     int `json:"page"`      // 页码（从1开始）
	PageSize int `json:"page_size"` // 每页大小
	Offset   int `json:"offset"`    // SQL偏移量
}

// parsePagination 解析分页参数
// defaultPage: 默认页码（通常为1）
// defaultPageSize: 默认每页大小
// maxPageSize: 最大每页大小（0表示不限制）
func parsePagination(c *gin.Context, defaultPage, defaultPageSize, maxPageSize int) Pagination {
	page := parseIntQuery(c, "page", defaultPage)
	pageSize := parseIntQuery(c, "page_size", defaultPageSize)

	// 确保页码至少为1
	if page < 1 {
		page = defaultPage
	}

	// 确保每页大小至少为1
	if pageSize < 1 {
		pageSize = defaultPageSize
	}

	// 限制最大每页大小
	if maxPageSize > 0 && pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return Pagination{
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
	}
}

// ================== 参数解析 ==================

// parseIntQuery 解析查询参数为整数
func parseIntQuery(c *gin.Context, key string, defaultValue int) int {
	val := strings.TrimSpace(c.Query(key))
	if val == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return result
}

// parseUintParam 解析路径参数为uint64
func parseUintParam(c *gin.Context, key string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(c.Param(key)), 10, 64)
}

// ================== 参数验证 ==================

// validateRequiredString 验证必填字符串字段
func validateRequiredString(field, value string, errs *[]FieldError) {
	if strings.TrimSpace(value) == "" {
		*errs = append(*errs, FieldError{
			Field:   field,
			Message: "必填字段不能为空",
		})
	}
}

// validateEnum 验证枚举值字段
func validateEnum(field, value string, allowed []string, errs *[]FieldError) {
	value = strings.TrimSpace(value)
	for _, allowedValue := range allowed {
		if value == allowedValue {
			return
		}
	}
	*errs = append(*errs, FieldError{
		Field:   field,
		Message: "值必须是以下之一: " + strings.Join(allowed, ", "),
	})
}

// validatePort 验证端口号范围
func validatePort(field string, port int, errs *[]FieldError) {
	if port < 1 || port > 65535 {
		*errs = append(*errs, FieldError{
			Field:   field,
			Message: "端口范围必须在1-65535之间",
		})
	}
}

// validateJSONString 验证JSON字符串格式
// 空字符串被视为合法（表示没有额外配置）
func validateJSONString(field, jsonStr string, errs *[]FieldError) {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return
	}

	var js interface{}
	if err := json.Unmarshal([]byte(jsonStr), &js); err != nil {
		*errs = append(*errs, FieldError{
			Field:   field,
			Message: "无效的JSON格式: " + err.Error(),
		})
	}
}

// ================== 安全日志 ==================

// redactSensitive 对敏感字段进行脱敏处理
// 支持的敏感字段：api_key, apikey, token, secret, password
func redactSensitive(key, value string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	isSensitive := strings.Contains(k, "api_key") ||
		strings.Contains(k, "apikey") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "password")

	if !isSensitive {
		return value
	}

	// 保留后4位，其余用*代替
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}

// sanitizeMapForLog 对映射中的敏感字段进行脱敏
func sanitizeMapForLog(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}

	sanitized := make(map[string]interface{}, len(fields))
	for key, val := range fields {
		if strVal, ok := val.(string); ok {
			sanitized[key] = redactSensitive(key, strVal)
		} else {
			sanitized[key] = val
		}
	}
	return sanitized
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ================== 连接测试 ==================

// ConnectionTestResult 连接测试结果
type ConnectionTestResult struct {
	Success   bool                   `json:"success"`              // 是否成功
	Message   string                 `json:"message"`              // 结果消息
	LatencyMs int64                  `json:"latency_ms,omitempty"` // 延迟（毫秒）
	Details   map[string]interface{} `json:"details,omitempty"`    // 详细信息
}

// ================== SSRF防护 ==================

// validateHostForSSRF 验证主机地址以防止SSRF攻击
// 支持IPv4、IPv6和域名，拒绝回环/link-local地址，对内网地址警告但允许
// 返回：是否允许, 是否为内网地址, 错误消息
func validateHostForSSRF(hostRaw string) (allowed bool, isPrivate bool, message string) {
	// 1. 基础规范化
	host := strings.TrimSpace(hostRaw)
	if host == "" {
		return false, false, "主机地址不能为空"
	}

	// 拒绝协议前缀
	if strings.Contains(host, "://") {
		return false, false, "主机地址不应包含协议前缀（如http://）"
	}

	// 2. 解析host和port（剥离端口）
	var hostOnly string
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostOnly = h
	} else {
		// SplitHostPort失败
		// 如果包含冒号但无法解析，拒绝（可能是格式错误）
		if strings.Contains(host, ":") {
			return false, false, "主机地址格式错误（包含冒号但无法解析端口）"
		}
		// 无端口的host
		hostOnly = host
	}

	// 去掉IPv6方括号
	hostOnly = strings.Trim(hostOnly, "[]")
	lower := strings.ToLower(hostOnly)

	// 3. 快速拒绝明显危险的字符串
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return false, true, "不允许访问localhost"
	}
	if lower == "0.0.0.0" {
		return false, true, "不允许访问0.0.0.0"
	}

	// 4. 判断是否为字面IP
	ip := net.ParseIP(hostOnly)
	if ip != nil {
		// 是IP地址，直接做CIDR分类
		return classifyIP(ip)
	}

	// 5. 是域名，需要DNS解析
	return validateDomain(hostOnly)
}

// classifyIP 对IP地址进行分类（回环/link-local/私网/公网）
func classifyIP(ip net.IP) (allowed bool, isPrivate bool, message string) {
	// 定义各类地址范围
	var (
		loopbackIPv4   = net.IPNet{IP: net.ParseIP("127.0.0.0"), Mask: net.CIDRMask(8, 32)}
		loopbackIPv6   = net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)}
		linkLocalIPv4  = net.IPNet{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)}
		linkLocalIPv6  = net.IPNet{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)}
		privateIPv4_10 = net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)}
		privateIPv4_12 = net.IPNet{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)}
		privateIPv4_16 = net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)}
		privateIPv6ULA = net.IPNet{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)}
	)

	// 检查回环地址（拒绝）
	if loopbackIPv4.Contains(ip) || loopbackIPv6.Contains(ip) {
		return false, true, "不允许访问回环地址"
	}

	// 检查link-local地址（拒绝）
	if linkLocalIPv4.Contains(ip) || linkLocalIPv6.Contains(ip) {
		return false, true, "不允许访问link-local地址"
	}

	// 检查私网地址（允许但标记）
	if privateIPv4_10.Contains(ip) || privateIPv4_12.Contains(ip) || privateIPv4_16.Contains(ip) || privateIPv6ULA.Contains(ip) {
		return true, true, ""
	}

	// 处理IPv4-mapped IPv6 (::ffff:127.0.0.1)
	if v4 := ip.To4(); v4 != nil {
		// 转换为IPv4再检查
		return classifyIP(v4)
	}

	// 公网地址
	return true, false, ""
}

// validateDomain 验证域名并解析DNS
func validateDomain(domain string) (allowed bool, isPrivate bool, message string) {
	// 尝试DNS解析
	ips, err := net.LookupIP(domain)
	if err != nil {
		// 解析失败，允许但提示（避免因DNS问题影响用户）
		return true, false, ""
	}

	if len(ips) == 0 {
		return true, false, ""
	}

	// 检查所有解析到的IP
	hasPrivate := false
	for _, ip := range ips {
		allowed, isPrivate, msg := classifyIP(ip)
		if !allowed {
			// 如果任何一个IP是危险的，拒绝整个域名
			return false, true, msg
		}
		if isPrivate {
			hasPrivate = true
		}
	}

	// 如果有任何IP是私网，标记为私网
	return true, hasPrivate, ""
}
