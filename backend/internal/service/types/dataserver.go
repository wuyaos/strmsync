// Package types 定义Service层的核心类型
package types

import "time"

// DataServerType 数据服务器类型
type DataServerType string

const (
	// DataServerTypeOpenList 表示OpenList数据服务器
	DataServerTypeOpenList DataServerType = "openlist"
	// 未来可扩展: DataServerTypeAList, DataServerTypeCloudDrive2 等
)

// String 返回字符串表示
func (t DataServerType) String() string {
	return string(t)
}

// IsValid 验证DataServerType是否有效
func (t DataServerType) IsValid() bool {
	switch t {
	case DataServerTypeOpenList:
		return true
	default:
		return false
	}
}

// STRMMode STRM链接生成模式
type STRMMode string

const (
	// STRMModeHTTP 表示HTTP下载链接模式（通过API下载）
	STRMModeHTTP STRMMode = "http"
	// STRMModeMount 表示本地挂载路径模式（直接访问挂载目录）
	STRMModeMount STRMMode = "mount"
)

// String 返回字符串表示
func (m STRMMode) String() string {
	return string(m)
}

// IsValid 验证STRMMode是否有效
func (m STRMMode) IsValid() bool {
	switch m {
	case STRMModeHTTP, STRMModeMount:
		return true
	default:
		return false
	}
}

// DataServerConfig 数据服务器配置
type DataServerConfig struct {
	Type     DataServerType // 数据服务器类型
	BaseURL  string         // API地址（如 http://localhost:5244）
	Username string         // 用户名（用于登录认证）
	Password string         // 密码（用于登录认证或目录密码）
	STRMMode STRMMode       // STRM模式
	// MountPath 本地挂载路径（mount模式必需）
	// 例如：/mnt/openlist 或 D:\mnt\openlist
	MountPath string
	Timeout   time.Duration // 请求超时时间（默认10秒）
}
