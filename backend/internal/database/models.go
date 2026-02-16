// Package database contains GORM models for STRMSync.
// Models are defined according to docs/IMPLEMENTATION_PLAN.md section 2.2.
package database

import "time"

// Source 数据源模型
type Source struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"uniqueIndex;not null" json:"name"`
	Type         string     `gorm:"index;not null" json:"type"` // local/clouddrive2/openlist
	Enabled      bool       `gorm:"not null;default:true" json:"enabled"`
	Config       string     `gorm:"type:text;not null" json:"config"` // JSON
	SourcePrefix string     `gorm:"not null" json:"source_prefix"`
	TargetPrefix string     `gorm:"not null" json:"target_prefix"`
	Options      string     `gorm:"type:text" json:"options"` // JSON
	Status       string     `gorm:"default:'idle'" json:"status"`
	LastScanAt   *time.Time `json:"last_scan_at"`
	FileCount    int        `gorm:"default:0" json:"file_count"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// 关联
	Files []File `gorm:"foreignKey:SourceID;constraint:OnDelete:CASCADE" json:"files,omitempty"`
	Tasks []Task `gorm:"foreignKey:SourceID;constraint:OnDelete:SET NULL" json:"tasks,omitempty"`
}

// File 文件索引模型
type File struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	SourceID       uint      `gorm:"index;not null" json:"source_id"`
	SourcePath     string    `gorm:"index;not null" json:"source_path"`
	TargetPath     string    `gorm:"not null" json:"target_path"`
	STRMContent    string    `gorm:"type:text;not null" json:"strm_content"`
	FileName       string    `gorm:"not null" json:"file_name"`
	FileSize       int64     `gorm:"not null" json:"file_size"`
	FileHash       string    `gorm:"index;not null" json:"file_hash"`
	ModifiedAt     time.Time `gorm:"not null" json:"modified_at"`
	IsDir          bool      `gorm:"not null;default:false" json:"is_dir"`
	STRMGenerated  bool      `gorm:"default:false" json:"strm_generated"`
	MetadataSynced bool      `gorm:"default:false" json:"metadata_synced"`
	Notified       bool      `gorm:"default:false" json:"notified"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// 关联
	Source        Source         `gorm:"foreignKey:SourceID" json:"source,omitempty"`
	MetadataFiles []MetadataFile `gorm:"foreignKey:FileID;constraint:OnDelete:CASCADE" json:"metadata_files,omitempty"`
}

// MetadataFile 元数据文件模型
type MetadataFile struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FileID     uint      `gorm:"index;not null" json:"file_id"`
	SourcePath string    `gorm:"not null" json:"source_path"`
	TargetPath string    `gorm:"not null" json:"target_path"`
	FileType   string    `gorm:"not null" json:"file_type"` // nfo/poster/fanart/subtitle
	Synced     bool      `gorm:"index;default:false" json:"synced"`
	CreatedAt  time.Time `json:"created_at"`

	// 关联
	File File `gorm:"foreignKey:FileID" json:"file,omitempty"`
}

// Task 任务模型
type Task struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"not null" json:"name"`
	Type           string     `gorm:"not null" json:"type"` // scan/watch/sync/notify
	SourceID       *uint      `gorm:"index" json:"source_id"`
	Status         string     `gorm:"index;not null;default:'pending'" json:"status"`
	Progress       int        `gorm:"default:0" json:"progress"`
	TotalFiles     int        `gorm:"default:0" json:"total_files"`
	ProcessedFiles int        `gorm:"default:0" json:"processed_files"`
	FailedFiles    int        `gorm:"default:0" json:"failed_files"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	CreatedAt      time.Time  `gorm:"index:idx_tasks_created_at,sort:desc" json:"created_at"`

	// 关联
	Source *Source `gorm:"foreignKey:SourceID" json:"source,omitempty"`
}

// Setting 系统设置模型
type Setting struct {
	Key       string    `gorm:"primaryKey" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"` // JSON
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 方法用于指定表名
func (Source) TableName() string       { return "sources" }
func (File) TableName() string         { return "files" }
func (MetadataFile) TableName() string { return "metadata_files" }
func (Task) TableName() string         { return "tasks" }
func (Setting) TableName() string      { return "settings" }
