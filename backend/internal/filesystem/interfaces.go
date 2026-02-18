// Package dataserver 数据服务器客户端接口
package filesystem

import (
	"context"
	
)

// Client 数据服务器客户端接口
type Client interface {
	// List 列出目录内容
	// maxDepth: 递归最大深度，0表示非递归，>0表示递归的最大层级
	List(ctx context.Context, path string, recursive bool, maxDepth int) ([]RemoteFile, error)

	// Watch 监控目录变化（如果支持）
	Watch(ctx context.Context, path string) (<-chan FileEvent, error)

	// BuildStreamURL 构建流媒体URL
	BuildStreamURL(ctx context.Context, serverID uint, filePath string) (string, error)

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error
}
