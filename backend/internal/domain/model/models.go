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
	ID        uint      `gorm:"primaryKey" json:"id"`
	UID       string    `gorm:"size:64;uniqueIndex" json:"uid"`                    // 唯一标识（基于连接信息生成）
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`                  // 服务器名称
	Type      string    `gorm:"index;not null" json:"type"`                        // 类型: clouddrive2/openlist
	Host      string    `gorm:"not null" json:"host"`                              // 主机地址
	Port      int       `gorm:"not null" json:"port"`                              // 端口
	APIKey    string    `gorm:"type:text" json:"api_key"`                          // API密钥(可选)
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`              // 是否启用
	Options   string    `gorm:"type:text" json:"options"`                          // JSON扩展字段
	// QoS配置（独立列，不参与UID计算，允许覆盖全局默认值）
	RequestTimeoutMs int `gorm:"not null;default:30000" json:"request_timeout_ms"` // 请求超时(毫秒)
	ConnectTimeoutMs int `gorm:"not null;default:10000" json:"connect_timeout_ms"` // 连接超时(毫秒)
	RetryMax         int `gorm:"not null;default:3" json:"retry_max"`              // 重试次数
	RetryBackoffMs   int `gorm:"not null;default:1000" json:"retry_backoff_ms"`    // 退避时间(毫秒)
	MaxConcurrent    int `gorm:"not null;default:10" json:"max_concurrent"`        // 最大并发
	CreatedAt        time.Time `json:"created_at"`                                  // 创建时间
	UpdatedAt        time.Time `json:"updated_at"`                                  // 更新时间
}

// MediaServer 媒体服务器配置模型
// 用于配置Emby/Jellyfin/Plex等媒体库服务器
type MediaServer struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UID       string    `gorm:"size:64;uniqueIndex" json:"uid"`                    // 唯一标识（基于连接信息生成）
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`                  // 服务器名称
	Type      string    `gorm:"index;not null" json:"type"`                        // 类型: emby/jellyfin/plex
	Host      string    `gorm:"not null" json:"host"`                              // 主机地址
	Port      int       `gorm:"not null" json:"port"`                              // 端口
	APIKey    string    `gorm:"type:text" json:"api_key"`                          // API密钥
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`              // 是否启用
	Options   string    `gorm:"type:text" json:"options"`                          // JSON扩展字段
	// QoS配置（独立列，不参与UID计算，允许覆盖全局默认值）
	RequestTimeoutMs int `gorm:"not null;default:30000" json:"request_timeout_ms"` // 请求超时(毫秒)
	ConnectTimeoutMs int `gorm:"not null;default:10000" json:"connect_timeout_ms"` // 连接超时(毫秒)
	RetryMax         int `gorm:"not null;default:3" json:"retry_max"`              // 重试次数
	RetryBackoffMs   int `gorm:"not null;default:1000" json:"retry_backoff_ms"`    // 退避时间(毫秒)
	MaxConcurrent    int `gorm:"not null;default:10" json:"max_concurrent"`        // 最大并发
	CreatedAt        time.Time `json:"created_at"`                                  // 创建时间
	UpdatedAt        time.Time `json:"updated_at"`                                  // 更新时间
}

// Job 任务配置模型
// 用于配置STRM生成任务的所有参数
type Job struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Name          string     `gorm:"uniqueIndex;not null" json:"name"`                               // 任务名称（唯一）
	Enabled       bool       `gorm:"not null;default:true" json:"enabled"`                           // 是否启用
	Cron          string     `gorm:"type:text" json:"cron"`                                          // Cron表达式(可选,用于定时调度)
	WatchMode     string     `gorm:"not null;default:'local'" json:"watch_mode"`                     // 监控模式: local/api
	SourcePath    string     `gorm:"not null" json:"source_path"`                                    // 监控目录
	TargetPath    string     `gorm:"not null" json:"target_path"`                                    // 目的目录(STRM输出)
	STRMPath      string     `gorm:"not null" json:"strm_path"`                                      // STRM文件内容路径
	DataServerID  *uint      `gorm:"index" json:"data_server_id"`                                    // 数据服务器ID(可空)
	MediaServerID *uint      `gorm:"index" json:"media_server_id"`                                   // 媒体服务器ID(可空)
	Options       string     `gorm:"type:text" json:"options"`                                       // JSON扩展选项
	Status        string     `gorm:"default:'idle'" json:"status"`                                   // 状态: idle/running/error
	LastRunAt     *time.Time `json:"last_run_at"`                                                    // 最后执行时间
	ErrorMessage  string     `gorm:"type:text" json:"error_message"`                                 // 错误信息
	CreatedAt     time.Time  `gorm:"index:idx_jobs_created_at,sort:desc" json:"created_at"`          // 创建时间
	UpdatedAt     time.Time  `json:"updated_at"`                                                     // 更新时间

	// 关联关系
	DataServer  *DataServer  `gorm:"foreignKey:DataServerID" json:"data_server,omitempty"`           // 关联的数据服务器
	MediaServer *MediaServer `gorm:"foreignKey:MediaServerID" json:"media_server,omitempty"`         // 关联的媒体服务器
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
	ID             uint       `gorm:"primaryKey" json:"id"`
	JobID          uint       `gorm:"index;not null" json:"job_id"`                       // 关联任务ID
	Status         string     `gorm:"not null;index:idx_task_runs_status_priority_available,priority:1" json:"status"` // 执行状态: pending/running/completed/failed/cancelled
	Priority       int        `gorm:"not null;default:2;index:idx_task_runs_status_priority_available,priority:2" json:"priority"` // 优先级: 1=High,2=Normal,3=Low
	AvailableAt    time.Time  `gorm:"index:idx_task_runs_status_priority_available,priority:3" json:"available_at"` // 可执行时间
	Attempts       int        `gorm:"not null;default:0" json:"attempts"`                  // 重试次数
	MaxAttempts    int        `gorm:"not null;default:3" json:"max_attempts"`              // 最大重试次数
	DedupKey       string     `gorm:"uniqueIndex;not null" json:"dedup_key"`               // 去重键
	WorkerID       string     `gorm:"index" json:"worker_id"`                              // 执行的Worker ID
	FailureKind    string     `gorm:"index" json:"failure_kind"`                           // 失败类型: retryable/permanent/cancelled
	StartedAt      time.Time  `gorm:"index" json:"started_at"`                            // 开始时间
	EndedAt        *time.Time `json:"ended_at"`                                           // 结束时间
	Duration       int64      `gorm:"default:0" json:"duration"`                          // 执行时长(秒)
	Progress       int        `gorm:"default:0" json:"progress"`                          // 进度(0-100)
	TotalFiles     int        `gorm:"default:0" json:"total_files"`                       // 总文件数
	ProcessedFiles int        `gorm:"default:0" json:"processed_files"`                   // 已处理文件数
	FailedFiles    int        `gorm:"default:0" json:"failed_files"`                      // 失败文件数
	ErrorMessage   string     `gorm:"type:text" json:"error_message"`                     // 错误信息
	Payload        string     `gorm:"type:text" json:"payload"`                           // JSON执行参数

	// 关联关系
	Job *Job `gorm:"foreignKey:JobID" json:"job,omitempty"`                              // 关联的任务
}

// LogEntry 日志记录模型
type LogEntry struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Level      string     `gorm:"index:idx_logs_level_created_at,priority:1;not null" json:"level"`          // 日志级别
	Module     *string    `gorm:"index" json:"module,omitempty"`                                             // 模块名称
	Message    string     `gorm:"type:text;not null" json:"message"`                                         // 日志消息
	RequestID  *string    `gorm:"index" json:"request_id,omitempty"`                                         // 请求ID
	UserAction *string    `gorm:"index" json:"user_action,omitempty"`                                        // 用户操作
	JobID      *uint      `gorm:"index" json:"job_id,omitempty"`                                             // 关联的任务ID
	CreatedAt  time.Time  `gorm:"index:idx_logs_level_created_at,priority:2;index" json:"created_at"`        // 创建时间
}

// Setting 系统设置模型
type Setting struct {
	Key       string    `gorm:"primaryKey" json:"key"`       // 设置键
	Value     string    `gorm:"type:text;not null" json:"value"` // 设置值(JSON)
	UpdatedAt time.Time `json:"updated_at"`                  // 更新时间
}

// QoSSettings QoS配置结构（用于Settings.Value的JSON解析）
// 约定：settings 表中 key="global_qos" 的记录，其 value 应符合此结构
type QoSSettings struct {
	RequestTimeoutMs int `json:"request_timeout_ms"` // 请求超时(毫秒)
	ConnectTimeoutMs int `json:"connect_timeout_ms"` // 连接超时(毫秒)
	RetryMax         int `json:"retry_max"`          // 重试次数
	RetryBackoffMs   int `json:"retry_backoff_ms"`   // 退避时间(毫秒)
	MaxConcurrent    int `json:"max_concurrent"`     // 最大并发
}

// TableName 指定表名
func (DataServer) TableName() string  { return "data_servers" }
func (MediaServer) TableName() string { return "media_servers" }
func (Job) TableName() string         { return "jobs" }
func (TaskRun) TableName() string     { return "task_runs" }
func (LogEntry) TableName() string    { return "logs" }
func (Setting) TableName() string     { return "settings" }

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
