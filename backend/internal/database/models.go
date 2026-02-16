package database

import (
	"time"
)

// Source 数据源配置表
// Author: STRMSync Team
type Source struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Type      string    `gorm:"size:20;not null" json:"type"` // local, clouddrive2, openlist
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Config    string    `gorm:"type:text" json:"config"` // JSON 加密配置
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// File 媒体文件索引表
// Author: STRMSync Team
type File struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	SourceID       uint      `gorm:"not null;index" json:"source_id"`
	RelativePath   string    `gorm:"size:1000;not null;index" json:"relative_path"` // 相对于数据源的路径
	FileName       string    `gorm:"size:255;not null" json:"file_name"`
	FileSize       int64     `gorm:"not null" json:"file_size"`
	ModTime        time.Time `gorm:"not null;index" json:"mod_time"`
	FastHash       string    `gorm:"size:32;index" json:"fast_hash"` // MD5 快速哈希
	StrmPath       string    `gorm:"size:1000" json:"strm_path"`     // STRM 文件路径
	StrmGenerated  bool      `gorm:"default:false;index" json:"strm_generated"`
	LastScanTime   time.Time `gorm:"index" json:"last_scan_time"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// 关联
	Source Source `gorm:"foreignKey:SourceID" json:"-"`
}

// MetadataFile 元数据文件表
// Author: STRMSync Team
type MetadataFile struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	FileID       uint      `gorm:"not null;index" json:"file_id"`
	Type         string    `gorm:"size:20;not null" json:"type"` // nfo, poster, fanart, subtitle
	SourcePath   string    `gorm:"size:1000;not null" json:"source_path"`
	TargetPath   string    `gorm:"size:1000" json:"target_path"`
	Synced       bool      `gorm:"default:false;index" json:"synced"`
	SyncTime     time.Time `json:"sync_time"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 关联
	File File `gorm:"foreignKey:FileID" json:"-"`
}

// Task 任务表
// Author: STRMSync Team
type Task struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Type       string    `gorm:"size:50;not null" json:"type"` // scan, watch, sync, notify
	SourceID   uint      `gorm:"index" json:"source_id"`
	Status     string    `gorm:"size:20;not null;index" json:"status"` // pending, running, completed, failed
	Progress   int       `gorm:"default:0" json:"progress"`            // 0-100
	TotalItems int       `gorm:"default:0" json:"total_items"`
	Message    string    `gorm:"type:text" json:"message"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Setting 系统设置表
// Author: STRMSync Team
type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:100;not null;uniqueIndex" json:"key"`
	Value     string    `gorm:"type:text" json:"value"` // JSON 格式
	Category  string    `gorm:"size:50;not null;index" json:"category"` // general, scan, strm, notifier
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Source) TableName() string {
	return "sources"
}

func (File) TableName() string {
	return "files"
}

func (MetadataFile) TableName() string {
	return "metadata_files"
}

func (Task) TableName() string {
	return "tasks"
}

func (Setting) TableName() string {
	return "settings"
}
