// Package syncengine 定义同步引擎使用的统一驱动层类型
//
// 本包为 STRM 同步引擎提供不同数据源类型（CloudDrive2、OpenList、本地文件系统）
// 的清晰抽象。
package syncengine

import (
	"fmt"
	"net/url"
	"time"
)

// DriverType 标识数据源驱动实现的类型
//
// 每个驱动类型对应一个特定的数据源提供商。
// 可以通过实现 Driver 接口来添加新的驱动类型。
type DriverType string

const (
	// DriverCloudDrive2 标识 CloudDrive2 云存储驱动
	DriverCloudDrive2 DriverType = "clouddrive2"

	// DriverOpenList 标识 OpenList 云存储驱动
	DriverOpenList DriverType = "openlist"

	// DriverLocal 标识本地文件系统驱动
	DriverLocal DriverType = "local"
)

// String 返回 DriverType 的字符串表示
func (t DriverType) String() string {
	return string(t)
}

// IsValid 报告 DriverType 是否是已知的、受支持的值
func (t DriverType) IsValid() bool {
	switch t {
	case DriverCloudDrive2, DriverOpenList, DriverLocal:
		return true
	default:
		return false
	}
}

// DriverCapability 声明驱动支持的可选功能
//
// 这允许同步引擎根据每个数据源可用的功能来调整其行为。
type DriverCapability struct {
	// Watch 表示驱动是否支持实时文件变更监控
	Watch bool

	// StrmHTTP 表示驱动是否能生成基于 HTTP 的 STRM URL
	StrmHTTP bool

	// StrmMount 表示驱动是否能生成本地挂载路径的 STRM
	StrmMount bool

	// PickCode 表示驱动是否为文件提供 115 PickCode
	// 这是 115 云存储服务特有的功能
	PickCode bool

	// SignURL 表示驱动是否生成带过期时间的签名 URL
	// 签名 URL 通过限制访问时间来增强安全性
	SignURL bool
}

// RemoteEntry 描述远程文件或目录
//
// 这是跨所有驱动使用的统一文件信息结构，
// 规范化了各种数据源 API 之间的差异。
type RemoteEntry struct {
	// Path 是 Unix 格式的完整远程路径 (/path/to/file)
	Path string

	// Name 是文件的基本名称（例如 "file.txt"）
	Name string

	// Size 是文件大小（字节），目录为 0
	Size int64

	// ModTime 是文件的最后修改时间
	ModTime time.Time

	// IsDir 表示此条目是否为目录
	IsDir bool
}

// DriverEventType 枚举文件变更事件类型
//
// 这些事件由支持 Watch 功能的驱动发出。
type DriverEventType int

const (
	// DriverEventCreate 表示文件被创建
	DriverEventCreate DriverEventType = iota + 1

	// DriverEventUpdate 表示文件被修改
	DriverEventUpdate

	// DriverEventDelete 表示文件被删除
	DriverEventDelete
)

// String 返回 DriverEventType 的人类友好名称
func (t DriverEventType) String() string {
	switch t {
	case DriverEventCreate:
		return "create"
	case DriverEventUpdate:
		return "update"
	case DriverEventDelete:
		return "delete"
	default:
		return fmt.Sprintf("DriverEventType(%d)", t)
	}
}

// DriverEvent 表示驱动发出的文件变更事件
//
// 事件由支持 Watch 功能的驱动通过 channel 发送。
type DriverEvent struct {
	// Type 是事件类型（创建、更新或删除）
	Type DriverEventType

	// Path 是相对于源根目录的路径（如果已知）
	Path string

	// AbsPath 是绝对路径或远程路径
	AbsPath string

	// Size 是文件大小（字节）
	Size int64

	// ModTime 是最后修改时间
	ModTime time.Time

	// IsDir 表示事件是否针对目录
	IsDir bool
}

// EngineEvent 表示一个文件变更事件（用于增量同步）
//
// EngineEvent 是 engine 内部使用的事件抽象，用于将外部事件（如 DriverEvent、
// ports.SyncPlanItem 等）转换为增量处理的统一输入。
type EngineEvent struct {
	// Type 是事件类型（创建、更新或删除）
	Type DriverEventType

	// AbsPath 是完整的远程路径（如 /Movies/movie.mkv）
	// 这是主要路径字段，用于构建 StrmInfo 和计算输出路径
	AbsPath string

	// RelPath 是相对于 sourceRoot 的路径（可选，仅用于日志）
	RelPath string

	// Size 是文件大小（字节）
	Size int64

	// ModTime 是最后修改时间
	ModTime time.Time

	// IsDir 表示事件是否针对目录（目录事件会被跳过）
	IsDir bool
}

// ListOptions 配置 Driver.List 方法的列表行为
type ListOptions struct {
	// Recursive 表示是否递归列出文件
	Recursive bool

	// MaxDepth 限制递归深度（0 = 非递归，>0 = 最大深度）
	// 深度为 1 表示只列出直接子项
	MaxDepth int
}

// WatchOptions 配置 Driver.Watch 方法的监控行为
type WatchOptions struct {
	// Recursive 表示是否监控子目录
	Recursive bool
}

// StrmInfo 是驱动生成的结构化 STRM 元数据
//
// 此结构包含以下所需的所有信息：
// 1. 写入 STRM 文件 (RawURL)
// 2. 与现有 STRM 内容进行比较 (BaseURL, Path, Sign 等)
// 3. 验证 STRM 完整性和过期时间
type StrmInfo struct {
	// RawURL 是要写入 .strm 文件的完整内容
	// 这通常是 Emby/Jellyfin 等媒体服务器可以流式传输的 URL
	RawURL string

	// BaseURL 是解析后的基础 URL (scheme://host[:port])
	// 用于比较以检测服务器是否更改
	// 对于本地文件系统驱动可能为 nil
	BaseURL *url.URL

	// Path 是 URL 的清理后的远程路径组件
	// 已规范化以进行一致的比较
	Path string

	// PickCode 是文件的可选 115 PickCode
	// 仅当 DriverCapability.PickCode 为 true 时存在
	PickCode string

	// Sign 是用于认证访问的可选签名/令牌
	// 仅当 DriverCapability.SignURL 为 true 时存在
	Sign string

	// ExpiresAt 是可选的签名过期时间
	// 当存在且已过去时，STRM 内容需要更新
	ExpiresAt time.Time
}

// BuildStrmRequest 描述构建 STRM 内容的输入
//
// 这被传递给 Driver.BuildStrmInfo 以生成结构化的 STRM 元数据。
type BuildStrmRequest struct {
	// ServerID 是数据服务器 ID（用于日志或多租户场景）
	ServerID uint

	// RemotePath 是完整的远程文件路径
	RemotePath string

	// RemoteMeta 是可选的文件元数据，以避免重新获取
	// 如果提供，驱动可以跳过额外的 Stat 调用
	RemoteMeta *RemoteEntry
}

// CompareInput 提供期望的 STRM 信息和现有内容以进行比较
//
// 这被传递给 Driver.CompareStrm 以确定是否需要更新。
type CompareInput struct {
	// Expected 是应该写入的 STRM 信息
	Expected StrmInfo

	// ActualRaw 是从现有 .strm 文件读取的原始内容
	// 这将被解析并与 Expected 进行比较
	ActualRaw string
}

// CompareResult 指示 STRM 内容是否需要更新
//
// 这允许同步引擎在内容相同时跳过不必要的写入。
type CompareResult struct {
	// Equal 表示内容相同（不需要更新）
	Equal bool

	// NeedUpdate 表示内容不同，应该重写
	NeedUpdate bool

	// Reason 提供比较结果的人类可读解释
	// 这对于日志和调试很有用
	Reason string
}

// ChangeReason 表示需要更新或跳过文件的原因
//
// 这用于提供更细粒度的统计信息和日志记录，
// 帮助理解同步引擎的决策过程。
type ChangeReason int

const (
	// ChangeReasonUnknown 表示未知或未分类的原因
	ChangeReasonUnknown ChangeReason = iota

	// ChangeReasonNew 表示本地文件不存在，需要创建
	ChangeReasonNew

	// ChangeReasonForced 表示强制更新模式（忽略内容比对）
	ChangeReasonForced

	// ChangeReasonContent 表示内容发生变化，需要更新
	ChangeReasonContent

	// ChangeReasonModTime 表示仅修改时间变化（内容相同）
	// 这种情况下需要更新 STRM 文件的时间戳
	ChangeReasonModTime

	// ChangeReasonUnchanged 表示内容和修改时间均未变化
	// 这是最常见的跳过原因
	ChangeReasonUnchanged

	// ChangeReasonSkipExisting 表示因 SkipExisting 选项而跳过
	ChangeReasonSkipExisting
)

// String 返回 ChangeReason 的人类可读表示
func (r ChangeReason) String() string {
	switch r {
	case ChangeReasonNew:
		return "new"
	case ChangeReasonForced:
		return "forced"
	case ChangeReasonContent:
		return "content"
	case ChangeReasonModTime:
		return "modtime"
	case ChangeReasonUnchanged:
		return "unchanged"
	case ChangeReasonSkipExisting:
		return "skip_existing"
	default:
		return fmt.Sprintf("ChangeReason(%d)", r)
	}
}

// ChangeDecision 表示更新决策结果
//
// 这封装了"是否需要更新"的决策以及相关原因，
// 使决策逻辑更加清晰和可测试。
type ChangeDecision struct {
	// ShouldUpdate 表示是否需要更新文件
	ShouldUpdate bool

	// Reason 表示决策原因
	Reason ChangeReason
}

// StrmReplaceRule STRM替换规则
type StrmReplaceRule struct {
	From string
	To   string
}

// EngineOptions 引擎配置选项
//
// 这些选项控制同步引擎的行为，包括并发控制、
// 文件过滤、更新策略和孤儿清理等。
type EngineOptions struct {
	// MaxConcurrency 最大并发数（默认：10）
	// 控制同时处理的文件数量，避免资源耗尽
	MaxConcurrency int

	// OutputRoot STRM 文件输出根目录（必填）
	// 例如：/mnt/strm 或 D:\strm
	OutputRoot string

	// FileExtensions 要同步的文件扩展名列表（默认：所有文件）
	// 例如：[]string{".mp4", ".mkv", ".avi"}
	FileExtensions []string

	// MinFileSize 最小文件大小（字节，小于该值的文件将被过滤）
	// 0 表示不限制
	MinFileSize int64

	// DryRun 是否为试运行模式（默认：false）
	// 试运行模式下不会实际写入文件，只输出统计信息
	DryRun bool

	// ForceUpdate 是否强制更新所有 STRM 文件（默认：false）
	// 启用时会忽略 CompareStrm 结果，强制重写
	ForceUpdate bool

	// SkipExisting 是否跳过已存在的 STRM 文件（默认：false）
	// 启用时如果文件已存在则跳过，不进行比对
	SkipExisting bool

	// ModTimeEpsilon 修改时间允许的误差阈值（默认：2 秒）
	// 由于不同文件系统的时间精度不同，小于阈值的时间差异将被忽略
	// 这避免了因时间漂移导致的无意义更新
	ModTimeEpsilon time.Duration

	// EnableOrphanCleanup 是否启用孤儿文件清理（默认：false）
	// 启用后会删除远程文件已不存在的本地 STRM 文件
	EnableOrphanCleanup bool

	// OrphanCleanupDryRun 是否以试运行模式执行孤儿清理（默认：false）
	// 启用后只记录将要删除的文件，不实际删除
	OrphanCleanupDryRun bool

	// StrmReplaceRules STRM 替换规则（按顺序执行）
	StrmReplaceRules []StrmReplaceRule

	// ExcludeDirs 排除目录（相对远端根路径）
	ExcludeDirs []string
}

// SyncStats 同步统计信息
//
// 这提供了同步过程的详细统计数据，
// 用于监控、日志记录和性能分析。
//
// 注意：在 DryRun 模式下，CreatedFiles、UpdatedFiles、DeletedOrphans
// 等统计字段表示"将要执行"的操作数量，而不是"实际执行"的操作数量。
type SyncStats struct {
	// 扫描统计
	TotalFiles    int64 // 扫描到的总文件数
	TotalDirs     int64 // 扫描到的总目录数
	FilteredFiles int64 // 被过滤的文件数（不符合扩展名）

	// 处理统计
	ProcessedFiles   int64 // 已处理的文件数
	CreatedFiles     int64 // 新创建的 STRM 文件数
	UpdatedFiles     int64 // 更新的 STRM 文件数
	UpdatedByModTime int64 // 仅因修改时间变化而更新的文件数
	SkippedFiles     int64 // 跳过的文件数（总计）
	SkippedUnchanged int64 // 因内容和时间均未变化而跳过的文件数
	FailedFiles      int64 // 处理失败的文件数
	DeletedOrphans   int64 // 删除的孤儿文件数

	// 时间统计
	StartTime time.Time     // 开始时间
	EndTime   time.Time     // 结束时间
	Duration  time.Duration // 总耗时

	// 错误收集
	Errors []SyncError // 错误列表（最多保留前100个）
}

// SyncError 同步错误信息
//
// 记录同步过程中发生的错误，
// 包括文件路径、错误信息和时间戳。
type SyncError struct {
	FilePath string    // 文件路径
	Error    string    // 错误信息
	Time     time.Time // 发生时间
}
