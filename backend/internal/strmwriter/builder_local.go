// Package strmwriter 提供Local格式的STRM内容生成
package strmwriter

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// LocalBuilder 生成本地路径格式的STRM内容
//
// 适用场景：
// - 数据服务器通过FUSE/WebDAV挂载到本地文件系统
// - 媒体服务器可以直接访问挂载点
// - 不需要通过网络API访问
//
// 生成逻辑：
// 1. 清理远程路径（Unix格式）
// 2. 去除前导斜杠，转换为相对路径
// 3. 转换为本地路径分隔符
// 4. 拼接到本地根路径
//
// 示例：
//
//	builder, _ := NewLocalBuilder(BuildConfig{
//	    Format:    StrmFormatLocal,
//	    LocalRoot: "/mnt/clouddrive",
//	})
//	content, _ := builder.Build(ctx, BuildRequest{
//	    RemotePath: "/115/Movies/Avatar.mp4",
//	    ServerID:   1,
//	})
//	// 结果: "/mnt/clouddrive/115/Movies/Avatar.mp4"
type LocalBuilder struct {
	root string // 本地挂载根路径（绝对路径）
}

// NewLocalBuilder 创建Local格式生成器
//
// 参数：
//   - cfg: 构建配置，必须包含 LocalRoot
//
// 返回：
//   - *LocalBuilder: 生成器实例
//   - error: 配置无效时返回错误
//
// 验证：
// - 格式必须是 StrmFormatLocal
// - LocalRoot 必须非空
func NewLocalBuilder(cfg BuildConfig) (*LocalBuilder, error) {
	// 确保格式正确
	cfg.Format = StrmFormatLocal

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("strmwriter: LocalBuilder配置无效: %w", err)
	}

	// 清理并规范化根路径
	root := filepath.Clean(cfg.LocalRoot)

	return &LocalBuilder{root: root}, nil
}

// Build 实现 ContentBuilder 接口
//
// 生成逻辑：
// 1. 验证请求参数
// 2. 清理远程路径（统一使用Unix格式）
// 3. 去除前导斜杠，得到相对路径
// 4. 转换路径分隔符为本地格式
// 5. 拼接到根路径
//
// 参数：
//   - ctx: 上下文（检查取消，nil会自动转为Background）
//   - req: 生成请求
//
// 返回：
//   - string: 本地文件路径（绝对路径）
//   - error: 生成失败时返回错误
//
// 错误情况：
// - context 已取消
// - 远程路径为空
func (b *LocalBuilder) Build(ctx context.Context, req BuildRequest) (string, error) {
	// 防御性编程：nil context 自动转为 Background
	if ctx == nil {
		ctx = context.Background()
	}

	// 检查 context 取消
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("strmwriter: context已取消: %w", err)
	}

	// 验证请求
	if err := req.Validate(); err != nil {
		return "", fmt.Errorf("strmwriter: LocalBuilder请求验证失败: %w", err)
	}

	// 清理远程路径（Unix格式）
	cleanPath := cleanRemotePath(req.RemotePath)

	// 去除前导斜杠，得到相对路径
	rel := strings.TrimPrefix(cleanPath, "/")

	// 转换为本地路径分隔符（Windows: \, Unix: /）
	local := filepath.FromSlash(rel)

	// 拼接到根路径
	fullPath := filepath.Join(b.root, local)

	return fullPath, nil
}
