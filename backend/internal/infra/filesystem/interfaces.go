// Package dataserver 数据服务器客户端接口
package filesystem

import (
	"context"
	"io"
)

// Client 数据服务器客户端接口
type Client interface {
	// Scan 流式扫描目录内容
	// maxDepth: 递归最大深度，0表示非递归，>0表示递归的最大层级
	// 返回条目通道与错误通道，扫描结束时必须关闭
	Scan(ctx context.Context, path string, recursive bool, maxDepth int) (<-chan RemoteFile, <-chan error)

	// Watch 监控目录变化（如果支持）
	Watch(ctx context.Context, path string) (<-chan FileEvent, error)

	// BuildStreamURL 构建流媒体URL
	BuildStreamURL(ctx context.Context, serverID uint, filePath string) (string, error)

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error

	// ResolveMountPath 将远端路径映射到本地挂载路径（若可用）
	// 返回本地文件系统路径，如果不支持挂载或路径无效则返回错误
	ResolveMountPath(ctx context.Context, remotePath string) (string, error)

	// ResolveAccessPath 将远端路径映射到本地访问路径（若可用）
	// 返回本地文件系统路径，如果未配置访问路径则返回错误
	ResolveAccessPath(ctx context.Context, remotePath string) (string, error)

	// Download 下载文件内容到writer（用于API下载）
	// 用于无法通过挂载路径访问文件时的回退方案
	Download(ctx context.Context, remotePath string, w io.Writer) error
}
