// Package mediaserver 实现Jellyfin媒体服务器客户端
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
