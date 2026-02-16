package adapters

import (
	"time"
)

// FileInfo 文件信息结构
// Author: STRMSync Team
type FileInfo struct {
	Path         string    `json:"path"`          // 文件路径（相对于数据源）
	Name         string    `json:"name"`          // 文件名
	Size         int64     `json:"size"`          // 文件大小（字节）
	ModTime      time.Time `json:"mod_time"`      // 修改时间
	IsDir        bool      `json:"is_dir"`        // 是否为目录
	IsVideo      bool      `json:"is_video"`      // 是否为视频文件
	IsMetadata   bool      `json:"is_metadata"`   // 是否为元数据文件
	RelativePath string    `json:"relative_path"` // 相对路径（用于数据库存储）
}

// ScanResult 扫描结果
// Author: STRMSync Team
type ScanResult struct {
	TotalFiles int         `json:"total_files"` // 总文件数
	VideoFiles int         `json:"video_files"` // 视频文件数
	Metadata   int         `json:"metadata"`    // 元数据文件数
	TotalSize  int64       `json:"total_size"`  // 总大小
	Files      []*FileInfo `json:"files"`       // 文件列表
}

// ScanOptions 扫描选项
// Author: STRMSync Team
type ScanOptions struct {
	Recursive    bool     `json:"recursive"`     // 是否递归扫描
	IncludeVideo bool     `json:"include_video"` // 是否包含视频文件
	IncludeMeta  bool     `json:"include_meta"`  // 是否包含元数据文件
	ExcludeNames []string `json:"exclude_names"` // 排除的文件/目录名
	MaxDepth     int      `json:"max_depth"`     // 最大递归深度（0 = 无限制）
}

// Adapter 数据源适配器接口
// Author: STRMSync Team
type Adapter interface {
	// GetType 获取适配器类型
	GetType() string

	// IsAvailable 检查数据源是否可用
	IsAvailable() error

	// ListFiles 列出指定路径下的文件
	ListFiles(path string, options *ScanOptions) ([]*FileInfo, error)

	// GetFileInfo 获取单个文件的详细信息
	GetFileInfo(path string) (*FileInfo, error)

	// Close 关闭适配器，释放资源
	Close() error
}

// AdapterConfig 适配器通用配置
// Author: STRMSync Team
type AdapterConfig struct {
	Type       string            `json:"type"`        // 适配器类型：local, clouddrive2, openlist
	Name       string            `json:"name"`        // 数据源名称
	BasePath   string            `json:"base_path"`   // 基础路径
	Enabled    bool              `json:"enabled"`     // 是否启用
	Properties map[string]string `json:"properties"`  // 其他属性（如 API URL、认证信息等）
}

// DefaultScanOptions 默认扫描选项
// Author: STRMSync Team
func DefaultScanOptions() *ScanOptions {
	return &ScanOptions{
		Recursive:    true,
		IncludeVideo: true,
		IncludeMeta:  true,
		ExcludeNames: []string{
			".tmp",
			".@__thumb",
			"@eaDir",
			"Thumbs.db",
			".DS_Store",
			"@Recycle",
			".Recycle.Bin",
		},
		MaxDepth: 0, // 无限制
	}
}

// ShouldExclude 判断文件/目录名是否应该被排除
// Author: STRMSync Team
func (opts *ScanOptions) ShouldExclude(name string) bool {
	for _, exclude := range opts.ExcludeNames {
		if name == exclude {
			return true
		}
	}
	return false
}
