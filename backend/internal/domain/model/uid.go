// Package model 提供 UID 生成工具
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// GenerateDataServerUID 生成 DataServer 的唯一标识
//
// UID 基于：type + host + port + normalized options + apikey
// 使用 SHA-256 hash，返回 hex 编码（64字符）
func GenerateDataServerUID(serverType, host string, port int, optionsJSON, apiKey string) (string, error) {
	input, err := buildUIDInput(serverType, host, port, optionsJSON, apiKey)
	if err != nil {
		return "", fmt.Errorf("build uid input: %w", err)
	}
	return computeUID(input), nil
}

// BuildDataServerUIDForUpdate 为更新场景生成 UID 并判断是否变化
//
// 返回值：
// - newUID: 新计算的 UID
// - changed: UID 是否发生变化
// - error: 生成过程中的错误
func BuildDataServerUIDForUpdate(currentUID, serverType, host string, port int, optionsJSON, apiKey string) (string, bool, error) {
	newUID, err := GenerateDataServerUID(serverType, host, port, optionsJSON, apiKey)
	if err != nil {
		return "", false, err
	}
	return newUID, newUID != currentUID, nil
}

// GenerateMediaServerUID 生成 MediaServer 的唯一标识
//
// UID 基于：type + host + port + normalized options + apikey
// 使用 SHA-256 hash，返回 hex 编码（64字符）
func GenerateMediaServerUID(serverType, host string, port int, optionsJSON, apiKey string) (string, error) {
	input, err := buildUIDInput(serverType, host, port, optionsJSON, apiKey)
	if err != nil {
		return "", fmt.Errorf("build uid input: %w", err)
	}
	return computeUID(input), nil
}

// BuildMediaServerUIDForUpdate 为更新场景生成 UID 并判断是否变化
//
// 返回值：
// - newUID: 新计算的 UID
// - changed: UID 是否发生变化
// - error: 生成过程中的错误
func BuildMediaServerUIDForUpdate(currentUID, serverType, host string, port int, optionsJSON, apiKey string) (string, bool, error) {
	newUID, err := GenerateMediaServerUID(serverType, host, port, optionsJSON, apiKey)
	if err != nil {
		return "", false, err
	}
	return newUID, newUID != currentUID, nil
}

// buildUIDInput 构建 UID 生成的输入字符串
//
// 使用结构化JSON序列化避免字段拼接歧义
// 格式：{"type":"...","host":"...","port":123,"options":"...","api_key":"..."}
func buildUIDInput(serverType, host string, port int, optionsJSON, apiKey string) (string, error) {
	// 1. normalize host（lower + trim）
	normalizedHost := strings.ToLower(strings.TrimSpace(host))

	// 2. 移除 options 中的 QoS 字段（避免 QoS 变更影响 UID）
	optionsWithoutQoS := stripQoSFields(optionsJSON)

	// 3. normalize options（递归排序 + 过滤空值）
	normalizedOptions, err := normalizeJSON(optionsWithoutQoS)
	if err != nil {
		return "", fmt.Errorf("normalize options: %w", err)
	}

	// 4. 构建结构化输入（使用结构体确保字段顺序稳定）
	input := struct {
		Type    string `json:"type"`
		Host    string `json:"host"`
		Port    int    `json:"port"`
		Options string `json:"options"`
		APIKey  string `json:"api_key"`
	}{
		Type:    strings.TrimSpace(serverType),
		Host:    normalizedHost,
		Port:    port,
		Options: normalizedOptions,
		APIKey:  strings.TrimSpace(apiKey),
	}

	// 5. JSON序列化（Go的struct序列化保证字段顺序）
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal uid input: %w", err)
	}

	return string(data), nil
}

// computeUID 计算 SHA-256 hash 并返回 hex 编码
func computeUID(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// normalizeJSON 归一化 JSON 字符串
//
// 步骤：
// 1. 解析 JSON 为 map（如果不是 object，直接返回原值）
// 2. 递归排序 key（确保输出稳定）
// 3. 过滤空值（null、空字符串、空数组、空对象；保留 false、0 等显式配置值）
// 4. 重新序列化为稳定 JSON
func normalizeJSON(jsonStr string) (string, error) {
	// 空字符串视为空对象
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return "{}", nil
	}

	// 解析 JSON
	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// 解析失败时，返回原字符串（兼容非 JSON）
		return jsonStr, nil
	}

	// 归一化
	normalized := normalizeValue(data)

	// nil结果（空对象或全部值被过滤）统一视为空配置，与空字符串输入保持一致
	if normalized == nil {
		return "{}", nil
	}

	// 序列化（稳定 JSON）
	result, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal normalized json: %w", err)
	}

	return string(result), nil
}

// normalizeValue 递归归一化 JSON 值
//
// 规则：
// - map: 排序 key，递归归一化 value，过滤空值
// - slice: 递归归一化元素，过滤空值
// - 基础类型: 直接返回（不过滤非空值如 false、0）
func normalizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		// 排序 key 并递归归一化
		normalized := make(map[string]any)
		for _, k := range sortedKeys(val) {
			normV := normalizeValue(val[k])
			if !isEmptyValue(normV) {
				normalized[k] = normV
			}
		}
		// 空 map 视为 nil
		if len(normalized) == 0 {
			return nil
		}
		return normalized

	case []any:
		// 递归归一化数组元素
		normalized := make([]any, 0, len(val))
		for _, v := range val {
			normV := normalizeValue(v)
			if !isEmptyValue(normV) {
				normalized = append(normalized, normV)
			}
		}
		// 空数组视为 nil
		if len(normalized) == 0 {
			return nil
		}
		return normalized

	default:
		// 基础类型（string/int/float/bool/null）直接返回
		return v
	}
}

// isEmptyValue 判断是否为"空值"（应该被过滤）
//
// 空值定义：
// - nil
// - 空字符串 ""
// - 空 map
// - 空 slice
//
// 注意：false、0、0.0 等显式配置值不视为空值，保留原值。
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case map[string]any:
		return len(val) == 0
	case []any:
		return len(val) == 0
	default:
		return false
	}
}

// sortedKeys 返回 map 的排序后的 key 列表（用于稳定遍历）
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// stripQoSFields 从 options JSON 中移除 QoS 字段
//
// 目的：确保 QoS 配置变更不会影响服务器 UID，因为 QoS 现已独立为数据库列。
// 此函数同时兼容历史数据（可能将 QoS 存入 options 的情况）。
//
// 移除的字段：
// - request_timeout_ms
// - connect_timeout_ms
// - retry_max
// - retry_backoff_ms
// - max_concurrent
// - qos (整个嵌套对象)
func stripQoSFields(optionsJSON string) string {
	optionsJSON = strings.TrimSpace(optionsJSON)
	if optionsJSON == "" {
		return optionsJSON
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(optionsJSON), &obj); err != nil {
		// 非法 JSON 直接返回原值（兼容非 JSON 格式）
		return optionsJSON
	}

	// 移除根级 QoS 字段
	delete(obj, "request_timeout_ms")
	delete(obj, "connect_timeout_ms")
	delete(obj, "retry_max")
	delete(obj, "retry_backoff_ms")
	delete(obj, "max_concurrent")

	// 移除嵌套 qos 对象（如果存在）
	delete(obj, "qos")

	out, err := json.Marshal(obj)
	if err != nil {
		return optionsJSON
	}
	return string(out)
}

