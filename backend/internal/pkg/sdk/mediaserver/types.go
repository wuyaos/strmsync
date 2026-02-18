// Package types 定义Service层的核心类型
package mediaserver

import "time"

// Type 媒体服务器类型
type Type string

const (
	// TypeEmby 表示Emby媒体服务器
	TypeEmby Type = "emby"
	// TypeJellyfin 表示Jellyfin媒体服务器
	TypeJellyfin Type = "jellyfin"
)

// String 返回字符串表示
func (t Type) String() string {
	return string(t)
}

// IsValid 验证Type是否有效
func (t Type) IsValid() bool {
	switch t {
	case TypeEmby, TypeJellyfin:
		return true
	default:
		return false
	}
}

// Config 媒体服务器配置
type Config struct {
	Type    Type          // 媒体服务器类型
	BaseURL string          // 服务端基础地址（如 http://localhost:8096）
	APIKey  string          // API Key
	Timeout time.Duration   // 请求超时时间（默认10秒）
}
