// Package types 定义Service层的核心类型
package filesystem

import "time"

// Type 数据服务器类型
type Type string

const (
	// TypeCloudDrive2 表示CloudDrive2数据服务器
	TypeCloudDrive2 Type = "clouddrive2"
	// TypeOpenList 表示OpenList数据服务器
	TypeOpenList Type = "openlist"
	// TypeLocal 表示本地文件系统
	TypeLocal Type = "local"
	// 未来可扩展: TypeAList 等
)

// String 返回字符串表示
func (t Type) String() string {
	return string(t)
}

// IsValid 验证Type是否有效
func (t Type) IsValid() bool {
	switch t {
	case TypeCloudDrive2, TypeOpenList, TypeLocal:
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

// Config 数据服务器配置
type Config struct {
	Type     Type     // 数据服务器类型
	BaseURL  string   // API地址（如 http://localhost:5244）
	Username string   // 用户名（用于登录认证）
	Password string   // 密码（用于登录认证或目录密码）
	STRMMode STRMMode // STRM模式
	// MountPath 本地挂载路径（mount模式必需）
	// 例如：/mnt/openlist 或 D:\mnt\openlist
	MountPath string
	// StrmMountPath STRM 生成使用的挂载路径（仅本地/挂载模式）
	// 为空时默认使用 MountPath
	StrmMountPath string
	Timeout       time.Duration // 请求超时时间（默认10秒）
}

// RemoteFile 远程文件信息
type RemoteFile struct {
	Path    string    // 文件路径
	Name    string    // 文件名
	Size    int64     // 文件大小
	ModTime time.Time // 修改时间
	IsDir   bool      // 是否为目录
}

// FileEvent 文件事件
type FileEvent struct {
	Type    string    // 事件类型
	Path    string    // 文件路径
	AbsPath string    // 绝对路径
	ModTime time.Time // 修改时间
	Size    int64     // 文件大小
	IsDir   bool      // 是否为目录
}
