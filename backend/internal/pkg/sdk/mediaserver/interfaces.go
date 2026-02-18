// Package mediaserver 媒体服务器客户端接口
package mediaserver

import "context"

// Client 媒体服务器客户端接口
type Client interface {
	// Scan 触发媒体库扫描
	Scan(ctx context.Context, libraryPath string) error

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error
}
