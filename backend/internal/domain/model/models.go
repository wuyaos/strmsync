// Package database 包含STRMSync的GORM模型定义
// 新架构: 配置管理(Servers) + 任务配置(Jobs) + 执行记录(TaskRuns)
package model

import (
	"time"

	"gorm.io/gorm"
)

// DataServer 数据服务器配置模型
// 用于配置CloudDrive2/OpenList等数据源服务器
type DataServer struct {
	ID      uint              `gorm:"primaryKey" json:"id"`
	UID     string            `gorm:"size:64;uniqueIndex" json:"uid"`       // 唯一标识（基于连接信息生成）
	Name    string            `gorm:"uniqueIndex;not null" json:"name"`     // 服务器名称
	Type    string            `gorm:"index;not null" json:"type"`           // 类型: clouddrive2/openlist
	Host    string            `gorm:"not null" json:"host"`                 // 主机地址
	Port    int               `gorm:"not null" json:"port"`                 // 端口
	APIKey  string            `gorm:"type:text" json:"api_key"`             // API密钥(可选)
	Enabled bool              `gorm:"not null;default:true" json:"enabled"` // 是否启用
	Options DataServerOptions `gorm:"type:text" json:"options"`             // 结构化配置
	// 高级配置（独立列，不参与UID计算，允许覆盖全局默认值）
	DownloadRatePerSec  int       `gorm:"not null;default:0" json:"download_rate_per_sec"`  // 下载队列每秒处理数量（0=使用全局）
	APIRate             int       `gorm:"not null;default:0" json:"api_rate"`               // 接口速率（每秒请求数，0=使用全局）
	APIConcurrency      int       `gorm:"not null;default:0" json:"api_concurrency"`        // 接口并发上限（0=使用全局）
	APIRetryMax         int       `gorm:"not null;default:0" json:"api_retry_max"`          // 接口重试次数（0=使用全局）
	APIRetryIntervalSec int       `gorm:"not null;default:0" json:"api_retry_interval_sec"` // 接口重试间隔（秒，0=使用全局）
	CreatedAt           time.Time `json:"created_at"`                                       // 创建时间
	UpdatedAt           time.Time `json:"updated_at"`                                       // 更新时间
}

// MediaServer 媒体服务器配置模型
// 用于配置Emby/Jellyfin/Plex等媒体库服务器
type MediaServer struct {
	ID      uint               `gorm:"primaryKey" json:"id"`
	UID     string             `gorm:"size:64;uniqueIndex" json:"uid"`       // 唯一标识（基于连接信息生成）
	Name    string             `gorm:"uniqueIndex;not null" json:"name"`     // 服务器名称
	Type    string             `gorm:"index;not null" json:"type"`           // 类型: emby/jellyfin/plex
	Host    string             `gorm:"not null" json:"host"`                 // 主机地址
	Port    int                `gorm:"not null" json:"port"`                 // 端口
	APIKey  string             `gorm:"type:text" json:"api_key"`             // API密钥
	Enabled bool               `gorm:"not null;default:true" json:"enabled"` // 是否启用
	Options MediaServerOptions `gorm:"type:text" json:"options"`             // 结构化配置
	// 高级配置（独立列，不参与UID计算，允许覆盖全局默认值）
	DownloadRatePerSec  int       `gorm:"not null;default:0" json:"download_rate_per_sec"`  // 下载队列每秒处理数量（0=使用全局）
	APIRate             int       `gorm:"not null;default:0" json:"api_rate"`               // 接口速率（每秒请求数，0=使用全局）
	APIConcurrency      int       `gorm:"not null;default:0" json:"api_concurrency"`        // 接口并发上限（0=使用全局）
	APIRetryMax         int       `gorm:"not null;default:0" json:"api_retry_max"`          // 接口重试次数（0=使用全局）
	APIRetryIntervalSec int       `gorm:"not null;default:0" json:"api_retry_interval_sec"` // 接口重试间隔（秒，0=使用全局）
	CreatedAt           time.Time `json:"created_at"`                                       // 创建时间
	UpdatedAt           time.Time `json:"updated_at"`                                       // 更新时间
}

// Job 任务配置模型
// 用于配置STRM生成任务的所有参数
type Job struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Name          string     `gorm:"uniqueIndex;not null" json:"name"`                      // 任务名称（唯一）
	Enabled       bool       `gorm:"not null;default:true" json:"enabled"`                  // 是否启用
	Cron          string     `gorm:"type:text" json:"cron"`                                 // Cron表达式(可选,用于定时调度)
	WatchMode     string     `gorm:"not null;default:'local'" json:"watch_mode"`            // 监控模式: local/api
	SourcePath    string     `gorm:"not null" json:"source_path"`                           // 监控目录
	RemoteRoot    string     `gorm:"type:text" json:"remote_root"`                          // 远程根目录（CD2/OpenList，API起点）
	TargetPath    string     `gorm:"not null" json:"target_path"`                           // 目的目录(STRM输出)
	STRMPath      string     `gorm:"not null" json:"strm_path"`                             // STRM文件内容路径
	DataServerID  *uint      `gorm:"index" json:"data_server_id"`                           // 数据服务器ID(可空)
	MediaServerID *uint      `gorm:"index" json:"media_server_id"`                          // 媒体服务器ID(可空)
	Options       JobOptions `gorm:"type:text" json:"options"`                              // 结构化配置
	Status        string     `gorm:"default:'idle'" json:"status"`                          // 状态: idle/running/error
	LastRunAt     *time.Time `json:"last_run_at"`                                           // 最后执行时间
	ErrorMessage  string     `gorm:"type:text" json:"error_message"`                        // 错误信息
	CreatedAt     time.Time  `gorm:"index:idx_jobs_created_at,sort:desc" json:"created_at"` // 创建时间
	UpdatedAt     time.Time  `json:"updated_at"`                                            // 更新时间

	// 关联关系
	DataServer  *DataServer  `gorm:"foreignKey:DataServerID" json:"data_server,omitempty"`                    // 关联的数据服务器
	MediaServer *MediaServer `gorm:"foreignKey:MediaServerID" json:"media_server,omitempty"`                  // 关联的媒体服务器
	TaskRuns    []TaskRun    `gorm:"foreignKey:JobID;constraint:OnDelete:CASCADE" json:"task_runs,omitempty"` // 执行记录列表
}

// TaskRun 任务执行记录模型
// 记录每次任务执行的详细信息
//
// 队列功能扩展：
// - Priority: 任务优先级（1=High, 2=Normal, 3=Low）
// - AvailableAt: 可执行时间（支持延迟重试）
// - Attempts: 重试次数
// - MaxAttempts: 最大重试次数
// - DedupKey: 去重键（唯一索引，防止重复入队）
// - WorkerID: 执行的 Worker ID
// - FailureKind: 失败类型（retryable/permanent/cancelled）
type TaskRun struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	JobID              uint       `gorm:"index;not null" json:"job_id"`                                                                // 关联任务ID
	Status             string     `gorm:"not null;index:idx_task_runs_status_priority_available,priority:1" json:"status"`             // 执行状态: pending/running/completed/failed/cancelled
	Priority           int        `gorm:"not null;default:2;index:idx_task_runs_status_priority_available,priority:2" json:"priority"` // 优先级: 1=High,2=Normal,3=Low
	AvailableAt        time.Time  `gorm:"index:idx_task_runs_status_priority_available,priority:3" json:"available_at"`                // 可执行时间
	Attempts           int        `gorm:"not null;default:0" json:"attempts"`                                                          // 重试次数
	MaxAttempts        int        `gorm:"not null;default:3" json:"max_attempts"`                                                      // 最大重试次数
	DedupKey           string     `gorm:"uniqueIndex;not null" json:"dedup_key"`                                                       // 去重键
	WorkerID           string     `gorm:"index" json:"worker_id"`                                                                      // 执行的Worker ID
	FailureKind        string     `gorm:"index" json:"failure_kind"`                                                                   // 失败类型: retryable/permanent/cancelled
	StartedAt          time.Time  `gorm:"index" json:"started_at"`                                                                     // 开始时间
	EndedAt            *time.Time `json:"ended_at"`                                                                                    // 结束时间
	Duration           int64      `gorm:"default:0" json:"duration"`                                                                   // 执行时长(秒)
	Progress           int        `gorm:"default:0" json:"progress"`                                                                   // 进度(0-100)
	TotalFiles         int        `gorm:"default:0" json:"total_files"`                                                                // 总文件数
	ProcessedFiles     int        `gorm:"default:0" json:"processed_files"`                                                            // 已处理文件数
	FailedFiles        int        `gorm:"default:0" json:"failed_files"`                                                               // 失败文件数
	CreatedFiles       int        `gorm:"default:0" json:"created_files"`                                                              // 新建STRM数
	UpdatedFiles       int        `gorm:"default:0" json:"updated_files"`                                                              // 更新STRM数
	SkippedFiles       int        `gorm:"default:0" json:"skipped_files"`                                                              // 跳过STRM数
	FilteredFiles      int        `gorm:"default:0" json:"filtered_files"`                                                             // 过滤文件数
	MetaTotalFiles     int        `gorm:"default:0" json:"meta_total_files"`                                                           // 元数据总数
	MetaCreatedFiles   int        `gorm:"default:0" json:"meta_created_files"`                                                         // 元数据新建
	MetaUpdatedFiles   int        `gorm:"default:0" json:"meta_updated_files"`                                                         // 元数据更新
	MetaProcessedFiles int        `gorm:"default:0" json:"meta_processed_files"`                                                       // 元数据已处理
	MetaFailedFiles    int        `gorm:"default:0" json:"meta_failed_files"`                                                          // 元数据失败
	ErrorMessage       string     `gorm:"type:text" json:"error_message"`                                                              // 错误信息
	Payload            string     `gorm:"type:text" json:"payload"`                                                                    // JSON执行参数

	// 关联关系
	Job *Job `gorm:"foreignKey:JobID" json:"job,omitempty"` // 关联的任务
}

// TaskRunEvent 任务执行事件明细
// 记录执行过程中的单个文件操作
type TaskRunEvent struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TaskRunID    uint      `gorm:"index;not null" json:"task_run_id"`
	JobID        uint      `gorm:"index;not null" json:"job_id"`
	Kind         string    `gorm:"index;not null" json:"kind"`   // strm/meta
	Op           string    `gorm:"index;not null" json:"op"`     // create/update/delete/copy/skip
	Status       string    `gorm:"index;not null" json:"status"` // success/failed/skipped
	SourcePath   string    `gorm:"type:text" json:"source_path"`
	TargetPath   string    `gorm:"type:text" json:"target_path"`
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

// LogEntry 日志记录模型
type LogEntry struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Level      string    `gorm:"index:idx_logs_level_created_at,priority:1;not null" json:"level"`   // 日志级别
	Module     *string   `gorm:"index" json:"module,omitempty"`                                      // 模块名称
	Message    string    `gorm:"type:text;not null" json:"message"`                                  // 日志消息
	RequestID  *string   `gorm:"index" json:"request_id,omitempty"`                                  // 请求ID
	UserAction *string   `gorm:"index" json:"user_action,omitempty"`                                 // 用户操作
	JobID      *uint     `gorm:"index" json:"job_id,omitempty"`                                      // 关联的任务ID
	CreatedAt  time.Time `gorm:"index:idx_logs_level_created_at,priority:2;index" json:"created_at"` // 创建时间
}

// Setting 系统设置模型
type Setting struct {
	Key       string    `gorm:"primaryKey" json:"key"`           // 设置键
	Value     string    `gorm:"type:text;not null" json:"value"` // 设置值(JSON)
	UpdatedAt time.Time `json:"updated_at"`                      // 更新时间
}

// QoSSettings 高级配置结构（用于Settings.Value的JSON解析）
// 约定：settings 表中 key="app_settings" 的记录，其 value 应符合此结构的 rate 段
type QoSSettings struct {
	DownloadRatePerSec  int `json:"download_rate_per_sec"`  // 下载队列每秒处理数量
	APIRate             int `json:"api_rate"`               // 接口速率（每秒请求数）
	APIConcurrency      int `json:"api_concurrency"`        // 接口并发上限
	APIRetryMax         int `json:"api_retry_max"`          // 接口重试次数
	APIRetryIntervalSec int `json:"api_retry_interval_sec"` // 接口重试间隔（秒）
}

// TableName 指定表名
func (DataServer) TableName() string   { return "data_servers" }
func (MediaServer) TableName() string  { return "media_servers" }
func (Job) TableName() string          { return "jobs" }
func (TaskRun) TableName() string      { return "task_runs" }
func (TaskRunEvent) TableName() string { return "task_run_events" }
func (LogEntry) TableName() string     { return "logs" }
func (Setting) TableName() string      { return "settings" }

// BeforeCreate 在创建 DataServer 前生成 UID
func (s *DataServer) BeforeCreate(tx *gorm.DB) error {
	if s.UID == "" {
		uid, err := GenerateDataServerUID(s.Type, s.Host, s.Port, s.Options, s.APIKey)
		if err != nil {
			return err
		}
		s.UID = uid
	}
	return nil
}

// BeforeCreate 在创建 MediaServer 前生成 UID
func (s *MediaServer) BeforeCreate(tx *gorm.DB) error {
	if s.UID == "" {
		uid, err := GenerateMediaServerUID(s.Type, s.Host, s.Port, s.Options, s.APIKey)
		if err != nil {
			return err
		}
		s.UID = uid
	}
	return nil
}
