package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JobOptions 表示 Job.Options 中存储的高级配置项
type JobOptions struct {
	MaxConcurrency      int               `json:"max_concurrency"`
	Recursive           *bool             `json:"recursive"`
	Interval            *int              `json:"interval"`
	AutoScanLibrary     *bool             `json:"auto_scan_library"`
	MinFileSize         int64             `json:"min_file_size"`
	MetadataMode        string            `json:"metadata_mode"`
	STRMMode            string            `json:"strm_mode"`
	ForceUpdate         bool              `json:"force_update"`
	SyncOpts            SyncOpts          `json:"sync_opts"`
	MediaExts           []string          `json:"media_exts"`
	MetaExts            []string          `json:"meta_exts"`
	ExcludeDirs         []string          `json:"exclude_dirs"`
	StrmReplaceRules    []StrmReplaceRule `json:"strm_replace_rules"`
	PreferRemoteList    bool              `json:"prefer_remote_list"`
	DryRun              bool              `json:"dry_run"`
	SkipExisting        bool              `json:"skip_existing"`
	EnableOrphanCleanup bool              `json:"enable_orphan_cleanup"`
	OrphanCleanupDryRun bool              `json:"orphan_cleanup_dry_run"`
}

// SyncOpts 表示 JobOptions 中的 sync_opts
type SyncOpts struct {
	FullResync    bool `json:"full_resync"`
	UpdateMeta    bool `json:"update_meta"`
	OverwriteMeta bool `json:"overwrite_meta"`
	SkipMeta      bool `json:"skip_meta"`
}

// StrmReplaceRule 表示 STRM 路径替换规则
type StrmReplaceRule struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// DataServerOptions 表示 DataServer.Options 中存储的高级配置项
type DataServerOptions struct {
	BaseURL        string `json:"base_url"`
	AccessPath     string `json:"access_path"`
	STRMMode       string `json:"strm_mode"`
	MountPath      string `json:"mount_path"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Username       string `json:"username"`
	Password       string `json:"password"`
}

// MediaServerOptions 表示 MediaServer.Options 中存储的高级配置项
// 目前后端暂未大量解析，作为占位扩展
type MediaServerOptions struct {
}

func (o JobOptions) Value() (driver.Value, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (o *JobOptions) Scan(value any) error {
	if value == nil {
		*o = JobOptions{}
		return nil
	}
	bytes, err := asBytes(value)
	if err != nil {
		return err
	}
	if len(bytes) == 0 {
		*o = JobOptions{}
		return nil
	}
	if err := json.Unmarshal(bytes, o); err != nil {
		return fmt.Errorf("parse job options: %w", err)
	}
	return nil
}

func (o DataServerOptions) Value() (driver.Value, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (o *DataServerOptions) Scan(value any) error {
	if value == nil {
		*o = DataServerOptions{}
		return nil
	}
	bytes, err := asBytes(value)
	if err != nil {
		return err
	}
	if len(bytes) == 0 {
		*o = DataServerOptions{}
		return nil
	}
	if err := json.Unmarshal(bytes, o); err != nil {
		return fmt.Errorf("parse data server options: %w", err)
	}
	return nil
}

func (o MediaServerOptions) Value() (driver.Value, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (o *MediaServerOptions) Scan(value any) error {
	if value == nil {
		*o = MediaServerOptions{}
		return nil
	}
	bytes, err := asBytes(value)
	if err != nil {
		return err
	}
	if len(bytes) == 0 {
		*o = MediaServerOptions{}
		return nil
	}
	if err := json.Unmarshal(bytes, o); err != nil {
		return fmt.Errorf("parse media server options: %w", err)
	}
	return nil
}

func asBytes(value any) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unsupported options type: %T", value)
	}
}
