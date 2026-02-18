// Package worker 提供 Worker 执行器实现
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"github.com/strmsync/strmsync/internal/strmwriter"
	"github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/queue"
	"github.com/strmsync/strmsync/internal/pkg/logger"
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

	// 3. 构建 Driver 和 Writer
	driver, err := e.cfg.DriverFactory.Build(ctx, server)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build driver: %w", err))
	}

	writer, err := e.cfg.WriterFactory.Build(ctx, job)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build writer: %w", err))
	}

	// 4. 构建 EngineOptions
	engineOpts, err := buildEngineOptions(job)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("build engine options: %w", err))
	}

	// 5. 创建 Engine 实例
	engine, err := syncengine.NewEngine(driver, writer, e.log.With(
		zap.Uint("job_id", job.ID),
		zap.Uint("task_id", task.ID),
		zap.String("driver_type", driver.Type().String()),
	), engineOpts)
	if err != nil {
		return syncengine.SyncStats{}, permanentTaskError(fmt.Errorf("new engine: %w", err))
	}

	// 6. 执行 Engine.RunOnce
	stats, runErr := engine.RunOnce(ctx, job.SourcePath)

	// 7. 更新 TaskRun 进度
	if updateErr := e.cfg.TaskRuns.UpdateProgress(ctx, task.ID, progressFromStats(stats)); updateErr != nil {
		e.log.Warn("update task progress failed",
			zap.Uint("task_id", task.ID),
			zap.Error(updateErr))
	}

	if runErr != nil {
		return stats, wrapTaskError(runErr)
	}

	return stats, nil
}

// buildEngineOptions 将 Job 配置转换为 EngineOptions
//
// 从 Job.Options (JSON) 解析可选配置：
// - MaxConcurrency: 最大并发数
// - FileExtensions: 文件扩展名过滤
// - DryRun: 干运行模式
// - ForceUpdate: 强制更新
// - SkipExisting: 跳过已存在文件
// - ModTimeEpsilonSeconds: ModTime 容差（秒）
// - EnableOrphanCleanup: 启用孤儿文件清理
// - OrphanCleanupDryRun: 孤儿清理干运行模式
func buildEngineOptions(job model.Job) (syncengine.EngineOptions, error) {
	if strings.TrimSpace(job.TargetPath) == "" {
		return syncengine.EngineOptions{}, fmt.Errorf("job %d target_path is empty", job.ID)
	}

	opts := syncengine.EngineOptions{
		OutputRoot: job.TargetPath,
	}

	// 解析 Options JSON
	var extra jobOptions
	if strings.TrimSpace(job.Options) != "" {
		if err := json.Unmarshal([]byte(job.Options), &extra); err != nil {
			return syncengine.EngineOptions{}, fmt.Errorf("parse job options: %w", err)
		}
	}

	// 应用可选配置
	if extra.MaxConcurrency > 0 {
		opts.MaxConcurrency = extra.MaxConcurrency
	}
	if len(extra.FileExtensions) > 0 {
		opts.FileExtensions = extra.FileExtensions
	}
	opts.DryRun = extra.DryRun
	opts.ForceUpdate = extra.ForceUpdate
	opts.SkipExisting = extra.SkipExisting
	opts.EnableOrphanCleanup = extra.EnableOrphanCleanup
	opts.OrphanCleanupDryRun = extra.OrphanCleanupDryRun
	if extra.ModTimeEpsilonSeconds > 0 {
		opts.ModTimeEpsilon = time.Duration(extra.ModTimeEpsilonSeconds) * time.Second
	}

	return opts, nil
}

// jobOptions 表示从 Job.Options 解析的引擎参数
type jobOptions struct {
	MaxConcurrency        int      `json:"max_concurrency"`
	FileExtensions        []string `json:"file_extensions"`
	DryRun                bool     `json:"dry_run"`
	ForceUpdate           bool     `json:"force_update"`
	SkipExisting          bool     `json:"skip_existing"`
	ModTimeEpsilonSeconds int      `json:"mod_time_epsilon_seconds"`
	EnableOrphanCleanup   bool     `json:"enable_orphan_cleanup"`
	OrphanCleanupDryRun   bool     `json:"orphan_cleanup_dry_run"`
}

// progressFromStats 生成 TaskRunProgress
//
// 计算进度百分比：
// - 如果有总文件数，按 ProcessedFiles / TotalFiles 计算
// - 如果没有总文件数但已完成，进度为 100
func progressFromStats(stats syncengine.SyncStats) TaskRunProgress {
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
		TotalFiles:     total,
		ProcessedFiles: processed,
		FailedFiles:    failed,
		Progress:       progress,
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
//
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

	return filesystem.Config{
		Type:      serverType,
		BaseURL:   baseURL,
		Username:  opts.Username,
		Password:  password,
		STRMMode:  strmMode,
		MountPath: opts.MountPath,
		Timeout:   timeout,
	}, nil
}

// dataServerOptions 表示 DataServer.Options 的可选字段
type dataServerOptions struct {
	BaseURL        string `json:"base_url"`
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
		"total_files":     progress.TotalFiles,
		"processed_files": progress.ProcessedFiles,
		"failed_files":    progress.FailedFiles,
		"progress":        progress.Progress,
	}
	return r.db.WithContext(ctx).Model(&model.TaskRun{}).
		Where("id = ?", taskID).
		Updates(updates).Error
}
