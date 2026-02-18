// Package mediaserver 实现Emby媒体服务器客户端
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
