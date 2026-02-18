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

	"go.uber.org/zap"
)

// Engine STRM 同步引擎
//
// 设计要点：
// - 驱动器抽象：通过 Driver 接口支持多种数据源
// - 写入器抽象：通过 Writer 接口支持多种写入目标
// - 并发控制：使用 semaphore 限制并发数
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

// Writer STRM 文件写入器接口
//
// 注意：此接口是 syncengine 内部定义，与 strmwriter.StrmWriter 功能相同
// 未来可以统一，当前为了避免循环依赖而分开定义
//
// 重要约束：Writer 实现必须对应本地文件系统，因为 Engine 会使用 os.Stat
// 来获取文件的 ModTime 以进行增量更新判定。如果未来需要支持非本地 Writer
// （如 S3、远程文件系统），需要扩展 Writer 接口增加 Stat 方法。
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
	if opts.ModTimeEpsilon <= 0 {
		opts.ModTimeEpsilon = 2 * time.Second
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
// 1. 扫描远程文件列表（使用 Driver.List）
// 2. 过滤文件（根据扩展名）
// 3. 并发处理每个文件：
//    a. 构建 STRM 内容（使用 Driver.BuildStrmInfo）
//    b. 比对现有内容（使用 Driver.CompareStrm）
//    c. 写入/更新文件（使用 Writer）
// 4. 收集统计信息并返回
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

	stats := SyncStats{
		StartTime: time.Now(),
	}

	e.logger.Info("开始同步任务",
		zap.String("remote_path", remotePath),
		zap.String("output_root", e.opts.OutputRoot),
		zap.Int("max_concurrency", e.opts.MaxConcurrency),
		zap.Bool("dry_run", e.opts.DryRun))

	// 步骤1: 扫描远程文件列表
	entries, err := e.scanRemoteFiles(ctx, remotePath)
	if err != nil {
		return stats, fmt.Errorf("扫描远程文件失败: %w", err)
	}

	e.logger.Info("扫描完成",
		zap.Int64("total_files", stats.TotalFiles),
		zap.Int64("total_dirs", stats.TotalDirs))

	// 步骤2: 过滤文件
	files := e.filterFiles(entries, &stats)
	e.logger.Info("文件过滤完成",
		zap.Int("matched_files", len(files)),
		zap.Int64("filtered_files", stats.FilteredFiles))

	// 步骤3: 并发处理文件
	if err := e.processFiles(ctx, files, &stats); err != nil {
		return stats, fmt.Errorf("处理文件失败: %w", err)
	}

	// 步骤4: 孤儿文件清理（可选）
	if e.opts.EnableOrphanCleanup {
		e.logger.Info("开始清理孤儿文件")
		// 注意：使用过滤后的文件列表构建索引，确保扩展名过滤规则变化后能清理旧 STRM
		remoteIndex, idxErr := e.buildRemoteIndex(files)
		if idxErr != nil {
			e.logger.Warn("构建远端索引失败，跳过孤儿清理",
				zap.Error(idxErr))
		} else {
			if err := e.CleanOrphans(ctx, remoteIndex, &stats); err != nil {
				e.logger.Warn("清理孤儿文件失败",
					zap.Error(err))
			} else {
				e.logger.Info("孤儿文件清理完成",
					zap.Int64("deleted", stats.DeletedOrphans))
			}
		}
	}

	// 步骤5: 统计完成
	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	e.logger.Info("同步任务完成",
		zap.Int64("processed", stats.ProcessedFiles),
		zap.Int64("created", stats.CreatedFiles),
		zap.Int64("updated", stats.UpdatedFiles),
		zap.Int64("updated_by_modtime", stats.UpdatedByModTime),
		zap.Int64("skipped", stats.SkippedFiles),
		zap.Int64("skipped_unchanged", stats.SkippedUnchanged),
		zap.Int64("failed", stats.FailedFiles),
		zap.Int64("deleted_orphans", stats.DeletedOrphans),
		zap.Duration("duration", stats.Duration))

	return stats, nil
}

// scanRemoteFiles 扫描远程文件列表
func (e *Engine) scanRemoteFiles(ctx context.Context, remotePath string) ([]RemoteEntry, error) {
	entries, err := e.driver.List(ctx, remotePath, ListOptions{
		Recursive: true,
		MaxDepth:  100, // 默认最大深度100层，避免无限递归
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// filterFiles 根据配置过滤文件
func (e *Engine) filterFiles(entries []RemoteEntry, stats *SyncStats) []RemoteEntry {
	var files []RemoteEntry

	for _, entry := range entries {
		// 统计目录和文件
		if entry.IsDir {
			atomic.AddInt64(&stats.TotalDirs, 1)
			continue
		}
		atomic.AddInt64(&stats.TotalFiles, 1)

		// 检查扩展名过滤
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

		files = append(files, entry)
	}

	return files
}

// processFiles 并发处理文件列表
func (e *Engine) processFiles(ctx context.Context, files []RemoteEntry, stats *SyncStats) error {
	// 创建信号量控制并发数
	sem := make(chan struct{}, e.opts.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex // 保护 stats.Errors

	// 使用可取消的子 context，确保所有 goroutine 能收到取消信号
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, file := range files {
		// 检查 context 取消（主循环级别）
		if ctx.Err() != nil {
			cancel() // 通知所有 goroutine 停止
			break   // 退出循环，等待已启动的 goroutine
		}

		wg.Add(1)
		go func(entry RemoteEntry) {
			defer wg.Done()

			// 获取信号量（可能被 context 取消中断）
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return // Context 已取消，提前退出
			}

			// 再次检查 context（在获取信号量后）
			if ctx.Err() != nil {
				return
			}

			// 处理单个文件
			if err := e.processFile(ctx, entry, stats); err != nil {
				atomic.AddInt64(&stats.FailedFiles, 1)

				// 记录错误（限制最多100个）
				mu.Lock()
				if len(stats.Errors) < 100 {
					stats.Errors = append(stats.Errors, SyncError{
						FilePath: entry.Path,
						Error:    err.Error(),
						Time:     time.Now(),
					})
				}
				mu.Unlock()

				e.logger.Warn("处理文件失败",
					zap.String("path", entry.Path),
					zap.Error(err))
			} else {
				atomic.AddInt64(&stats.ProcessedFiles, 1)
			}
		}(file)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	// 检查是否因取消而退出
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// normalizeModTime 统一修改时间到 UTC 并按精度截断
//
// 不同文件系统和操作系统的时间精度不同（FAT: 2秒, NTFS: 100ns, ext4: 1ns），
// 此函数将时间统一到 UTC 并按指定精度截断，以便进行可靠的比较。
//
// 参数：
//   - t: 要规范化的时间
//   - precision: 精度（例如 time.Second）
//
// 返回：
//   - 规范化后的时间
func normalizeModTime(t time.Time, precision time.Duration) time.Time {
	if t.IsZero() {
		return t
	}
	if precision <= 0 {
		precision = time.Second
	}
	return t.UTC().Truncate(precision)
}

// modTimeDifferent 判断两个修改时间是否有显著差异
//
// 考虑时间精度和允许的误差阈值，判断两个时间是否应视为不同。
// 这避免了因时间漂移或精度差异导致的无意义更新。
//
// 参数：
//   - local: 本地文件的修改时间
//   - remote: 远程文件的修改时间
//   - epsilon: 允许的误差阈值
//
// 返回：
//   - true 表示时间有显著差异，false 表示时间相同或在阈值内
func modTimeDifferent(local time.Time, remote time.Time, epsilon time.Duration) bool {
	if local.IsZero() || remote.IsZero() {
		return false
	}
	if epsilon < 0 {
		epsilon = 0
	}

	// 规范化到相同精度
	local = normalizeModTime(local, time.Second)
	remote = normalizeModTime(remote, time.Second)

	// 计算绝对差值
	delta := remote.Sub(local)
	if delta < 0 {
		delta = -delta
	}

	return delta > epsilon
}

// DecideUpdate 根据内容和修改时间判定是否需要更新
//
// 这是更新决策的核心逻辑，根据多种因素（文件存在性、强制更新标志、
// 内容相等性、修改时间差异）综合判断是否需要更新文件。
//
// 决策优先级：
// 1. 文件不存在 → 创建（ChangeReasonNew）
// 2. 强制更新模式 → 更新（ChangeReasonForced）
// 3. 内容不同 → 更新（ChangeReasonContent）
// 4. 内容相同但 ModTime 超出阈值 → 更新（ChangeReasonModTime）
// 5. 内容和 ModTime 均未变化 → 跳过（ChangeReasonUnchanged）
//
// 参数：
//   - localExists: 本地文件是否存在
//   - localModTime: 本地文件的修改时间
//   - remoteModTime: 远程文件的修改时间
//   - contentEqual: 内容是否相等
//   - opts: 引擎配置选项
//
// 返回：
//   - ChangeDecision: 决策结果
//   - error: 决策失败时返回错误
func DecideUpdate(localExists bool, localModTime time.Time, remoteModTime time.Time, contentEqual bool, opts EngineOptions) (ChangeDecision, error) {
	// 文件不存在，需要创建
	if !localExists {
		return ChangeDecision{ShouldUpdate: true, Reason: ChangeReasonNew}, nil
	}

	// 强制更新模式
	if opts.ForceUpdate {
		return ChangeDecision{ShouldUpdate: true, Reason: ChangeReasonForced}, nil
	}

	// 内容不同，需要更新
	if !contentEqual {
		return ChangeDecision{ShouldUpdate: true, Reason: ChangeReasonContent}, nil
	}

	// 内容相同，检查 ModTime
	epsilon := opts.ModTimeEpsilon
	if epsilon <= 0 {
		epsilon = 2 * time.Second
	}

	if modTimeDifferent(localModTime, remoteModTime, epsilon) {
		// ModTime 超出阈值，需要更新时间戳
		return ChangeDecision{ShouldUpdate: true, Reason: ChangeReasonModTime}, nil
	}

	// 内容和 ModTime 均未变化，跳过
	return ChangeDecision{ShouldUpdate: false, Reason: ChangeReasonUnchanged}, nil
}

// processFile 处理单个文件
//
// 工作流程：
// 1. 构建 STRM 内容（BuildStrmInfo）
// 2. 计算输出文件路径
// 3. Dry Run 检查
// 4. 获取本地文件元信息（存在性 + ModTime）
// 5. SkipExisting 检查
// 6. 读取现有文件内容并比对
// 7. 使用 DecideUpdate 判定是否更新
// 8. 写入或更新文件
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

	// 步骤2: 计算输出文件路径
	outputPath, err := e.calculateOutputPath(entry.Path)
	if err != nil {
		return fmt.Errorf("计算输出路径失败: %w", err)
	}

	// 步骤3: Dry Run 模式 - 只统计，不实际写入
	if e.opts.DryRun {
		e.logger.Debug("Dry Run: 跳过写入",
			zap.String("remote_path", entry.Path),
			zap.String("output_path", outputPath))
		atomic.AddInt64(&stats.SkippedFiles, 1)
		return nil
	}

	// 步骤4: 获取本地文件元信息（存在性 + ModTime）
	// 注意：这里直接使用 os.Stat 是基于 Writer 对应本地文件系统的假设
	// 这样可以获取精确的 ModTime 用于增量更新判定
	localExists := false
	localModTime := time.Time{}
	if info, err := os.Stat(outputPath); err == nil {
		localExists = true
		localModTime = info.ModTime()
	} else if !isNotExist(err) {
		return fmt.Errorf("获取本地文件信息失败: %w", err)
	}

	// 步骤5: 检查是否跳过已存在文件
	if e.opts.SkipExisting && localExists {
		existingContent, err := e.writer.Read(ctx, outputPath)
		if err == nil && strings.TrimSpace(existingContent) != "" {
			e.logger.Debug("跳过已存在文件",
				zap.String("output_path", outputPath))
			atomic.AddInt64(&stats.SkippedFiles, 1)
			return nil
		}
		if err != nil && !isNotExist(err) {
			return fmt.Errorf("读取现有文件失败: %w", err)
		}
	}

	// 步骤6: 读取现有文件内容并比对
	contentEqual := false
	if !e.opts.ForceUpdate && localExists {
		existingContent, err := e.writer.Read(ctx, outputPath)
		if err != nil {
			if isNotExist(err) {
				// 文件在 Stat 和 Read 之间被删除
				localExists = false
				localModTime = time.Time{}
			} else {
				return fmt.Errorf("读取现有文件失败: %w", err)
			}
		} else {
			// 比对内容
			compareResult, compareErr := e.driver.CompareStrm(ctx, CompareInput{
				Expected:  strmInfo,
				ActualRaw: existingContent,
			})
			if compareErr != nil {
				e.logger.Warn("比对 STRM 内容失败，将更新",
					zap.String("path", outputPath),
					zap.Error(compareErr))
			} else if compareResult.Equal {
				contentEqual = true
				e.logger.Debug("内容相同",
					zap.String("path", outputPath),
					zap.String("reason", compareResult.Reason))
			} else {
				e.logger.Debug("内容不同，需要更新",
					zap.String("path", outputPath),
					zap.String("reason", compareResult.Reason))
			}
		}
	}

	// 步骤7: 使用 DecideUpdate 判定是否更新
	decision, err := DecideUpdate(localExists, localModTime, entry.ModTime, contentEqual, e.opts)
	if err != nil {
		return fmt.Errorf("判定更新策略失败: %w", err)
	}

	if !decision.ShouldUpdate {
		atomic.AddInt64(&stats.SkippedFiles, 1)
		if decision.Reason == ChangeReasonUnchanged {
			atomic.AddInt64(&stats.SkippedUnchanged, 1)
		}
		e.logger.Debug("跳过更新",
			zap.String("path", outputPath),
			zap.String("reason", decision.Reason.String()))
		return nil
	}

	// 步骤8: 写入或更新文件
	if err := e.writer.Write(ctx, outputPath, strmInfo.RawURL, entry.ModTime); err != nil {
		return fmt.Errorf("写入 STRM 文件失败: %w", err)
	}

	// 更新统计信息
	if !localExists {
		atomic.AddInt64(&stats.CreatedFiles, 1)
		e.logger.Debug("创建 STRM 文件",
			zap.String("output_path", outputPath))
	} else {
		atomic.AddInt64(&stats.UpdatedFiles, 1)
		if decision.Reason == ChangeReasonModTime {
			atomic.AddInt64(&stats.UpdatedByModTime, 1)
		}
		e.logger.Debug("更新 STRM 文件",
			zap.String("output_path", outputPath),
			zap.String("reason", decision.Reason.String()))
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

// buildRemoteIndex 构建远端快照索引
//
// 此方法遍历所有远端文件，为每个文件计算对应的本地输出路径，
// 并构建一个快照索引（map）。此索引用于快速判断哪些本地 STRM 文件是孤儿。
//
// 使用基于输出相对路径的索引比逐个调用 Driver.Stat 高效得多，
// 特别是在处理大量文件时避免了大量的远程 API 调用。
//
// 参数：
//   - entries: 远端文件列表
//
// 返回：
//   - map[string]struct{}: 远端文件的输出相对路径集合
//   - error: 构建失败时返回第一个遇到的错误（部分失败不中断整体）
func (e *Engine) buildRemoteIndex(entries []RemoteEntry) (map[string]struct{}, error) {
	index := make(map[string]struct{}, len(entries))
	var firstErr error

	for _, entry := range entries {
		// 跳过目录
		if entry.IsDir {
			continue
		}

		// 计算输出路径
		outputPath, err := e.calculateOutputPath(entry.Path)
		if err != nil {
			wrapped := fmt.Errorf("计算输出路径失败: %w", err)
			if firstErr == nil {
				firstErr = wrapped
			}
			e.logger.Warn("构建远端索引失败",
				zap.String("remote_path", entry.Path),
				zap.Error(wrapped))
			continue
		}

		// 计算相对路径
		rel, err := filepath.Rel(e.opts.OutputRoot, outputPath)
		if err != nil {
			wrapped := fmt.Errorf("计算相对路径失败: %w", err)
			if firstErr == nil {
				firstErr = wrapped
			}
			e.logger.Warn("构建远端索引失败",
				zap.String("output_path", outputPath),
				zap.Error(wrapped))
			continue
		}

		// 安全检查：防止路径逃逸
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			wrapped := fmt.Errorf("路径逃逸检测: %s 逃逸了根目录 %s", outputPath, e.opts.OutputRoot)
			if firstErr == nil {
				firstErr = wrapped
			}
			e.logger.Warn("构建远端索引失败",
				zap.String("output_path", outputPath),
				zap.Error(wrapped))
			continue
		}

		// 统一使用 Unix 路径格式作为索引键
		rel = filepath.ToSlash(rel)
		index[rel] = struct{}{}
	}

	return index, firstErr
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

