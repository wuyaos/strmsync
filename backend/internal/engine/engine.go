// Package syncengine 提供 STRM 同步引擎实现
package syncengine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"
)

// Engine STRM 同步引擎
//
// 设计要点：
// - 驱动器抽象：通过 Driver 接口支持多种数据源
// - 写入器抽象：通过 Writer 接口支持多种写入目标
// - 并发控制：使用 conc/pool 控制并发数
// - 统计收集：记录同步过程的统计信息
// - 可取消：支持 context 取消同步任务
//
// 使用示例：
//
//	engine := NewEngine(driver, writer, logger, EngineOptions{
//	    MaxConcurrency: 10,
//	    OutputRoot: "/mnt/strm",
//	})
//	stats, err := engine.RunOnce(ctx, "/media/movies")
type Engine struct {
	driver Driver        // 数据源驱动（CloudDrive2/OpenList/Local）
	writer Writer        // STRM 文件写入器
	logger *zap.Logger   // 日志记录器
	opts   EngineOptions // 引擎配置选项

	// 运行状态
	running atomic.Bool
}

func (e *Engine) emitStrmEvent(ctx context.Context, event StrmEvent) {
	if e == nil || e.opts.EventSink == nil {
		return
	}
	e.opts.EventSink.OnStrmEvent(ctx, event)
}

// Writer STRM 文件写入器接口
//
// 注意：此接口是 syncengine 内部定义，与 strmwriter.StrmWriter 功能相同
// 未来可以统一，当前为了避免循环依赖而分开定义
//
// 重要约束：Writer 实现必须对应本地文件系统，以便进行本地读写。
// 如需支持非本地 Writer（如 S3、远程文件系统），需要扩展 Writer 接口
// 或引入独立的读写适配层。
type Writer interface {
	// Read 读取 STRM 文件内容
	Read(ctx context.Context, path string) (string, error)

	// Write 写入 STRM 文件
	Write(ctx context.Context, path string, content string, modTime time.Time) error

	// Delete 删除 STRM 文件
	Delete(ctx context.Context, path string) error
}

// NewEngine 创建同步引擎
//
// 参数：
//   - driver: 数据源驱动（不能为 nil）
//   - writer: STRM 写入器（不能为 nil）
//   - logger: 日志记录器（不能为 nil）
//   - opts: 引擎配置选项
//
// 返回：
//   - *Engine: 引擎实例
//   - error: 参数无效时返回错误
func NewEngine(driver Driver, writer Writer, logger *zap.Logger, opts EngineOptions) (*Engine, error) {
	if driver == nil {
		return nil, fmt.Errorf("syncengine: driver 不能为 nil: %w", ErrInvalidInput)
	}
	if writer == nil {
		return nil, fmt.Errorf("syncengine: writer 不能为 nil: %w", ErrInvalidInput)
	}
	if logger == nil {
		return nil, fmt.Errorf("syncengine: logger 不能为 nil: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(opts.OutputRoot) == "" {
		return nil, fmt.Errorf("syncengine: OutputRoot 不能为空: %w", ErrInvalidInput)
	}

	// 设置默认值
	if opts.MaxConcurrency <= 0 {
		opts.MaxConcurrency = 10
	}
	return &Engine{
		driver: driver,
		writer: writer,
		logger: logger,
		opts:   opts,
	}, nil
}

// RunOnce 执行一次完整的同步流程
//
// 工作流程：
//  1. 扫描远程文件流（使用 Driver.Scan）
//  2. 过滤文件（根据扩展名/排除目录/最小大小）
//  3. 使用 conc/pool 并发处理每个文件
//  4. 收集统计信息并返回
//
// 参数：
//   - ctx: 上下文，用于取消
//   - remotePath: 远程路径（数据源的根路径）
//
// 返回：
//   - SyncStats: 同步统计信息
//   - error: 同步失败时返回错误（部分文件失败不返回错误，记录在 SyncStats.Errors 中）
func (e *Engine) RunOnce(ctx context.Context, remotePath string) (SyncStats, error) {
	// 防止并发执行
	if !e.running.CompareAndSwap(false, true) {
		return SyncStats{}, fmt.Errorf("syncengine: 引擎正在运行中")
	}
	defer e.running.Store(false)
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stats := SyncStats{
		StartTime: time.Now(),
	}

	e.logger.Info("开始同步任务",
		zap.String("remote_root", remotePath),
		zap.String("output_root", e.opts.OutputRoot),
		zap.Int("max_concurrency", e.opts.MaxConcurrency),
		zap.Bool("dry_run", e.opts.DryRun))

	// 步骤1: 扫描远程文件流
	opt := ListOptions{
		Recursive: true,
		MaxDepth:  25,
	}
	var entryCh <-chan RemoteEntry
	var scanErrCh <-chan error
	if e.opts.ScanOverride != nil {
		entryCh, scanErrCh = e.opts.ScanOverride(ctx, remotePath, opt)
	} else {
		entryCh, scanErrCh = e.driver.Scan(ctx, remotePath, opt)
	}

	// 步骤2: 并发处理文件（流式+池化）
	p := pool.New().WithMaxGoroutines(e.opts.MaxConcurrency).WithContext(ctx)
	var mu sync.Mutex
	appendError := func(path string, err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if len(stats.Errors) >= 100 {
			return
		}
		stats.Errors = append(stats.Errors, SyncError{
			FilePath: path,
			Error:    err.Error(),
			Time:     time.Now(),
		})
	}

	var orphanIndex map[string]struct{}
	if e.opts.EnableOrphanCleanup {
		orphanIndex = make(map[string]struct{}, 1024)
	}

	for entryCh != nil || scanErrCh != nil {
		select {
		case entry, ok := <-entryCh:
			if !ok {
				entryCh = nil
				continue
			}

			if entry.IsDir {
				atomic.AddInt64(&stats.TotalDirs, 1)
				continue
			}
			atomic.AddInt64(&stats.TotalFiles, 1)

			// 排除目录过滤
			if IsExcludedPath(remotePath, entry.Path, e.opts.ExcludeDirs) {
				atomic.AddInt64(&stats.FilteredFiles, 1)
				continue
			}

			// 扩展名过滤
			if len(e.opts.FileExtensions) > 0 {
				matched := false
				for _, ext := range e.opts.FileExtensions {
					if strings.HasSuffix(strings.ToLower(entry.Name), strings.ToLower(ext)) {
						matched = true
						break
					}
				}
				if !matched {
					atomic.AddInt64(&stats.FilteredFiles, 1)
					continue
				}
			}

			if e.opts.MinFileSize > 0 && entry.Size > 0 && entry.Size < e.opts.MinFileSize {
				atomic.AddInt64(&stats.FilteredFiles, 1)
				continue
			}

			if orphanIndex != nil {
				outputPath, err := e.calculateOutputPath(entry.Path)
				if err != nil {
					appendError(entry.Path, fmt.Errorf("计算输出路径失败: %w", err))
				} else {
					rel, err := filepath.Rel(e.opts.OutputRoot, outputPath)
					if err != nil {
						appendError(outputPath, fmt.Errorf("计算相对路径失败: %w", err))
					} else if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
						rel = filepath.ToSlash(rel)
						orphanIndex[rel] = struct{}{}
					}
				}
			}

			entryCopy := entry
			p.Go(func(ctx context.Context) error {
				if err := e.processFile(ctx, entryCopy, &stats); err != nil {
					atomic.AddInt64(&stats.FailedFiles, 1)
					appendError(entryCopy.Path, err)
					e.logger.Warn("处理文件失败",
						zap.String("path", entryCopy.Path),
						zap.Error(err))
				} else {
					atomic.AddInt64(&stats.ProcessedFiles, 1)
				}
				return nil
			})
		case err, ok := <-scanErrCh:
			if !ok {
				scanErrCh = nil
				continue
			}
			if err != nil {
				return stats, fmt.Errorf("扫描远程文件失败: %w", err)
			}
		case <-ctx.Done():
			return stats, ctx.Err()
		}
	}

	if err := p.Wait(); err != nil {
		return stats, err
	}

	// 步骤4: 孤儿文件清理（可选）
	if e.opts.EnableOrphanCleanup {
		e.logger.Info("开始清理孤儿文件")
		if err := e.CleanOrphans(ctx, orphanIndex, &stats); err != nil {
			e.logger.Warn("清理孤儿文件失败",
				zap.Error(err))
		} else {
			e.logger.Info("孤儿文件清理完成",
				zap.Int64("deleted", stats.DeletedOrphans))
		}
	}

	// 步骤5: 统计完成
	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	e.logger.Debug("同步任务完成",
		zap.String("result", "同步任务完成"),
		zap.String("source", "同步引擎.任务完成"),
		zap.Int64("processed", stats.ProcessedFiles),
		zap.Int64("created", stats.CreatedFiles),
		zap.Int64("updated", stats.UpdatedFiles),
		zap.Int64("skipped", stats.SkippedFiles),
		zap.Int64("skipped_unchanged", stats.SkippedUnchanged),
		zap.Int64("failed", stats.FailedFiles),
		zap.Int64("deleted_orphans", stats.DeletedOrphans),
		zap.Duration("duration", stats.Duration))

	return stats, nil
}

// RunIncremental 仅处理特定事件对应的文件（增量同步）
//
// 工作流程：
// 1. 处理删除事件（计算输出路径并删除）
// 2. 过滤新增/更新事件（扩展名过滤）
// 3. 并发处理文件（复用 processFile）
//
// 参数：
//   - ctx: 上下文，用于取消
//   - events: 文件变更事件列表
//
// 返回：
//   - SyncStats: 同步统计信息
//   - error: 同步失败时返回错误（部分文件失败不返回错误，记录在 SyncStats.Errors 中）
func (e *Engine) RunIncremental(ctx context.Context, events []EngineEvent) (SyncStats, error) {
	// 防止并发执行
	if !e.running.CompareAndSwap(false, true) {
		return SyncStats{}, fmt.Errorf("syncengine: 引擎正在运行中")
	}
	defer e.running.Store(false)
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stats := SyncStats{
		StartTime: time.Now(),
	}

	e.logger.Info("开始增量同步任务",
		zap.Int("event_count", len(events)),
		zap.String("output_root", e.opts.OutputRoot),
		zap.Int("max_concurrency", e.opts.MaxConcurrency),
		zap.Bool("dry_run", e.opts.DryRun))

	if len(events) == 0 {
		stats.EndTime = time.Now()
		stats.Duration = stats.EndTime.Sub(stats.StartTime)
		return stats, nil
	}

	// appendError 添加错误到 stats（限制最多100个）
	appendError := func(path string, err error) {
		if len(stats.Errors) >= 100 {
			return
		}
		stats.Errors = append(stats.Errors, SyncError{
			FilePath: path,
			Error:    err.Error(),
			Time:     time.Now(),
		})
	}

	// resolveEventPath 获取事件的有效路径
	resolveEventPath := func(event EngineEvent) (string, error) {
		if strings.TrimSpace(event.AbsPath) != "" {
			return event.AbsPath, nil
		}
		if strings.TrimSpace(event.RelPath) != "" {
			return event.RelPath, nil
		}
		return "", fmt.Errorf("事件路径为空")
	}

	// removeEmptyParents 尝试删除空父目录（仅限 OutputRoot 之下）
	// 注意：这里直接使用 os.Remove 而不是 writer.Delete，因为 writer.Delete 专用于文件
	removeEmptyParents := func(outputPath string) {
		rootAbs, err := filepath.Abs(e.opts.OutputRoot)
		if err != nil {
			return
		}
		dir := filepath.Dir(outputPath)
		for {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return
			}
			// 已到达或超出 OutputRoot，停止
			rel, err := filepath.Rel(rootAbs, absDir)
			if err != nil {
				return
			}
			if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return
			}
			// 尝试删除目录（仅当为空时成功）
			// 如果目录非空、不存在或其他错误，均停止删除
			if err := os.Remove(dir); err != nil {
				return
			}
			// 成功删除，继续向上
			dir = filepath.Dir(dir)
		}
	}

	// 步骤1: 处理删除事件
	for _, event := range events {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		switch event.Type {
		case DriverEventDelete:
			if event.IsDir {
				atomic.AddInt64(&stats.TotalDirs, 1)
				continue
			}
			path, err := resolveEventPath(event)
			if err != nil {
				atomic.AddInt64(&stats.FailedFiles, 1)
				appendError("", err)
				continue
			}

			// 扩展名过滤（避免删除不相关文件）
			if len(e.opts.FileExtensions) > 0 {
				name := filepath.Base(path)
				matched := false
				for _, ext := range e.opts.FileExtensions {
					if strings.HasSuffix(strings.ToLower(name), strings.ToLower(ext)) {
						matched = true
						break
					}
				}
				if !matched {
					atomic.AddInt64(&stats.FilteredFiles, 1)
					continue
				}
			}

			outputPath, err := e.calculateOutputPath(path)
			if err != nil {
				atomic.AddInt64(&stats.FailedFiles, 1)
				appendError(path, fmt.Errorf("计算输出路径失败: %w", err))
				continue
			}

			// Dry Run 模式 - 只统计，不实际删除
			if e.opts.DryRun {
				e.logger.Debug("Dry Run: 删除 STRM 文件",
					zap.String("output_path", outputPath))
				atomic.AddInt64(&stats.DeletedOrphans, 1)
				e.emitStrmEvent(ctx, StrmEvent{
					Op:           "delete",
					Status:       "skipped",
					SourcePath:   path,
					TargetPath:   outputPath,
					ErrorMessage: "dry_run",
				})
				continue
			}

			if err := e.writer.Delete(ctx, outputPath); err != nil && !isNotExist(err) {
				atomic.AddInt64(&stats.FailedFiles, 1)
				appendError(path, fmt.Errorf("删除 STRM 文件失败: %w", err))
				e.logger.Warn("删除 STRM 文件失败",
					zap.String("output_path", outputPath),
					zap.Error(err))
				e.emitStrmEvent(ctx, StrmEvent{
					Op:           "delete",
					Status:       "failed",
					SourcePath:   path,
					TargetPath:   outputPath,
					ErrorMessage: err.Error(),
				})
				continue
			}

			atomic.AddInt64(&stats.DeletedOrphans, 1)
			e.logger.Debug("删除 STRM 文件",
				zap.String("output_path", outputPath))
			e.emitStrmEvent(ctx, StrmEvent{
				Op:         "delete",
				Status:     "success",
				SourcePath: path,
				TargetPath: outputPath,
			})
			removeEmptyParents(outputPath)
		case DriverEventCreate, DriverEventUpdate:
			// 交由后续处理
			continue
		default:
			atomic.AddInt64(&stats.FailedFiles, 1)
			appendError("", fmt.Errorf("未处理的事件类型: %s", event.Type.String()))
		}
	}

	// 步骤2: 收集新增/更新事件并转换为 RemoteEntry
	entries := make([]RemoteEntry, 0, len(events))
	for _, event := range events {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		switch event.Type {
		case DriverEventCreate, DriverEventUpdate:
			if event.IsDir {
				atomic.AddInt64(&stats.TotalDirs, 1)
				continue
			}
			path, err := resolveEventPath(event)
			if err != nil {
				atomic.AddInt64(&stats.FailedFiles, 1)
				appendError("", err)
				continue
			}
			entries = append(entries, RemoteEntry{
				Path:    path,
				Name:    filepath.Base(path),
				Size:    event.Size,
				ModTime: event.ModTime,
				IsDir:   false,
			})
			atomic.AddInt64(&stats.TotalFiles, 1)
		case DriverEventDelete:
			continue
		default:
			atomic.AddInt64(&stats.FailedFiles, 1)
			appendError("", fmt.Errorf("未处理的事件类型: %s", event.Type.String()))
		}
	}

	// 步骤3: 过滤并并发处理文件
	p := pool.New().WithMaxGoroutines(e.opts.MaxConcurrency).WithContext(ctx)
	var mu sync.Mutex
	appendError = func(path string, err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if len(stats.Errors) >= 100 {
			return
		}
		stats.Errors = append(stats.Errors, SyncError{
			FilePath: path,
			Error:    err.Error(),
			Time:     time.Now(),
		})
	}

	for _, entry := range entries {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}

		// 排除目录过滤（增量模式不依赖远端根路径）
		if IsExcludedPath("/", entry.Path, e.opts.ExcludeDirs) {
			atomic.AddInt64(&stats.FilteredFiles, 1)
			continue
		}

		// 扩展名过滤
		if len(e.opts.FileExtensions) > 0 {
			matched := false
			for _, ext := range e.opts.FileExtensions {
				if strings.HasSuffix(strings.ToLower(entry.Name), strings.ToLower(ext)) {
					matched = true
					break
				}
			}
			if !matched {
				atomic.AddInt64(&stats.FilteredFiles, 1)
				continue
			}
		}

		if e.opts.MinFileSize > 0 && entry.Size > 0 && entry.Size < e.opts.MinFileSize {
			atomic.AddInt64(&stats.FilteredFiles, 1)
			continue
		}

		entryCopy := entry
		p.Go(func(ctx context.Context) error {
			if err := e.processFile(ctx, entryCopy, &stats); err != nil {
				atomic.AddInt64(&stats.FailedFiles, 1)
				appendError(entryCopy.Path, err)
				e.logger.Warn("处理文件失败",
					zap.String("path", entryCopy.Path),
					zap.Error(err))
			} else {
				atomic.AddInt64(&stats.ProcessedFiles, 1)
			}
			return nil
		})
	}

	if err := p.Wait(); err != nil {
		return stats, fmt.Errorf("处理文件失败: %w", err)
	}

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	e.logger.Info("增量同步任务完成",
		zap.Int64("processed", stats.ProcessedFiles),
		zap.Int64("created", stats.CreatedFiles),
		zap.Int64("updated", stats.UpdatedFiles),
		zap.Int64("skipped", stats.SkippedFiles),
		zap.Int64("skipped_unchanged", stats.SkippedUnchanged),
		zap.Int64("failed", stats.FailedFiles),
		zap.Int64("deleted_orphans", stats.DeletedOrphans),
		zap.Duration("duration", stats.Duration))

	return stats, nil
}

// applyMountPathMapping 应用挂载路径映射（系统级基线转换）
// 在用户替换规则之前执行，确保路径统一
func applyMountPathMapping(input string, mapping *MountPathMapping) string {
	if mapping == nil || mapping.From == "" {
		return input
	}
	if strings.HasPrefix(input, mapping.From) {
		return mapping.To + strings.TrimPrefix(input, mapping.From)
	}
	return input
}

// applyStrmReplaceRules 应用用户自定义替换规则
// 在挂载路径映射之后执行
func applyStrmReplaceRules(input string, rules []StrmReplaceRule) string {
	if len(rules) == 0 {
		return input
	}
	output := input
	for _, rule := range rules {
		if rule.From == "" {
			continue
		}
		if strings.HasPrefix(output, rule.From) {
			output = rule.To + strings.TrimPrefix(output, rule.From)
		}
	}
	return output
}

// processFile 处理单个文件
//
// 工作流程：
// 1. 构建 STRM 内容（BuildStrmInfo）
// 2. 计算输出文件路径
// 3. Dry Run 检查
// 4. 获取本地文件存在性
// 5. SkipExisting 检查
// 6. 读取现有文件文本并比对
// 7. 写入或更新文件
//
// 参数：
//   - ctx: 上下文
//   - entry: 远程文件条目
//   - stats: 统计信息（原子更新）
//
// 返回：
//   - error: 处理失败时返回错误
func (e *Engine) processFile(ctx context.Context, entry RemoteEntry, stats *SyncStats) error {
	// 步骤1: 构建 STRM 内容
	strmInfo, err := e.driver.BuildStrmInfo(ctx, BuildStrmRequest{
		ServerID:   0, // TODO: 从配置获取
		RemotePath: entry.Path,
		RemoteMeta: &entry,
	})
	if err != nil {
		return fmt.Errorf("构建 STRM 信息失败: %w", err)
	}

	// 步骤1.1: 应用挂载路径映射（系统级基线转换）
	rawURL := applyMountPathMapping(strmInfo.RawURL, e.opts.MountPathMapping)

	// 步骤1.2: 应用用户自定义替换规则
	expectedContent := applyStrmReplaceRules(rawURL, e.opts.StrmReplaceRules)

	// 步骤2: 计算输出文件路径
	outputPath, err := e.calculateOutputPath(entry.Path)
	if err != nil {
		return fmt.Errorf("计算输出路径失败: %w", err)
	}

	// 步骤3: Dry Run 模式 - 只统计，不实际写入
	if e.opts.DryRun {
		e.logger.Debug("Dry Run: 跳过写入",
			zap.String("remote_file_path", entry.Path),
			zap.String("output_path", outputPath))
		atomic.AddInt64(&stats.SkippedFiles, 1)
		e.emitStrmEvent(ctx, StrmEvent{
			Op:           "skip",
			Status:       "skipped",
			SourcePath:   entry.Path,
			TargetPath:   outputPath,
			ErrorMessage: "dry_run",
		})
		return nil
	}

	// 步骤4: 获取本地文件存在性
	localExists := false
	if _, err := os.Stat(outputPath); err == nil {
		localExists = true
	} else if !isNotExist(err) {
		return fmt.Errorf("获取本地文件信息失败: %w", err)
	}

	// 步骤5: SkipExisting 检查
	if e.opts.SkipExisting && localExists {
		atomic.AddInt64(&stats.SkippedFiles, 1)
		e.emitStrmEvent(ctx, StrmEvent{
			Op:           "skip",
			Status:       "skipped",
			SourcePath:   entry.Path,
			TargetPath:   outputPath,
			ErrorMessage: "skip_existing",
		})
		return nil
	}

	// 步骤6: 读取现有文件文本并比对
	if localExists && !e.opts.ForceUpdate {
		existingContent, err := e.writer.Read(ctx, outputPath)
		if err != nil {
			if !isNotExist(err) {
				return fmt.Errorf("读取现有文件失败: %w", err)
			}
		} else if strings.Compare(strings.TrimSpace(existingContent), strings.TrimSpace(expectedContent)) == 0 {
			atomic.AddInt64(&stats.SkippedFiles, 1)
			atomic.AddInt64(&stats.SkippedUnchanged, 1)
			e.logger.Debug("内容相同，跳过更新",
				zap.String("path", outputPath))
			e.emitStrmEvent(ctx, StrmEvent{
				Op:           "skip",
				Status:       "skipped",
				SourcePath:   entry.Path,
				TargetPath:   outputPath,
				ErrorMessage: "content_equal",
			})
			return nil
		}
	}

	// 步骤7: 写入或更新文件
	op := "update"
	if !localExists {
		op = "create"
	}
	if err := e.writer.Write(ctx, outputPath, expectedContent, entry.ModTime); err != nil {
		e.emitStrmEvent(ctx, StrmEvent{
			Op:           op,
			Status:       "failed",
			SourcePath:   entry.Path,
			TargetPath:   outputPath,
			ErrorMessage: err.Error(),
		})
		return fmt.Errorf("写入 STRM 文件失败: %w", err)
	}

	// 更新统计信息
	if !localExists {
		atomic.AddInt64(&stats.CreatedFiles, 1)
		e.logger.Debug("创建 STRM 文件",
			zap.String("output_path", outputPath))
		e.emitStrmEvent(ctx, StrmEvent{
			Op:         "create",
			Status:     "success",
			SourcePath: entry.Path,
			TargetPath: outputPath,
		})
	} else {
		atomic.AddInt64(&stats.UpdatedFiles, 1)
		e.logger.Debug("更新 STRM 文件",
			zap.String("output_path", outputPath))
		e.emitStrmEvent(ctx, StrmEvent{
			Op:         "update",
			Status:     "success",
			SourcePath: entry.Path,
			TargetPath: outputPath,
		})
	}

	return nil
}

// calculateOutputPath 计算输出文件路径
//
// 规则：
// - 远程路径：/media/movies/folder/file.mp4
// - 输出路径：<OutputRoot>/media/movies/folder/file.strm
//
// 安全性：
// - 防止路径逃逸（使用 path.Clean 规范化）
// - 验证结果路径在 OutputRoot 之下
//
// 参数：
//   - remotePath: 远程文件路径（Unix 格式）
//
// 返回：
//   - string: 输出文件路径（本地路径格式，跨平台）
//   - error: 路径无效或逃逸时返回错误
func (e *Engine) calculateOutputPath(remotePath string) (string, error) {
	// 安全性：使用 path.Clean 规范化 Unix 路径
	cleanPath := filepath.ToSlash(remotePath) // 确保是 Unix 路径
	cleanPath = filepath.Clean("/" + cleanPath)
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	// 转换为本地路径格式
	cleanPath = filepath.FromSlash(cleanPath)

	// 替换扩展名为 .strm
	ext := filepath.Ext(cleanPath)
	if ext != "" {
		cleanPath = strings.TrimSuffix(cleanPath, ext) + ".strm"
	} else {
		cleanPath = cleanPath + ".strm"
	}

	// 使用 filepath.Join 拼接路径
	outputPath := filepath.Join(e.opts.OutputRoot, cleanPath)

	// 安全验证：确保结果路径在 OutputRoot 之下
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("无法解析输出路径: %w", err)
	}
	absRoot, err := filepath.Abs(e.opts.OutputRoot)
	if err != nil {
		return "", fmt.Errorf("无法解析根路径: %w", err)
	}

	rel, err := filepath.Rel(absRoot, absOutput)
	if err != nil {
		return "", fmt.Errorf("路径验证失败: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("路径逃逸检测: %s 逃逸了根目录 %s", remotePath, e.opts.OutputRoot)
	}

	return outputPath, nil
}

// CleanOrphans 清理本地孤儿 STRM 文件
//
// 此方法扫描本地输出目录下的所有 .strm 文件，
// 并根据远端快照索引判断哪些文件的源文件已不存在，
// 然后删除这些孤儿文件。
//
// 孤儿文件是指：本地存在 STRM 文件，但远程文件已被删除或移动。
// 清理孤儿文件可以保持本地 STRM 目录与远程文件系统的一致性。
//
// 安全特性：
// - 支持 DryRun 模式（只记录不删除）
// - 路径逃逸检测
// - 错误不中断整体流程（部分失败记录日志）
// - 只删除 .strm 文件（大小写不敏感）
//
// 参数：
//   - ctx: 上下文，用于取消
//   - remoteIndex: 远端文件的快照索引
//   - stats: 统计信息（原子更新）
//
// 返回：
//   - error: 清理失败时返回错误
func (e *Engine) CleanOrphans(ctx context.Context, remoteIndex map[string]struct{}, stats *SyncStats) error {
	if remoteIndex == nil {
		return fmt.Errorf("远端索引为空: %w", ErrInvalidInput)
	}

	dryRun := e.opts.DryRun || e.opts.OrphanCleanupDryRun
	var firstErr error

	// 使用 filepath.WalkDir 遍历输出目录
	walkErr := filepath.WalkDir(e.opts.OutputRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			wrapped := fmt.Errorf("访问路径失败: %w", err)
			if firstErr == nil {
				firstErr = wrapped
			}
			e.logger.Warn("扫描 STRM 目录失败",
				zap.String("path", path),
				zap.Error(wrapped))
			return nil // 继续处理其他文件
		}

		// 检查 context 取消
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 跳过目录
		if d.IsDir() {
			return nil
		}

		// 只处理 .strm 文件（大小写不敏感）
		if !strings.EqualFold(filepath.Ext(d.Name()), ".strm") {
			return nil
		}

		// 计算相对路径
		rel, err := filepath.Rel(e.opts.OutputRoot, path)
		if err != nil {
			wrapped := fmt.Errorf("计算相对路径失败: %w", err)
			if firstErr == nil {
				firstErr = wrapped
			}
			e.logger.Warn("扫描 STRM 目录失败",
				zap.String("path", path),
				zap.Error(wrapped))
			return nil
		}

		// 安全检查：防止路径逃逸
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			wrapped := fmt.Errorf("路径逃逸检测: %s 逃逸了根目录 %s", path, e.opts.OutputRoot)
			if firstErr == nil {
				firstErr = wrapped
			}
			e.logger.Warn("扫描 STRM 目录失败",
				zap.String("path", path),
				zap.Error(wrapped))
			return nil
		}

		// 统一使用 Unix 路径格式
		rel = filepath.ToSlash(rel)

		// 检查是否在远端索引中
		if _, ok := remoteIndex[rel]; ok {
			// 不是孤儿，跳过
			return nil
		}

		// 是孤儿文件
		if dryRun {
			e.logger.Debug("Dry Run: 删除孤儿 STRM 文件",
				zap.String("path", path))
			atomic.AddInt64(&stats.DeletedOrphans, 1)
			return nil
		}

		// 删除孤儿文件
		if err := e.writer.Delete(ctx, path); err != nil {
			wrapped := fmt.Errorf("删除孤儿文件失败: %w", err)
			if firstErr == nil {
				firstErr = wrapped
			}
			e.logger.Warn("删除孤儿文件失败",
				zap.String("path", path),
				zap.Error(wrapped))
			return nil
		}

		atomic.AddInt64(&stats.DeletedOrphans, 1)
		e.logger.Debug("删除孤儿 STRM 文件",
			zap.String("path", path))
		return nil
	})

	if walkErr != nil {
		return fmt.Errorf("清理孤儿文件失败: %w", walkErr)
	}
	if firstErr != nil {
		return fmt.Errorf("清理孤儿文件部分失败: %w", firstErr)
	}
	return nil
}

// isNotExist 判断错误是否表示文件不存在
//
// 支持：
// - os.ErrNotExist
// - 包含 "not exist" 的错误消息（用于非标准错误）
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	// 标准的文件不存在错误
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	// 一些写入器可能返回非标准错误，尝试从消息判断
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "not exist") ||
		strings.Contains(errMsg, "no such file")
}
