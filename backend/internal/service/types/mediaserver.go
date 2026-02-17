// Package types 定义Service层的核心类型
package types

import "time"

// MediaServerType 媒体服务器类型
type MediaServerType string

const (
	// MediaServerTypeEmby 表示Emby媒体服务器
	MediaServerTypeEmby MediaServerType = "emby"
	// MediaServerTypeJellyfin 表示Jellyfin媒体服务器
	MediaServerTypeJellyfin MediaServerType = "jellyfin"
)

// String 返回字符串表示
func (t MediaServerType) String() string {
	return string(t)
}

// IsValid 验证MediaServerType是否有效
func (t MediaServerType) IsValid() bool {
	switch t {
	case MediaServerTypeEmby, MediaServerTypeJellyfin:
		return true
	default:
		return false
	}
}

// MediaServerConfig 媒体服务器配置
type MediaServerConfig struct {
	Type    MediaServerType // 媒体服务器类型
	BaseURL string          // 服务端基础地址（如 http://localhost:8096）
	APIKey  string          // API Key
	Timeout time.Duration   // 请求超时时间（默认10秒）
}
