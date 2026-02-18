// Package ports 定义应用层的端口类型
package ports

import "time"

// JobID 任务ID
type JobID = uint

// TaskRunID 任务执行记录ID
type TaskRunID = uint

// DataServerID 数据服务器ID
type DataServerID = uint

// MediaServerID 媒体服务器ID
type MediaServerID = uint

// WatchMode 监控模式
type WatchMode string

// FileListRequest 文件列表请求
type FileListRequest struct {
	ServerID  DataServerID // 数据服务器ID
	Path      string       // 路径
	Recursive bool         // 是否递归
	MaxDepth  *int         // 递归最大深度（nil 表示使用默认值，0 表示非递归）
}

const (
	WatchModeAPI   WatchMode = "api"   // API模式（通过CloudDrive2等API监控）
	WatchModeLocal WatchMode = "local" // 本地模式（直接监控本地挂载路径）
)

// String 实现Stringer接口
func (m WatchMode) String() string {
	return string(m)
}

// IsValid 验证WatchMode是否有效
func (m WatchMode) IsValid() bool {
	switch m {
	case WatchModeAPI, WatchModeLocal:
		return true
	default:
		return false
	}
}

// FileEventType 文件事件类型
type FileEventType int

const (
	FileEventCreate FileEventType = iota + 1 // 文件创建
	FileEventUpdate                          // 文件更新
	FileEventDelete                          // 文件删除
)

// String 实现Stringer接口
func (t FileEventType) String() string {
	switch t {
	case FileEventCreate:
		return "create"
	case FileEventUpdate:
		return "update"
	case FileEventDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// FileEvent 文件变更事件
type FileEvent struct {
	Type    FileEventType // 事件类型
	Path    string        // 文件路径（相对于source_path）
	AbsPath string        // 绝对路径
	ModTime time.Time     // 修改时间
	Size    int64         // 文件大小
	IsDir   bool          // 是否为目录
}

// SyncOperation 同步操作类型
type SyncOperation int

const (
	SyncOpCreate SyncOperation = iota + 1 // 创建strm文件
	SyncOpUpdate                           // 更新strm文件
	SyncOpDelete                           // 删除strm文件
)

// String 实现Stringer接口
func (o SyncOperation) String() string {
	switch o {
	case SyncOpCreate:
		return "create"
	case SyncOpUpdate:
		return "update"
	case SyncOpDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// SyncPlanItem 同步计划项
type SyncPlanItem struct {
	Op             SyncOperation // 操作类型
	SourcePath     string        // 源文件路径（CloudDrive2虚拟路径）
	TargetStrmPath string        // 目标strm文件路径（本地文件系统）
	StreamURL      string        // 流媒体URL（写入strm文件的内容）
	Size           int64         // 源文件大小
	ModTime        time.Time     // 源文件修改时间
}

// TaskRunSummary 任务执行摘要
type TaskRunSummary struct {
	CreatedCount int       // 创建的strm文件数量
	UpdatedCount int       // 更新的strm文件数量
	DeletedCount int       // 删除的strm文件数量
	FailedCount  int       // 失败的操作数量
	Duration     int64     // 执行时长（秒）
	StartedAt    time.Time // 开始时间
	EndedAt      time.Time // 结束时间
	ErrorMessage string    // 错误信息（如果失败）
}

// RemoteFile 远程文件信息（从DataServer获取）
type RemoteFile struct {
	Path    string    // 文件路径
	Name    string    // 文件名
	Size    int64     // 文件大小
	ModTime time.Time // 修改时间
	IsDir   bool      // 是否为目录
}

// JobConfig Job配置信息（从database.Job解析）
type JobConfig struct {
	ID              JobID         // Job ID
	Name            string        // 任务名称
	WatchMode       WatchMode     // 监控模式
	DataServerID    DataServerID  // 数据服务器ID
	MediaServerID   MediaServerID // 媒体服务器ID（可选）
	SourcePath      string        // 源路径
	TargetPath      string        // 目标路径
	Recursive       bool          // 是否递归
	Extensions      []string      // 允许的扩展名（如[".mkv", ".mp4"]）
	Interval        int           // 扫描间隔（秒，仅api模式）
	Enabled         bool          // 是否启用
	AutoScanLibrary bool          // 完成后是否自动扫描媒体库
}

// ExecutionContext 执行上下文
type ExecutionContext struct {
	JobID      JobID      // Job ID
	TaskRunID  TaskRunID  // TaskRun ID
	JobConfig  *JobConfig // Job配置
	CancelFunc func()     // 取消函数
}
