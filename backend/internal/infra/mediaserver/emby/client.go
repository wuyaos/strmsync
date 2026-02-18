// Package mediaserver provides Emby media server adapter implementation.
//
// This is an internal sub-package that shares the parent mediaserver package namespace.
// It provides the Emby-specific adapter implementation that handles API endpoint
// paths and authentication headers.
//
// The embyAdapter is created automatically by the parent package's factory function
// (newAdapter) based on server type. It should not be instantiated directly.
//
// Emby-specific behaviors:
//   - API endpoints require "/emby" prefix
//   - Uses "X-Emby-Token" header for authentication
package mediaserver

import (
	"net/http"
)

// embyAdapter Emby服务器适配器
type embyAdapter struct{}

// endpointPath 构建Emby的API路径（需要/emby前缀）
func (embyAdapter) endpointPath(path string) string {
	return "/emby" + path
}

// setAuthHeader 设置Emby认证头
func (embyAdapter) setAuthHeader(req *http.Request, apiKey string) {
	req.Header.Set("X-Emby-Token", apiKey)
}
