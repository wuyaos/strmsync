// Package worker 提供 Worker 执行器实现
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	appports "github.com/strmsync/strmsync/internal/app/ports"
	appsync "github.com/strmsync/strmsync/internal/app/sync"
	appconfig "github.com/strmsync/strmsync/internal/config"
	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"github.com/strmsync/strmsync/internal/queue"
	"github.com/strmsync/strmsync/internal/strmwriter"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Executor 执行单个 TaskRun 的同步逻辑
//
// 设计要点：
// - 将 Job 配置转换为 EngineOptions
// - 构建 Driver 与 Writer 实例
// - 调用 syncengine.Engine.RunOnce
// - 更新 TaskRun 进度统计
//
// 错误处理：
// - 使用 syncqueue.TaskError 包装错误，区分可重试和永久失败
// - 配置错误、权限问题等为永久失败
// - 网络错误、临时 IO 错误为可重试失败
type Executor struct {
	cfg ExecutorConfig
	log *zap.Logger
}

// ExecutorConfig 描述 Executor 所需的依赖项
type ExecutorConfig struct {
	JobRepo       JobRepository
	DataServers   DataServerRepository
	TaskRuns      TaskRunRepository
	DriverFactory DriverFactory
	WriterFactory WriterFactory
	Logger        *zap.Logger
}

// NewExecutor 创建 Executor 实例
//
// 参数：
//   - cfg: Executor 配置（JobRepo、DataServers、TaskRuns 必填）
//
// 返回：
//   - *Executor: Executor 实例
//   - error: 配置无效时返回错误
func NewExecutor(cfg ExecutorConfig) (*Executor, error) {
	if cfg.JobRepo == nil {
		return nil, fmt.Errorf("worker: job repository is nil")
	}
	if cfg.DataServers == nil {
		return nil, fmt.Errorf("worker: data server repository is nil")
	}
	if cfg.TaskRuns == nil {
		return nil, fmt.Errorf("worker: task run repository is nil")
	}

	// 设置默认工厂
	if cfg.DriverFactory == nil {
		cfg.DriverFactory = DefaultDriverFactory{Logger: cfg.Logger}
	}
	if cfg.WriterFactory == nil {
		cfg.WriterFactory = DefaultWriterFactory{Logger: cfg.Logger}
	}

	// 设置日志器
	if cfg.Logger == nil {
		cfg.Logger = logger.With(zap.String("component", "worker-executor"))
	}

	return &Executor{
		cfg: cfg,
		log: cfg.Logger,
	}, nil
}

// Run 执行一个 TaskRun 并返回统计信息
//
// 执行流程：
// 1. 加载 Job 配置
// 2. 加载 DataServer 配置
// 3. 构建 Driver 和 Writer
// 4. 构建 EngineOptions
// 5. 创建 Engine 实例
// 6. 执行 Engine.RunOnce
// 7. 更新 TaskRun 进度
//
// 错误处理：
// - 配置错误返回永久失败
// - 执行错误根据类型返回可重试或永久失败
func (e *Executor) Run(ctx context.Context, task *model.TaskRun) (syncengine.SyncStats, error) {
	if e == nil {
		return syncengine.SyncStats{}, fmt.Errorf("worker: executor is nil")
	}
	if task == nil {
		return syncengine.SyncStats{}, fmt.Errorf("worker: task is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. 加载 Job 配置
	job, err := e.cfg.JobRepo.GetByID(ctx, task.JobID)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("load job %d: %w", task.JobID, err))
	}
	if !job.Enabled {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("job %d is disabled", job.ID))
	}
	if job.DataServerID == nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("job %d missing data server", job.ID))
	}

	// 2. 加载 DataServer 配置
	server, err := e.cfg.DataServers.GetByID(ctx, *job.DataServerID)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("load data server %d: %w", *job.DataServerID, err))
	}

	execLog := e.log.With(
		zap.Uint("job_id", job.ID),
		zap.String("job_name", job.Name),
		zap.Uint("task_id", task.ID),
	)
	execLog.Info("加载任务配置",
		zap.String("name", job.Name),
		zap.String("watch_mode", job.WatchMode),
		zap.String("source_path", job.SourcePath),
		zap.String("target_path", job.TargetPath),
		zap.String("strm_path", job.STRMPath),
		zap.Bool("enabled", job.Enabled))
	execLog.Info("加载数据服务器配置",
		zap.Uint("server_id", server.ID),
		zap.String("name", server.Name),
		zap.String("type", server.Type),
		zap.String("host", server.Host),
		zap.Int("port", server.Port),
		zap.Bool("enabled", server.Enabled))

	extra, err := parseJobOptions(job.Options)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("parse job options: %w", err))
	}
	execLog.Info("解析任务选项完成",
		zap.Int("max_concurrency", extra.MaxConcurrency),
		zap.Int64("min_file_size", extra.MinFileSize),
		zap.String("metadata_mode", extra.MetadataMode),
		zap.String("strm_mode", extra.STRMMode),
		zap.Int("media_exts", len(extra.MediaExts)),
		zap.Int("meta_exts", len(extra.MetaExts)),
		zap.Int("strm_replace_rules", len(extra.StrmReplaceRules)),
		zap.Any("sync_opts", extra.SyncOpts))

	serverForDriver, err := applyJobStrmMode(server, extra)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("apply job strm_mode: %w", err))
	}

	// 3. 构建 Driver 和 Writer
	driver, err := e.cfg.DriverFactory.Build(ctx, serverForDriver)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build driver for job %s: %w", job.Name, err))
	}

	writer, err := e.cfg.WriterFactory.Build(ctx, job)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build writer for job %s: %w", job.Name, err))
	}
	execLog.Info("构建同步组件完成",
		zap.String("driver_type", driver.Type().String()))

	// 4. 构建 EngineOptions
	engineOpts, err := buildEngineOptions(job, extra)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build engine options: %w", err))
	}

	// 5. 创建 Engine 实例
	engine, err := syncengine.NewEngine(driver, writer, e.log.With(
		zap.Uint("job_id", job.ID),
		zap.String("job_name", job.Name),
		zap.Uint("task_id", task.ID),
		zap.String("driver_type", driver.Type().String()),
	), engineOpts)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("new engine: %w", err))
	}

	// 6. 执行 Engine.RunOnce
	remotePath, err := resolveEngineRemotePath(job, serverForDriver)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("resolve engine remote path: %w", err))
	}
	execLog.Info("开始执行同步任务",
		zap.String("remote_path", remotePath))
	stats, runErr := engine.RunOnce(ctx, remotePath)

	metaStats := metadataStats{}
	var metaErr error
	if runErr == nil {
		metaStats, metaErr = e.syncMetadata(ctx, job, serverForDriver, driver, extra, remotePath)
	}
	if metaErr != nil {
		execLog.Warn("元数据同步失败", zap.Error(metaErr))
	}

	// 7. 更新 TaskRun 进度
	if updateErr := e.cfg.TaskRuns.UpdateProgress(ctx, task.ID, progressFromStats(stats, metaStats)); updateErr != nil {
		e.log.Warn("update task progress failed",
			zap.Uint("task_id", task.ID),
			zap.Error(updateErr))
	}

	if runErr != nil {
		return stats, wrapTaskError(runErr)
	}
	if metaErr != nil {
		return stats, wrapTaskError(metaErr)
	}

	execLog.Info("同步任务执行完成",
		zap.Int64("total_files", stats.TotalFiles),
		zap.Int64("processed_files", stats.ProcessedFiles),
		zap.Int64("created_files", stats.CreatedFiles),
		zap.Int64("updated_files", stats.UpdatedFiles),
		zap.Int64("skipped_files", stats.SkippedFiles),
		zap.Int64("filtered_files", stats.FilteredFiles),
		zap.Int64("failed_files", stats.FailedFiles),
		zap.Int64("meta_total_files", metaStats.Total),
		zap.Int64("meta_processed_files", metaStats.Processed),
		zap.Int64("meta_failed_files", metaStats.Failed),
		zap.Duration("duration", stats.Duration))

	return stats, nil
}

// buildEngineOptions 将 Job 配置转换为 EngineOptions
//
// 从 Job.Options (JSON) 解析可选配置：
// - MaxConcurrency: 最大并发数
// - MediaExts: 媒体文件扩展名过滤
// - MinFileSize: 媒体文件最小大小（MB）
// - DryRun: 干运行模式
// - ForceUpdate: 强制更新
// - SkipExisting: 跳过已存在文件
// - ModTimeEpsilonSeconds: ModTime 容差（秒）
// - EnableOrphanCleanup: 启用孤儿文件清理
// - OrphanCleanupDryRun: 孤儿清理干运行模式
// - StrmReplaceRules: STRM 替换规则
func buildEngineOptions(job model.Job, extra jobOptions) (syncengine.EngineOptions, error) {
	if strings.TrimSpace(job.TargetPath) == "" {
		return syncengine.EngineOptions{}, fmt.Errorf("job %d target_path is empty", job.ID)
	}

	opts := syncengine.EngineOptions{
		OutputRoot: job.TargetPath,
	}

	// 应用可选配置
	if extra.MaxConcurrency > 0 {
		opts.MaxConcurrency = extra.MaxConcurrency
	}
	mediaExts := normalizeExtensions(extra.MediaExts, nil)
	if len(mediaExts) == 0 {
		mediaExts = appconfig.DefaultMediaExtensions()
	}
	opts.FileExtensions = mediaExts
	opts.MinFileSize = normalizeMinFileSize(extra.MinFileSize)
	opts.DryRun = extra.DryRun
	opts.ForceUpdate = extra.ForceUpdate
	opts.SkipExisting = extra.SkipExisting
	opts.EnableOrphanCleanup = extra.EnableOrphanCleanup
	opts.OrphanCleanupDryRun = extra.OrphanCleanupDryRun
	if extra.ModTimeEpsilonSeconds > 0 {
		opts.ModTimeEpsilon = time.Duration(extra.ModTimeEpsilonSeconds) * time.Second
	}
	opts.StrmReplaceRules = normalizeStrmReplaceRules(extra.StrmReplaceRules)
	opts.ExcludeDirs = syncengine.NormalizeExcludeDirs(extra.ExcludeDirs)

	return opts, nil
}

func resolveEngineRemotePath(job model.Job, server model.DataServer) (string, error) {
	remotePath := strings.TrimSpace(job.SourcePath)
	if strings.TrimSpace(server.Type) != filesystem.TypeLocal.String() {
		return remotePath, nil
	}

	opts := dataServerOptions{}
	if strings.TrimSpace(server.Options) != "" {
		if err := json.Unmarshal([]byte(server.Options), &opts); err != nil {
			return "", fmt.Errorf("parse data server options: %w", err)
		}
	}

	accessPath := strings.TrimSpace(opts.AccessPath)
	if accessPath == "" || remotePath == "" {
		return remotePath, nil
	}

	if !filepath.IsAbs(remotePath) && filepath.VolumeName(remotePath) == "" {
		return filepath.ToSlash(strings.TrimLeft(remotePath, "/")), nil
	}

	cleanMount := filepath.Clean(accessPath)
	cleanSource := filepath.Clean(remotePath)
	if cleanSource == cleanMount {
		return "/", nil
	}

	prefix := cleanMount + string(filepath.Separator)
	if strings.HasPrefix(cleanSource, prefix) {
		rel := strings.TrimPrefix(cleanSource, prefix)
		rel = filepath.ToSlash(rel)
		rel = strings.TrimLeft(rel, "/")
		if rel == "" {
			return "/", nil
		}
		return rel, nil
	}

	return "", fmt.Errorf("source_path %s must be under access_path %s", remotePath, accessPath)
}

// jobOptions 表示从 Job.Options 解析的引擎参数
type jobOptions struct {
	MaxConcurrency        int               `json:"max_concurrency"`
	MediaExts             []string          `json:"media_exts"`
	MetaExts              []string          `json:"meta_exts"`
	ExcludeDirs           []string          `json:"exclude_dirs"`
	MinFileSize           int64             `json:"min_file_size"`
	DryRun                bool              `json:"dry_run"`
	ForceUpdate           bool              `json:"force_update"`
	SkipExisting          bool              `json:"skip_existing"`
	ModTimeEpsilonSeconds int               `json:"mod_time_epsilon_seconds"`
	EnableOrphanCleanup   bool              `json:"enable_orphan_cleanup"`
	OrphanCleanupDryRun   bool              `json:"orphan_cleanup_dry_run"`
	MetadataMode          string            `json:"metadata_mode"`
	SyncOpts              syncOpts          `json:"sync_opts"`
	STRMMode              string            `json:"strm_mode"`
	StrmReplaceRules      []strmReplaceRule `json:"strm_replace_rules"`
}

type syncOpts struct {
	UpdateMeta    bool `json:"update_meta"`
	OverwriteMeta bool `json:"overwrite_meta"`
	SkipMeta      bool `json:"skip_meta"`
}

type strmReplaceRule struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type metadataStats struct {
	Total     int64
	Created   int64
	Updated   int64
	Processed int64
	Failed    int64
}

type metaStrategy int

const (
	metaStrategyUpdate metaStrategy = iota
	metaStrategyOverwrite
	metaStrategySkip
)

func (s metaStrategy) String() string {
	switch s {
	case metaStrategyUpdate:
		return "update"
	case metaStrategyOverwrite:
		return "overwrite"
	case metaStrategySkip:
		return "skip"
	default:
		return "unknown"
	}
}

func parseJobOptions(raw string) (jobOptions, error) {
	var opts jobOptions
	if strings.TrimSpace(raw) == "" {
		return opts, nil
	}
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		return jobOptions{}, err
	}
	return opts, nil
}

func normalizeExtensions(exts []string, fallback []string) []string {
	if exts == nil {
		exts = fallback
	}
	if len(exts) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(exts))
	for _, ext := range exts {
		item := strings.ToLower(strings.TrimSpace(ext))
		if item == "" {
			continue
		}
		if !strings.HasPrefix(item, ".") {
			item = "." + item
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func normalizeMinFileSize(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return value * 1024 * 1024
}

func normalizeStrmReplaceRules(rules []strmReplaceRule) []syncengine.StrmReplaceRule {
	if len(rules) == 0 {
		return nil
	}
	normalized := make([]syncengine.StrmReplaceRule, 0, len(rules))
	for _, rule := range rules {
		from := strings.TrimSpace(rule.From)
		to := strings.TrimSpace(rule.To)
		if from == "" && to == "" {
			continue
		}
		normalized = append(normalized, syncengine.StrmReplaceRule{
			From: from,
			To:   to,
		})
	}
	return normalized
}

func resolveMetaStrategy(opts syncOpts) metaStrategy {
	if opts.OverwriteMeta {
		return metaStrategyOverwrite
	}
	if opts.SkipMeta {
		return metaStrategySkip
	}
	return metaStrategyUpdate
}

func buildTargetMetaPath(targetRoot, remotePath string) (string, error) {
	cleanPath := filepath.ToSlash(remotePath)
	cleanPath = filepath.Clean("/" + cleanPath)
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	cleanPath = filepath.FromSlash(cleanPath)
	return filepath.Join(targetRoot, cleanPath), nil
}

func metaFileSame(info os.FileInfo, entry syncengine.RemoteEntry, epsilon time.Duration) bool {
	if info == nil {
		return false
	}
	if entry.Size > 0 && info.Size() != entry.Size {
		return false
	}
	if !entry.ModTime.IsZero() {
		diff := info.ModTime().Sub(entry.ModTime)
		if diff < 0 {
			diff = -diff
		}
		if diff > epsilon {
			return false
		}
	}
	return true
}

func applyJobStrmMode(server model.DataServer, extra jobOptions) (model.DataServer, error) {
	mode := strings.ToLower(strings.TrimSpace(extra.STRMMode))
	if mode == "" {
		return server, nil
	}

	serverType := filesystem.Type(strings.TrimSpace(server.Type))
	if !serverType.IsValid() {
		return server, fmt.Errorf("invalid server type: %s", server.Type)
	}

	options := map[string]interface{}{}
	if strings.TrimSpace(server.Options) != "" {
		if err := json.Unmarshal([]byte(server.Options), &options); err != nil {
			return server, fmt.Errorf("parse data server options: %w", err)
		}
	}

	mountPath := strings.TrimSpace(getOptionString(options, "mount_path"))
	if mountPath == "" {
		mountPath = strings.TrimSpace(getOptionString(options, "access_path"))
	}

	var strmMode filesystem.STRMMode
	switch mode {
	case "local":
		strmMode = filesystem.STRMModeMount
	case "url":
		strmMode = filesystem.STRMModeHTTP
	default:
		return server, nil
	}

	if serverType == filesystem.TypeLocal {
		strmMode = filesystem.STRMModeMount
	} else if strmMode == filesystem.STRMModeMount && mountPath == "" {
		strmMode = filesystem.STRMModeHTTP
	}

	options["strm_mode"] = strmMode.String()
	encoded, err := json.Marshal(options)
	if err != nil {
		return server, fmt.Errorf("encode data server options: %w", err)
	}

	server.Options = string(encoded)
	return server, nil
}

func getOptionString(options map[string]interface{}, key string) string {
	if options == nil {
		return ""
	}
	raw, ok := options[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func buildMetadataClient(server model.DataServer, log *zap.Logger) (filesystem.Client, error) {
	cfg, err := buildFilesystemConfig(server)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(cfg.MountPath) != "" {
		cfg.STRMMode = filesystem.STRMModeMount
	} else {
		cfg.STRMMode = filesystem.STRMModeHTTP
	}

	return filesystem.NewClient(cfg, filesystem.WithLogger(log))
}

func (e *Executor) syncMetadata(ctx context.Context, job model.Job, server model.DataServer, driver syncengine.Driver, extra jobOptions, remotePath string) (metadataStats, error) {
	metaExts := normalizeExtensions(extra.MetaExts, appconfig.DefaultMetaExtensions())
	if len(metaExts) == 0 {
		return metadataStats{}, nil
	}

	metaSet := make(map[string]struct{}, len(metaExts))
	for _, ext := range metaExts {
		metaSet[ext] = struct{}{}
	}

	excludeDirs := syncengine.NormalizeExcludeDirs(extra.ExcludeDirs)

	entries, err := driver.List(ctx, remotePath, syncengine.ListOptions{
		Recursive: true,
		MaxDepth:  100,
	})
	if err != nil {
		return metadataStats{}, fmt.Errorf("list metadata entries: %w", err)
	}

	metaLogger := e.log.With(zap.String("component", "metadata"))
	client, err := buildMetadataClient(server, metaLogger)
	if err != nil {
		return metadataStats{}, fmt.Errorf("build metadata client: %w", err)
	}

	mode := strings.ToLower(strings.TrimSpace(extra.MetadataMode))
	if mode == "api" {
		mode = "download"
	}
	preferMount := mode != "download"
	replicator := appsync.NewMetadataReplicator(client, job.TargetPath, metaLogger, appsync.WithPreferMount(preferMount))

	strategy := resolveMetaStrategy(extra.SyncOpts)
	metaLogger.Info("开始同步元数据",
		zap.String("remote_path", remotePath),
		zap.Int("meta_exts", len(metaExts)),
		zap.String("mode", mode),
		zap.String("strategy", strategy.String()))

	items := make(chan appports.SyncPlanItem, 100)
	stats := metadataStats{}
	preFailed := int64(0)
	planned := int64(0)

	go func() {
		defer close(items)
		for _, entry := range entries {
			if ctx.Err() != nil {
				return
			}
			if entry.IsDir {
				continue
			}
			if syncengine.IsExcludedPath(remotePath, entry.Path, excludeDirs) {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name))
			if _, ok := metaSet[ext]; !ok {
				continue
			}

			stats.Total++

			targetPath, err := buildTargetMetaPath(job.TargetPath, entry.Path)
			if err != nil {
				preFailed++
				continue
			}

			info, statErr := os.Stat(targetPath)
			if statErr != nil && !os.IsNotExist(statErr) {
				preFailed++
				continue
			}

			exists := statErr == nil
			same := metaFileSame(info, entry, 2*time.Second)

			switch strategy {
			case metaStrategySkip:
				if exists {
					continue
				}
			case metaStrategyUpdate:
				if exists && same {
					continue
				}
			case metaStrategyOverwrite:
				// 总是覆盖
			}

			op := appports.SyncOpCreate
			if exists {
				op = appports.SyncOpUpdate
			}
			if op == appports.SyncOpCreate {
				stats.Created++
			} else {
				stats.Updated++
			}

			items <- appports.SyncPlanItem{
				Op:             op,
				Kind:           appports.PlanItemMetadata,
				SourcePath:     entry.Path,
				TargetMetaPath: targetPath,
				Size:           entry.Size,
				ModTime:        entry.ModTime,
			}
			planned++
		}
	}()

	_, failed, err := replicator.Apply(ctx, items)
	stats.Processed = planned + preFailed
	stats.Failed = preFailed + int64(failed)
	if err != nil {
		return stats, err
	}

	metaLogger.Info("元数据同步完成",
		zap.Int64("total", stats.Total),
		zap.Int64("created", stats.Created),
		zap.Int64("updated", stats.Updated),
		zap.Int64("processed", stats.Processed),
		zap.Int64("failed", stats.Failed))

	return stats, nil
}

// progressFromStats 生成 TaskRunProgress
//
// 计算进度百分比：
// - 如果有总文件数，按 ProcessedFiles / TotalFiles 计算
// - 如果没有总文件数但已完成，进度为 100
func progressFromStats(stats syncengine.SyncStats, meta metadataStats) TaskRunProgress {
	total := clampInt64(stats.TotalFiles)
	processed := clampInt64(stats.ProcessedFiles)
	failed := clampInt64(stats.FailedFiles)

	progress := 0
	if total > 0 {
		raw := int((int64(processed) * 100) / int64(total))
		if raw > 100 {
			raw = 100 // 防止ProcessedFiles超过TotalFiles时溢出
		}
		progress = raw
	} else if !stats.EndTime.IsZero() && len(stats.Errors) == 0 {
		// 没有总文件数，但已完成且无错误，进度为 100
		progress = 100
	}

	return TaskRunProgress{
		TotalFiles:         total,
		ProcessedFiles:     processed,
		FailedFiles:        failed,
		CreatedFiles:       clampInt64(stats.CreatedFiles),
		UpdatedFiles:       clampInt64(stats.UpdatedFiles),
		SkippedFiles:       clampInt64(stats.SkippedFiles),
		FilteredFiles:      clampInt64(stats.FilteredFiles),
		MetaTotalFiles:     clampInt64(meta.Total),
		MetaCreatedFiles:   clampInt64(meta.Created),
		MetaUpdatedFiles:   clampInt64(meta.Updated),
		MetaProcessedFiles: clampInt64(meta.Processed),
		MetaFailedFiles:    clampInt64(meta.Failed),
		Progress:           progress,
	}
}

// clampInt64 将 int64 安全转换为 int
//
// 避免溢出：
// - 超过 int.MaxValue 时返回 int.MaxValue
// - 负数时返回 0
func clampInt64(v int64) int {
	if v > int64(math.MaxInt) {
		return math.MaxInt
	}
	if v < 0 {
		return 0
	}
	return int(v)
}

// wrapTaskError 将错误转换为可识别的 TaskError
//
// 错误分类：
// - context.Canceled / DeadlineExceeded -> Cancelled
// - syncengine.ErrInvalidInput / ErrNotSupported -> Permanent
// - 其他错误 -> Retryable
func wrapTaskError(err error) error {
	if err == nil {
		return nil
	}

	// 已经是 TaskError，直接返回
	var taskErr *syncqueue.TaskError
	if errors.As(err, &taskErr) {
		return err
	}

	// Context 取消或超时
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &syncqueue.TaskError{Kind: syncqueue.FailureCancelled, Err: err}
	}

	// Engine 错误分类
	if errors.Is(err, syncengine.ErrInvalidInput) || errors.Is(err, syncengine.ErrNotSupported) {
		return &syncqueue.TaskError{Kind: syncqueue.FailurePermanent, Err: err}
	}

	// 默认为可重试
	return &syncqueue.TaskError{Kind: syncqueue.FailureRetryable, Err: err}
}

// permanentTaskError 创建永久失败的 TaskError
func permanentTaskError(err error) error {
	if err == nil {
		return nil
	}
	return &syncqueue.TaskError{Kind: syncqueue.FailurePermanent, Err: err}
}

// DefaultDriverFactory 根据 DataServer.Type 构建 Driver
type DefaultDriverFactory struct {
	Logger *zap.Logger
}

// Build 构建同步引擎 Driver 实例
//
// 流程：
// 1. 将 DataServer 转换为 filesystem.Config
// 2. 创建 filesystem.Client
func (f DefaultDriverFactory) Build(ctx context.Context, server model.DataServer) (syncengine.Driver, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(server.Type) == "" {
		return nil, fmt.Errorf("driver factory: server type is empty")
	}

	// 构建 filesystem.Config
	cfg, err := buildFilesystemConfig(server)
	if err != nil {
		return nil, err
	}

	// 创建 filesystem.Client
	client, err := filesystem.NewClient(cfg, filesystem.WithLogger(f.logger()))
	if err != nil {
		return nil, fmt.Errorf("driver factory: new filesystem client: %w", err)
	}

	// 适配为 syncengine.Driver
	driverType := syncengine.DriverType(cfg.Type.String())
	adapter, err := filesystem.NewAdapter(client, driverType)
	if err != nil {
		return nil, fmt.Errorf("driver factory: new adapter: %w", err)
	}

	return adapter, nil
}

func (f DefaultDriverFactory) logger() *zap.Logger {
	if f.Logger != nil {
		return f.Logger
	}
	return logger.With(zap.String("component", "worker-driver-factory"))
}

// DefaultWriterFactory 构建本地 STRM 写入器
type DefaultWriterFactory struct {
	Logger *zap.Logger
}

// Build 创建 Writer 实例
//
// 使用 strmwriter.NewLocalWriter 创建本地文件系统写入器。
func (f DefaultWriterFactory) Build(ctx context.Context, job model.Job) (syncengine.Writer, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(job.TargetPath) == "" {
		return nil, fmt.Errorf("writer factory: job %d target_path is empty", job.ID)
	}

	writer, err := strmwriter.NewLocalWriter(job.TargetPath, strmwriter.WithEnforceRoot(true))
	if err != nil {
		return nil, fmt.Errorf("writer factory: new local writer: %w", err)
	}
	return writer, nil
}

// buildFilesystemConfig 将 DataServer 转换为 filesystem.Config
//
// 从 DataServer.Options (JSON) 解析可选配置：
// - BaseURL: 服务器基础 URL
// - STRMMode: STRM 模式（http/mount）
// - MountPath: 挂载路径
// - TimeoutSeconds: 请求超时（秒）
// - Username: 用户名
// - Password: 密码
func buildFilesystemConfig(server model.DataServer) (filesystem.Config, error) {
	// 解析 Options JSON
	opts := dataServerOptions{}
	if strings.TrimSpace(server.Options) != "" {
		if err := json.Unmarshal([]byte(server.Options), &opts); err != nil {
			return filesystem.Config{}, fmt.Errorf("parse data server options: %w", err)
		}
	}

	// 验证服务器类型
	serverType := filesystem.Type(strings.TrimSpace(server.Type))
	if !serverType.IsValid() {
		return filesystem.Config{}, fmt.Errorf("invalid server type: %s", server.Type)
	}

	// 构造 BaseURL
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" && serverType != filesystem.TypeLocal {
		baseURL = fmt.Sprintf("http://%s:%d", strings.TrimSpace(server.Host), server.Port)
	}

	// 解析 STRMMode（显式校验，返回更明确的错误）
	strmMode := filesystem.STRMModeHTTP
	if strings.TrimSpace(opts.STRMMode) != "" {
		strmMode = filesystem.STRMMode(strings.TrimSpace(opts.STRMMode))
		if !strmMode.IsValid() {
			return filesystem.Config{}, fmt.Errorf("invalid strm_mode: %s (valid: http, mount)", opts.STRMMode)
		}
	} else if serverType == filesystem.TypeLocal {
		// 本地数据服务器默认使用挂载模式，避免 HTTP 模式缺少 base_url
		strmMode = filesystem.STRMModeMount
	}
	if serverType == filesystem.TypeLocal {
		strmMode = filesystem.STRMModeMount
	}

	// 解析 Timeout
	timeout := time.Duration(opts.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	// 解析 Password（优先使用 Options 中的 Password，否则使用 APIKey）
	password := opts.Password
	if strings.TrimSpace(password) == "" {
		password = server.APIKey
	}

	accessPath := strings.TrimSpace(opts.AccessPath)
	mountPath := strings.TrimSpace(opts.MountPath)
	scanRoot := mountPath
	if serverType == filesystem.TypeLocal && accessPath != "" {
		scanRoot = accessPath
	}
	if scanRoot == "" {
		scanRoot = mountPath
	}
	strmMount := mountPath
	if strmMount == "" {
		strmMount = scanRoot
	}

	return filesystem.Config{
		Type:          serverType,
		BaseURL:       baseURL,
		Username:      opts.Username,
		Password:      password,
		STRMMode:      strmMode,
		MountPath:     scanRoot,
		StrmMountPath: strmMount,
		Timeout:       timeout,
	}, nil
}

// dataServerOptions 表示 DataServer.Options 的可选字段
type dataServerOptions struct {
	BaseURL        string `json:"base_url"`
	AccessPath     string `json:"access_path"`
	STRMMode       string `json:"strm_mode"`
	MountPath      string `json:"mount_path"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Username       string `json:"username"`
	Password       string `json:"password"`
}

// GormTaskRunRepository 是基于 GORM 的 TaskRunRepository 实现
type GormTaskRunRepository struct {
	db *gorm.DB
}

// NewGormTaskRunRepository 创建 GormTaskRunRepository
func NewGormTaskRunRepository(db *gorm.DB) (*GormTaskRunRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("worker: gorm db is nil")
	}
	return &GormTaskRunRepository{db: db}, nil
}

// UpdateProgress 更新 TaskRun 进度字段
func (r *GormTaskRunRepository) UpdateProgress(ctx context.Context, taskID uint, progress TaskRunProgress) error {
	if ctx == nil {
		ctx = context.Background()
	}
	updates := map[string]any{
		"total_files":          progress.TotalFiles,
		"processed_files":      progress.ProcessedFiles,
		"failed_files":         progress.FailedFiles,
		"created_files":        progress.CreatedFiles,
		"updated_files":        progress.UpdatedFiles,
		"skipped_files":        progress.SkippedFiles,
		"filtered_files":       progress.FilteredFiles,
		"meta_total_files":     progress.MetaTotalFiles,
		"meta_created_files":   progress.MetaCreatedFiles,
		"meta_updated_files":   progress.MetaUpdatedFiles,
		"meta_processed_files": progress.MetaProcessedFiles,
		"meta_failed_files":    progress.MetaFailedFiles,
		"progress":             progress.Progress,
	}
	return r.db.WithContext(ctx).Model(&model.TaskRun{}).
		Where("id = ?", taskID).
		Updates(updates).Error
}
