// Package strmwriter 提供本地文件系统的 STRM 写入器实现
package strmwriter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// 默认目录权限：rwxr-xr-x (755)
	defaultDirPerm = 0o755

	// 默认文件权限：rw-r--r-- (644)
	defaultFilePerm = 0o644
)

// LocalWriter 实现本地文件系统的 STRM 文件写入
//
// 设计要点：
// - 支持根路径限制（enforceRoot），防止路径穿越攻击
// - 可配置目录和文件权限
// - 并发安全（无共享状态）
// - 所有操作尊重 context 取消
//
// 安全性：
// - 如果 enforceRoot=true，则所有路径必须在 root 之下
// - 使用 filepath.Rel 验证路径不会逃逸
type LocalWriter struct {
	root        string      // 根目录（为空则不限制）
	enforceRoot bool        // 是否强制路径必须在 root 之下
	dirPerm     os.FileMode // 创建目录时使用的权限
	filePerm    os.FileMode // 创建文件时使用的权限
}

// Option 是 LocalWriter 的配置选项函数
type Option func(*LocalWriter)

// WithEnforceRoot 控制是否强制路径必须在 root 之下
//
// 参数：
//   - enforce: true 表示强制路径验证，false 表示不限制
//
// 用途：
// - 生产环境建议设置为 true 以增强安全性
// - 测试环境可设置为 false 以方便测试
func WithEnforceRoot(enforce bool) Option {
	return func(w *LocalWriter) {
		w.enforceRoot = enforce
	}
}

// WithPermissions 设置目录和文件的权限
//
// 参数：
//   - dirPerm: 创建目录时使用的权限（如 0o755）
//   - filePerm: 创建文件时使用的权限（如 0o644）
//
// 默认值：
//   - dirPerm: 0o755 (rwxr-xr-x)
//   - filePerm: 0o644 (rw-r--r--)
func WithPermissions(dirPerm, filePerm os.FileMode) Option {
	return func(w *LocalWriter) {
		w.dirPerm = dirPerm
		w.filePerm = filePerm
	}
}

// NewLocalWriter 创建一个本地文件系统写入器
//
// 参数：
//   - root: 根目录路径（空字符串表示不限制根目录）
//   - opts: 可选配置项
//
// 返回：
//   - *LocalWriter: 写入器实例
//   - error: 配置无效时返回错误
//
// 行为：
// - 如果 root 非空，会将其转换为绝对路径
// - 如果 root 为空但 enforceRoot=true（通过 WithEnforceRoot），则返回错误
// - 默认使用 0o755 目录权限和 0o644 文件权限
//
// 示例：
//   // 不限制根目录
//   writer, _ := NewLocalWriter("")
//
//   // 限制在指定根目录下
//   writer, _ := NewLocalWriter("/mnt/media", WithEnforceRoot(true))
//
//   // 自定义权限
//   writer, _ := NewLocalWriter("/mnt/media",
//       WithEnforceRoot(true),
//       WithPermissions(0o750, 0o640))
func NewLocalWriter(root string, opts ...Option) (*LocalWriter, error) {
	writer := &LocalWriter{
		dirPerm:  defaultDirPerm,
		filePerm: defaultFilePerm,
	}

	// 如果 root 非空，转换为绝对路径
	if strings.TrimSpace(root) != "" {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			// 如果无法转换为绝对路径，使用 Clean 过的路径
			absRoot = filepath.Clean(root)
		}
		writer.root = absRoot
		writer.enforceRoot = true
	}

	// 应用配置选项
	for _, opt := range opts {
		opt(writer)
	}

	// 验证配置一致性
	if writer.enforceRoot && strings.TrimSpace(writer.root) == "" {
		return nil, fmt.Errorf("strmwriter: enforceRoot=true 时必须指定 root 路径")
	}

	return writer, nil
}

// Read 读取 STRM 文件内容
//
// 实现说明：
// - 检查 context 是否已取消
// - 验证路径（如果 enforceRoot=true）
// - 使用 os.ReadFile 读取
func (w *LocalWriter) Read(ctx context.Context, targetPath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := w.validatePath(targetPath); err != nil {
		return "", err
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("strmwriter: 读取 %s 失败: %w", targetPath, err)
	}
	return string(data), nil
}

// Write 创建或更新 STRM 文件并应用修改时间
//
// 实现说明：
// - 检查 context 是否已取消
// - 验证路径（如果 enforceRoot=true）
// - 确保父目录存在（使用 os.MkdirAll）
// - 写入文件内容（使用 os.WriteFile）
// - 如果 modTime 非零，设置文件修改时间（使用 os.Chtimes）
//
// 注意：
// - 文件存在则覆盖
// - 目录和文件的权限由配置决定
func (w *LocalWriter) Write(ctx context.Context, targetPath string, content string, modTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.validatePath(targetPath); err != nil {
		return err
	}

	// 确保父目录存在
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, w.dirPerm); err != nil {
		return fmt.Errorf("strmwriter: 创建目录 %s 失败: %w", targetDir, err)
	}

	// 写入文件
	if err := os.WriteFile(targetPath, []byte(content), w.filePerm); err != nil {
		return fmt.Errorf("strmwriter: 写入 %s 失败: %w", targetPath, err)
	}

	// 设置修改时间（如果提供）
	// 注意：修改时间设置失败不会阻断流程，但会记录在日志中（未来实现）
	if !modTime.IsZero() {
		_ = os.Chtimes(targetPath, modTime, modTime)
		// 忽略 Chtimes 错误：某些文件系统不支持修改时间
	}

	return nil
}

// Delete 删除 STRM 文件
//
// 实现说明：
// - 检查 context 是否已取消
// - 验证路径（如果 enforceRoot=true）
// - 使用 os.Remove 删除文件
// - 文件不存在时不返回错误（幂等操作）
func (w *LocalWriter) Delete(ctx context.Context, targetPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.validatePath(targetPath); err != nil {
		return err
	}
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("strmwriter: 删除 %s 失败: %w", targetPath, err)
	}
	return nil
}

// validatePath 验证路径是否在允许的范围内
//
// 安全检查：
// - 如果 enforceRoot=false，直接通过
// - 如果 enforceRoot=true，确保路径在 root 之下
// - 相对路径会被视为相对于 root
// - 使用 filepath.Rel 计算相对路径
// - 检查相对路径是否以 ".." 开头（路径穿越）
//
// 返回：
//   - nil: 路径有效
//   - error: 路径无效或逃逸
func (w *LocalWriter) validatePath(targetPath string) error {
	if !w.enforceRoot {
		return nil
	}

	// 处理相对路径：如果路径不是绝对路径，将其视为相对于 root
	var absTarget string
	if filepath.IsAbs(targetPath) {
		absTarget = filepath.Clean(targetPath)
	} else {
		// 相对路径：相对于 root
		absTarget = filepath.Clean(filepath.Join(w.root, targetPath))
	}

	rel, err := filepath.Rel(w.root, absTarget)
	if err != nil {
		return fmt.Errorf("strmwriter: 验证路径 %s 失败: %w", targetPath, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("strmwriter: 路径 %s 逃逸了根目录 %s", targetPath, w.root)
	}
	return nil
}
