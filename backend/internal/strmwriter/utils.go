// Package strmwriter 提供共享的工具函数
package strmwriter

import (
	"path"
	"strings"
)

// cleanRemotePath 清理并规范化远程路径
//
// 规范化逻辑：
// - 空路径或"/"返回"/"
// - 添加前导斜杠（如果没有）
// - 使用 path.Clean 清理路径（去除 .., ., 多余斜杠等）
// - 去除尾部斜杠（除非是根路径）
//
// 注意：
// - 这里使用 path.Clean 而非 filepath.Clean
// - 因为远程路径总是Unix格式，与本地操作系统无关
// - path.Clean 会自动处理：
//   - 多个连续斜杠 -> 单个斜杠
//   - . 段 -> 移除
//   - .. 段 -> 正确解析（防止路径逃逸）
//
// 参数：
//   - p: 原始远程路径
//
// 返回：
//   - string: 清理后的路径（Unix格式，带前导斜杠）
func cleanRemotePath(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}

	// 确保前导斜杠
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}

	// 使用 path.Clean（Unix风格）清理路径
	// path.Clean 会：
	// 1. 将多个斜杠替换为单个斜杠
	// 2. 消除 . 元素
	// 3. 消除内部的 .. 元素及其前面的元素
	// 4. 消除开头的 .. 元素（防止逃逸）
	cleaned := path.Clean(trimmed)

	// path.Clean 会将单独的 "/" 保留为 "/"
	// 但会将 "/foo/" 清理为 "/foo"
	return cleaned
}
