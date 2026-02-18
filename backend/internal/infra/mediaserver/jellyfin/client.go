// Package mediaserver provides Jellyfin media server adapter implementation.
//
// This is an internal sub-package that shares the parent mediaserver package namespace.
// It provides the Jellyfin-specific adapter implementation that handles API endpoint
// paths and authentication headers.
//
// The jellyfinAdapter is created automatically by the parent package's factory function
// (newAdapter) based on server type. It should not be instantiated directly.
//
// Jellyfin-specific behaviors:
//   - API endpoints use standard paths (no prefix)
//   - Uses "Authorization: MediaBrowser Token=" header for authentication
package mediaserver

import (
	"fmt"
	"net/http"
)

// jellyfinAdapter Jellyfin服务器适配器
type jellyfinAdapter struct{}

// endpointPath 构建Jellyfin的API路径（不需要前缀）
func (jellyfinAdapter) endpointPath(path string) string {
	return path
}

// setAuthHeader 设置Jellyfin认证头（使用MediaBrowser格式）
func (jellyfinAdapter) setAuthHeader(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, apiKey))
}
