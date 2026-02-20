// Package strmwriter 定义STRM内容生成相关类型
package strmwriter

import (
	"fmt"
	"strings"
)

// StrmFormat 定义STRM内容的格式类型
//
// 不同的格式决定了STRM文件中存储的内容：
// - HTTP: 存储HTTP URL，适用于网络流媒体访问
// - Local: 存储本地文件路径，适用于本地挂载场景
type StrmFormat string

const (
	// StrmFormatHTTP HTTP URL格式
	// 生成如 "http://server:port/d/path/to/video.mp4" 的URL
	StrmFormatHTTP StrmFormat = "http"

	// StrmFormatLocal 本地路径格式
	// 生成如 "/mnt/media/path/to/video.mp4" 的本地路径
	StrmFormatLocal StrmFormat = "local"
)

// IsValid 检查格式是否有效
//
// 返回：
//   - true: 格式有效
//   - false: 格式无效或未知
func (f StrmFormat) IsValid() bool {
	switch f {
	case StrmFormatHTTP, StrmFormatLocal:
		return true
	default:
		return false
	}
}

// String 实现 fmt.Stringer 接口
func (f StrmFormat) String() string {
	return string(f)
}

// ServerID 数据服务器ID类型
//
// 使用自定义类型防止与其他ID类型混用，提高类型安全性
type ServerID uint

// RemoteMeta 远程文件元数据
//
// 用于传递额外的文件信息，如文件大小、修改时间等
// 使用 map[string]string 而非 map[string]interface{} 以避免类型断言错误
type RemoteMeta map[string]string

// BuildRequest 表示STRM内容生成请求
//
// 包含生成STRM内容所需的全部信息：
// - RemotePath: 远程文件的完整路径（Unix格式，如 "/Movies/Avatar.mp4"）
// - ServerID: 数据服务器ID，用于关联服务器配置
// - RemoteMeta: 可选的文件元数据，用于特殊场景（如签名URL）
type BuildRequest struct {
	RemotePath string     // 远程文件路径（必需）
	ServerID   ServerID   // 服务器ID（必需）
	RemoteMeta RemoteMeta // 可选元数据
}

// Validate 验证请求是否有效
//
// 返回：
//   - error: 验证失败时返回错误，成功返回 nil
func (r BuildRequest) Validate() error {
	if strings.TrimSpace(r.RemotePath) == "" {
		return fmt.Errorf("strmwriter: remote_path不能为空")
	}
	// 注意：ServerID 对于某些场景（如纯本地生成）可能为0
	// 调用方应根据具体场景判断是否需要 ServerID
	return nil
}

// BuildConfig STRM内容生成配置
//
// 不同格式需要不同的配置项：
//
// HTTP格式必需：
//   - BaseURL: 服务器基础URL（如 "http://127.0.0.1:19798"）
//   - URLPathPrefix: URL路径前缀（如 "/d"，OpenList使用）
//
// Local格式必需：
//   - LocalRoot: 本地挂载根路径（如 "/mnt/media"）
type BuildConfig struct {
	Format        StrmFormat // STRM格式（必需）
	BaseURL       string     // HTTP格式：基础URL（必需）
	URLPathPrefix string     // HTTP格式：路径前缀（可选，默认"/d"）
	LocalRoot     string     // Local格式：本地根路径（必需）
}

// Validate 验证配置是否符合格式要求
//
// 验证规则：
// - 格式必须有效
// - HTTP格式：BaseURL 必须非空
// - Local格式：LocalRoot 必须非空
//
// 返回：
//   - error: 验证失败时返回错误，成功返回 nil
func (c BuildConfig) Validate() error {
	if !c.Format.IsValid() {
		return fmt.Errorf("strmwriter: 无效的格式: %s", c.Format)
	}

	switch c.Format {
	case StrmFormatHTTP:
		if strings.TrimSpace(c.BaseURL) == "" {
			return fmt.Errorf("strmwriter: HTTP格式必须提供base_url")
		}
	case StrmFormatLocal:
		if strings.TrimSpace(c.LocalRoot) == "" {
			return fmt.Errorf("strmwriter: Local格式必须提供local_root")
		}
	}

	return nil
}

// WithDefaults 应用默认值
//
// 为可选字段设置合理的默认值：
// - HTTP格式：URLPathPrefix 默认为 "/d"
//
// 返回：
//   - BuildConfig: 应用默认值后的配置副本
func (c BuildConfig) WithDefaults() BuildConfig {
	cfg := c
	if cfg.Format == StrmFormatHTTP && strings.TrimSpace(cfg.URLPathPrefix) == "" {
		cfg.URLPathPrefix = "/d"
	}
	return cfg
}
