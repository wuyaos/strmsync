// Package worker 提供 Worker 执行器实现
package worker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
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
	TaskRunEvents TaskRunEventRepository
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

	extra := job.Options
	if extra.SyncOpts.FullResync {
		extra.ForceUpdate = true
		extra.SkipExisting = false
	}
	execLog.Info("解析任务选项完成",
		zap.Int("max_concurrency", extra.MaxConcurrency),
		zap.Int64("min_file_size", extra.MinFileSize),
		zap.String("metadata_mode", extra.MetadataMode),
		zap.String("strm_mode", extra.STRMMode),
		zap.Bool("force_update", extra.ForceUpdate),
		zap.Bool("full_resync", extra.SyncOpts.FullResync),
		zap.Int("media_exts", len(extra.MediaExts)),
		zap.Int("meta_exts", len(extra.MetaExts)),
		zap.Int("strm_replace_rules", len(extra.StrmReplaceRules)),
		zap.Any("sync_opts", extra.SyncOpts))

	serverForDriver, err := applyJobStrmMode(server, extra)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("apply job strm_mode: %w", err))
	}

	// 3. 构建 Driver 和 Writer
	driverServer := serverForDriver
	useLocalStrm := shouldUseLocalStrmDriver(serverForDriver, extra)
	if useLocalStrm {
		localServer, err := buildLocalDriverServer(job, serverForDriver)
		if err != nil {
			return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build local driver server: %w", err))
		}
		driverServer = localServer
		execLog.Info("STRM 使用本地路径驱动",
			zap.String("server_type", serverForDriver.Type),
			zap.String("source_path", job.SourcePath),
			zap.String("access_path", strings.TrimSpace(getAccessPathFromServer(serverForDriver))),
			zap.String("mount_path", strings.TrimSpace(getMountPathFromServer(serverForDriver))))
	}

	driver, err := e.cfg.DriverFactory.Build(ctx, driverServer)
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
	// 设置挂载路径映射（系统级基线转换，在用户替换规则之前执行）
	if useLocalStrm {
		accessPath := filepath.Clean(strings.TrimSpace(getAccessPathFromServer(serverForDriver)))
		mountPath := filepath.Clean(strings.TrimSpace(getMountPathFromServer(serverForDriver)))
		if accessPath != "" && mountPath != "" && accessPath != mountPath {
			engineOpts.MountPathMapping = &syncengine.MountPathMapping{
				From: accessPath,
				To:   mountPath,
			}
		}
	}
	if extra.PreferRemoteList && useLocalStrm && isRemoteServerType(serverForDriver.Type) {
		remoteRoot, err := resolveRemoteListRoot(job, serverForDriver)
		if err != nil {
			if hasAccessPath(serverForDriver) {
				execLog.Warn("远程列表不可用，回退本地遍历",
					zap.String("source_path", job.SourcePath),
					zap.Error(err))
			} else {
				return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("resolve remote list root: %w", err))
			}
		} else {
			remoteDriver, err := e.cfg.DriverFactory.Build(ctx, serverForDriver)
			if err != nil {
				return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build remote list driver: %w", err))
			}
			var fallbackDriver syncengine.Driver
			if hasAccessPath(serverForDriver) {
				localServer, err := buildLocalDriverServer(job, serverForDriver)
				if err != nil {
					return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build local fallback driver: %w", err))
				}
				fallbackDriver, err = e.cfg.DriverFactory.Build(ctx, localServer)
				if err != nil {
					return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build local fallback driver: %w", err))
				}
			}
			engineOpts.ListOverride = buildRemoteListOverride(remoteDriver, remoteRoot, fallbackDriver)
			execLog.Info("远程列表 + 本地 STRM",
				zap.String("remote_root", remoteRoot),
				zap.String("source_path", job.SourcePath),
				zap.Bool("prefer_remote_list", extra.PreferRemoteList))
		}
	}
	var eventSink *taskRunEventSink
	if e.cfg.TaskRunEvents != nil {
		eventLogger := e.log.With(zap.String("component", "task-run-event"))
		eventSink = newTaskRunEventSink(e.cfg.TaskRunEvents, task.ID, job.ID, job.Name, eventLogger)
		engineOpts.EventSink = eventSink
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
		zap.String("remote_root", remotePath))
	stats, runErr := engine.RunOnce(ctx, remotePath)

	metaStats := metadataStats{}
	var metaErr error
	if runErr == nil {
		metaStats, metaErr = e.syncMetadata(ctx, job, serverForDriver, driver, extra, remotePath, eventSink)
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
// 从 Job.Options 读取可选配置：
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
func buildEngineOptions(job model.Job, extra model.JobOptions) (syncengine.EngineOptions, error) {
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
		if strings.TrimSpace(job.RemoteRoot) != "" {
			return strings.TrimSpace(job.RemoteRoot), nil
		}
		return remotePath, nil
	}

	opts := server.Options

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

func shouldUseLocalStrmDriver(server model.DataServer, extra model.JobOptions) bool {
	if !isRemoteServerType(server.Type) {
		return false
	}
	if strings.ToLower(strings.TrimSpace(server.Type)) == filesystem.TypeOpenList.String() {
		if strings.TrimSpace(getAccessPathFromServer(server)) == "" {
			return false
		}
	}
	mode := strings.ToLower(strings.TrimSpace(extra.STRMMode))
	return mode != "url"
}

func isRemoteServerType(serverType string) bool {
	switch strings.ToLower(strings.TrimSpace(serverType)) {
	case filesystem.TypeCloudDrive2.String(), filesystem.TypeOpenList.String():
		return true
	default:
		return false
	}
}

func buildLocalDriverServer(job model.Job, server model.DataServer) (model.DataServer, error) {
	sourcePath := strings.TrimSpace(job.SourcePath)
	if sourcePath == "" {
		return model.DataServer{}, fmt.Errorf("source_path is required for local driver")
	}

	accessPath := strings.TrimSpace(getAccessPathFromServer(server))
	if accessPath == "" {
		accessPath = sourcePath
	}
	mountPath := strings.TrimSpace(getMountPathFromServer(server))
	if mountPath == "" {
		mountPath = accessPath
	}

	opts := model.DataServerOptions{
		AccessPath: accessPath,
		MountPath:  mountPath,
		STRMMode:   filesystem.STRMModeMount.String(),
	}
	return model.DataServer{
		Type:    filesystem.TypeLocal.String(),
		Options: opts,
	}, nil
}

func resolveRemoteListRoot(job model.Job, server model.DataServer) (string, error) {
	if strings.TrimSpace(job.RemoteRoot) != "" {
		return normalizeRemoteRoot(job.RemoteRoot), nil
	}
	opts := server.Options
	accessPath := strings.TrimSpace(opts.AccessPath)
	sourcePath := strings.TrimSpace(job.SourcePath)
	if sourcePath == "" {
		return "", fmt.Errorf("source_path is required for remote listing")
	}
	mountPath := strings.TrimSpace(opts.MountPath)

	if accessPath == "" {
		return normalizeRemoteRoot(sourcePath), nil
	}

	if mountPath != "" && (filepath.IsAbs(sourcePath) || filepath.VolumeName(sourcePath) != "") {
		cleanMount := filepath.Clean(mountPath)
		cleanSource := filepath.Clean(sourcePath)
		if cleanSource == cleanMount {
			return normalizeRemoteRoot(accessPath), nil
		}
		prefix := cleanMount + string(filepath.Separator)
		if strings.HasPrefix(cleanSource, prefix) {
			rel := strings.TrimPrefix(cleanSource, prefix)
			rel = filepath.ToSlash(rel)
			rel = strings.TrimLeft(rel, "/")
			if rel == "" {
				return normalizeRemoteRoot(accessPath), nil
			}
			return normalizeRemoteRoot(path.Join(accessPath, rel)), nil
		}
		return "", fmt.Errorf("source_path %s must be under mount_path %s", sourcePath, mountPath)
	}

	if !filepath.IsAbs(sourcePath) && filepath.VolumeName(sourcePath) == "" {
		return normalizeRemoteRoot(path.Join(accessPath, filepath.ToSlash(strings.TrimLeft(sourcePath, "/")))), nil
	}

	return "", fmt.Errorf("source_path %s must be under mount_path %s", sourcePath, mountPath)
}

func getAccessPathFromServer(server model.DataServer) string {
	return strings.TrimSpace(server.Options.AccessPath)
}

func getMountPathFromServer(server model.DataServer) string {
	return strings.TrimSpace(server.Options.MountPath)
}

func hasAccessPath(server model.DataServer) bool {
	return strings.TrimSpace(getAccessPathFromServer(server)) != ""
}

func resolveLocalListPath(job model.Job) string {
	sourcePath := strings.TrimSpace(job.SourcePath)
	if sourcePath == "" {
		return "/"
	}
	if filepath.IsAbs(sourcePath) || filepath.VolumeName(sourcePath) != "" {
		return "/"
	}
	cleaned := filepath.ToSlash(strings.TrimLeft(sourcePath, "/"))
	if cleaned == "" {
		return "/"
	}
	return cleaned
}

func buildLocalMetadataListDriver(ctx context.Context, job model.Job, server model.DataServer, factory DriverFactory) (syncengine.Driver, string, error) {
	localServer, err := buildLocalDriverServer(job, server)
	if err != nil {
		return nil, "", err
	}
	localDriver, err := factory.Build(ctx, localServer)
	if err != nil {
		return nil, "", err
	}
	return localDriver, resolveLocalListPath(job), nil
}

func normalizeRemoteRoot(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "/"
	}
	cleaned := strings.ReplaceAll(trimmed, "\\", "/")
	return path.Clean("/" + cleaned)
}

func buildRemoteListOverride(remoteDriver syncengine.Driver, remoteRoot string, fallbackDriver syncengine.Driver) func(ctx context.Context, remotePath string, opt syncengine.ListOptions) ([]syncengine.RemoteEntry, error) {
	root := normalizeRemoteRoot(remoteRoot)
	return func(ctx context.Context, remotePath string, opt syncengine.ListOptions) ([]syncengine.RemoteEntry, error) {
		entries, err := remoteDriver.List(ctx, root, opt)
		if err != nil {
			if fallbackDriver != nil {
				return fallbackDriver.List(ctx, remotePath, opt)
			}
			return nil, fmt.Errorf("remote list %s failed: %w", root, err)
		}
		return mapRemoteEntriesToLocal(root, entries), nil
	}
}

func mapRemoteEntriesToLocal(root string, entries []syncengine.RemoteEntry) []syncengine.RemoteEntry {
	if len(entries) == 0 {
		return nil
	}
	cleanRoot := normalizeRemoteRoot(root)
	cleanRoot = strings.TrimRight(cleanRoot, "/")
	if cleanRoot == "" {
		cleanRoot = "/"
	}
	mapped := make([]syncengine.RemoteEntry, 0, len(entries))
	for _, entry := range entries {
		entryPath := normalizeRemoteRoot(entry.Path)
		rel := ""
		if cleanRoot == "/" {
			rel = strings.TrimPrefix(entryPath, "/")
		} else if entryPath == cleanRoot {
			rel = ""
		} else if strings.HasPrefix(entryPath, cleanRoot+"/") {
			rel = strings.TrimPrefix(entryPath, cleanRoot+"/")
		} else {
			continue
		}
		localPath := path.Clean("/" + rel)
		entry.Path = localPath
		mapped = append(mapped, entry)
	}
	return mapped
}

// model.JobOptions 表示 Job.Options 的引擎参数
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

func normalizeStrmReplaceRules(rules []model.StrmReplaceRule) []syncengine.StrmReplaceRule {
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

func resolveMetaStrategy(opts model.SyncOpts) metaStrategy {
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

func applyJobStrmMode(server model.DataServer, extra model.JobOptions) (model.DataServer, error) {
	mode := strings.ToLower(strings.TrimSpace(extra.STRMMode))
	if mode == "" {
		return server, nil
	}

	serverType := filesystem.Type(strings.TrimSpace(server.Type))
	if !serverType.IsValid() {
		return server, fmt.Errorf("invalid server type: %s", server.Type)
	}

	options := server.Options
	mountPath := strings.TrimSpace(options.MountPath)
	if mountPath == "" {
		mountPath = strings.TrimSpace(options.AccessPath)
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

	options.STRMMode = strmMode.String()
	server.Options = options
	return server, nil
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

func buildLocalMetadataClient(job model.Job, log *zap.Logger) (filesystem.Client, error) {
	sourcePath := strings.TrimSpace(job.SourcePath)
	if sourcePath == "" {
		return nil, fmt.Errorf("source_path is required for local metadata client")
	}
	cfg := filesystem.Config{
		Type:          filesystem.TypeLocal,
		MountPath:     sourcePath,
		StrmMountPath: sourcePath,
		STRMMode:      filesystem.STRMModeMount,
		Timeout:       10 * time.Second,
	}
	return filesystem.NewClient(cfg, filesystem.WithLogger(log))
}

func (e *Executor) syncMetadata(ctx context.Context, job model.Job, server model.DataServer, driver syncengine.Driver, extra model.JobOptions, remotePath string, eventSink *taskRunEventSink) (metadataStats, error) {
	metaExts := normalizeExtensions(extra.MetaExts, appconfig.DefaultMetaExtensions())
	if len(metaExts) == 0 {
		return metadataStats{}, nil
	}

	metaSet := make(map[string]struct{}, len(metaExts))
	for _, ext := range metaExts {
		metaSet[ext] = struct{}{}
	}

	excludeDirs := syncengine.NormalizeExcludeDirs(extra.ExcludeDirs)

	mode := strings.ToLower(strings.TrimSpace(extra.MetadataMode))
	if mode == "api" {
		mode = "download"
	}
	if mode == "none" {
		return metadataStats{}, nil
	}
	// Local 模式不支持下载
	if mode == "download" && strings.ToLower(strings.TrimSpace(server.Type)) == "local" {
		return metadataStats{}, fmt.Errorf("metadata_mode 'download' is not supported for local server type, use 'copy' instead")
	}
	if mode != "download" && isRemoteServerType(server.Type) && strings.TrimSpace(getAccessPathFromServer(server)) == "" {
		return metadataStats{}, fmt.Errorf("metadata_mode %s requires access_path", mode)
	}

	listDriver := driver
	listPath := remotePath
	var err error
	if mode == "download" && isRemoteServerType(server.Type) {
		if extra.PreferRemoteList || !hasAccessPath(server) {
			remoteRoot, err := resolveRemoteListRoot(job, server)
			if err != nil {
				if hasAccessPath(server) {
					listDriver, listPath, err = buildLocalMetadataListDriver(ctx, job, server, e.cfg.DriverFactory)
					if err != nil {
						return metadataStats{}, fmt.Errorf("build local metadata driver: %w", err)
					}
				} else {
					return metadataStats{}, fmt.Errorf("resolve remote metadata root: %w", err)
				}
			} else {
				remoteDriver, err := e.cfg.DriverFactory.Build(ctx, server)
				if err != nil {
					return metadataStats{}, fmt.Errorf("build remote metadata driver: %w", err)
				}
				listDriver = remoteDriver
				listPath = remoteRoot
			}
		} else {
			listDriver, listPath, err = buildLocalMetadataListDriver(ctx, job, server, e.cfg.DriverFactory)
			if err != nil {
				return metadataStats{}, fmt.Errorf("build local metadata driver: %w", err)
			}
		}
	}

	entries, err := listDriver.List(ctx, listPath, syncengine.ListOptions{
		Recursive: true,
		MaxDepth:  100,
	})
	if err != nil {
		return metadataStats{}, fmt.Errorf("list metadata entries: %w", err)
	}

	metaLogger := e.log.With(zap.String("component", "metadata"))
	var client filesystem.Client
	if mode == "download" {
		client, err = buildMetadataClient(server, metaLogger)
	} else if isRemoteServerType(server.Type) {
		client, err = buildLocalMetadataClient(job, metaLogger)
	} else {
		client, err = buildMetadataClient(server, metaLogger)
	}
	if err != nil {
		return metadataStats{}, fmt.Errorf("build metadata client: %w", err)
	}

	preferMount := mode != "download"
	options := []appsync.MetadataReplicatorOption{
		appsync.WithPreferMount(preferMount),
	}
	if eventSink != nil {
		options = append(options, appsync.WithEventSink(eventSink))
	}
	replicator := appsync.NewMetadataReplicator(client, job.TargetPath, metaLogger, options...)

	strategy := resolveMetaStrategy(extra.SyncOpts)
	metaLogger.Info("开始同步元数据",
		zap.String("remote_root", remotePath),
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
					if eventSink != nil {
						eventSink.OnMetaEvent(ctx, appsync.MetaEvent{
							Op:           "skip",
							Status:       "skipped",
							SourcePath:   entry.Path,
							TargetPath:   targetPath,
							ErrorMessage: "skip_existing",
						})
					}
					continue
				}
			case metaStrategyUpdate:
				if exists && same {
					if eventSink != nil {
						eventSink.OnMetaEvent(ctx, appsync.MetaEvent{
							Op:           "skip",
							Status:       "skipped",
							SourcePath:   entry.Path,
							TargetPath:   targetPath,
							ErrorMessage: "unchanged",
						})
					}
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
// - 其他错误 -> 交由 syncqueue.ClassifyFailureKind 判断
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

	kind := syncqueue.ClassifyFailureKind(err)
	return &syncqueue.TaskError{Kind: kind, Err: err}
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
// 从 DataServer.Options 读取可选配置：
// - BaseURL: 服务器基础 URL
// - STRMMode: STRM 模式（http/mount）
// - MountPath: 挂载路径
// - TimeoutSeconds: 请求超时（秒）
// - Username: 用户名
// - Password: 密码
func buildFilesystemConfig(server model.DataServer) (filesystem.Config, error) {
	opts := server.Options

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

// GormTaskRunEventRepository 是基于 GORM 的 TaskRunEventRepository 实现
type GormTaskRunEventRepository struct {
	db *gorm.DB
}

// NewGormTaskRunEventRepository 创建 GormTaskRunEventRepository
func NewGormTaskRunEventRepository(db *gorm.DB) (*GormTaskRunEventRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("worker: gorm db is nil")
	}
	return &GormTaskRunEventRepository{db: db}, nil
}

// Create 写入单条执行事件
func (r *GormTaskRunEventRepository) Create(ctx context.Context, event *model.TaskRunEvent) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("worker: task run event repo is nil")
	}
	if event == nil {
		return fmt.Errorf("worker: task run event is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return r.db.WithContext(ctx).Create(event).Error
}

type taskRunEventSink struct {
	repo    TaskRunEventRepository
	now     func() time.Time
	task    uint
	job     uint
	jobName string
	logger  *zap.Logger
}

func newTaskRunEventSink(repo TaskRunEventRepository, taskID uint, jobID uint, jobName string, logger *zap.Logger) *taskRunEventSink {
	if repo == nil {
		return nil
	}
	return &taskRunEventSink{
		repo:    repo,
		now:     time.Now,
		task:    taskID,
		job:     jobID,
		jobName: strings.TrimSpace(jobName),
		logger:  logger,
	}
}

func (s *taskRunEventSink) logEvent(kind string, op string, status string, source string, target string, errMsg string) {
	if s == nil || s.logger == nil {
		return
	}
	name := s.jobName
	if name == "" {
		name = fmt.Sprintf("任务%d", s.job)
	} else if !strings.HasPrefix(name, "任务") {
		name = "任务" + name
	}
	action := formatEventAction(kind, op)
	message := formatEventMessage(name, action, source, target, status, errMsg)
	s.logger.Info(message)
}

func formatEventAction(kind string, op string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	op = strings.ToLower(strings.TrimSpace(op))
	switch kind {
	case "meta":
		if op == "copy" || op == "create" {
			return "复制元数据"
		}
		if op == "update" {
			return "更新元数据"
		}
		if op == "delete" {
			return "删除元数据"
		}
		if op == "skip" {
			return "跳过元数据"
		}
	default:
		if op == "create" {
			return "生成STRM"
		}
		if op == "update" {
			return "更新STRM"
		}
		if op == "delete" {
			return "删除STRM"
		}
		if op == "skip" {
			return "跳过STRM"
		}
	}
	if op == "" {
		return "操作"
	}
	return "操作" + op
}

func formatEventMessage(jobName string, action string, source string, target string, status string, errMsg string) string {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	status = strings.ToLower(strings.TrimSpace(status))
	errMsg = strings.TrimSpace(errMsg)

	pathPart := source
	if target != "" {
		if pathPart != "" {
			pathPart = pathPart + " => " + target
		} else {
			pathPart = target
		}
	}

	reason := ""
	if status == "skipped" {
		switch errMsg {
		case "skip_existing":
			reason = "已存在,跳过"
		case "unchanged":
			reason = "内容未变化,跳过"
		case "dry_run":
			reason = "dry_run,跳过"
		default:
			if errMsg != "" {
				reason = errMsg
			} else {
				reason = "已跳过"
			}
		}
	} else if status == "failed" && errMsg != "" {
		reason = errMsg
	}

	if pathPart != "" && reason != "" {
		return fmt.Sprintf("%s %s %s %s", jobName, action, pathPart, reason)
	}
	if pathPart != "" {
		return fmt.Sprintf("%s %s %s", jobName, action, pathPart)
	}
	if reason != "" {
		return fmt.Sprintf("%s %s %s", jobName, action, reason)
	}
	return fmt.Sprintf("%s %s", jobName, action)
}

func (s *taskRunEventSink) OnStrmEvent(ctx context.Context, event syncengine.StrmEvent) {
	if s == nil || s.repo == nil {
		return
	}
	op := strings.TrimSpace(event.Op)
	if op == "" {
		op = "unknown"
	}
	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = "success"
	}
	errMsg := strings.TrimSpace(event.ErrorMessage)
	record := &model.TaskRunEvent{
		TaskRunID:    s.task,
		JobID:        s.job,
		Kind:         "strm",
		Op:           op,
		Status:       status,
		SourcePath:   strings.TrimSpace(event.SourcePath),
		TargetPath:   strings.TrimSpace(event.TargetPath),
		ErrorMessage: errMsg,
		CreatedAt:    s.now(),
	}
	_ = s.repo.Create(ctx, record)
	s.logEvent("strm", op, status, record.SourcePath, record.TargetPath, errMsg)
}

func (s *taskRunEventSink) OnMetaEvent(ctx context.Context, event appsync.MetaEvent) {
	if s == nil || s.repo == nil {
		return
	}
	op := strings.TrimSpace(event.Op)
	if op == "" {
		op = "unknown"
	}
	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = "success"
	}
	errMsg := strings.TrimSpace(event.ErrorMessage)
	record := &model.TaskRunEvent{
		TaskRunID:    s.task,
		JobID:        s.job,
		Kind:         "meta",
		Op:           op,
		Status:       status,
		SourcePath:   strings.TrimSpace(event.SourcePath),
		TargetPath:   strings.TrimSpace(event.TargetPath),
		ErrorMessage: errMsg,
		CreatedAt:    s.now(),
	}
	_ = s.repo.Create(ctx, record)
	s.logEvent("meta", op, status, record.SourcePath, record.TargetPath, errMsg)
}
