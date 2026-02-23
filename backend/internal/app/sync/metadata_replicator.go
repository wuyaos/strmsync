// Package sync 实现元数据文件复制器
package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"go.uber.org/zap"
)

// MetadataReplicator 元数据文件复制器实现
type MetadataReplicator struct {
	fs         filesystem.Client
	targetRoot string
	logger     *zap.Logger
	// 策略配置
	preferMount bool // true=优先挂载路径复制，false=优先API下载
	eventSink   MetaEventSink
}

// MetaEvent 元数据处理事件
type MetaEvent struct {
	Op           string
	Status       string
	SourcePath   string
	TargetPath   string
	ErrorMessage string
}

// MetaEventSink 元数据事件回调
type MetaEventSink interface {
	OnMetaEvent(ctx context.Context, event MetaEvent)
}

// MetadataReplicatorOption 复制器配置选项
type MetadataReplicatorOption func(*MetadataReplicator)

// WithPreferMount 设置是否优先使用挂载路径复制
func WithPreferMount(prefer bool) MetadataReplicatorOption {
	return func(r *MetadataReplicator) {
		r.preferMount = prefer
	}
}

// WithEventSink 设置元数据事件回调
func WithEventSink(sink MetaEventSink) MetadataReplicatorOption {
	return func(r *MetadataReplicator) {
		r.eventSink = sink
	}
}

// NewMetadataReplicator 创建元数据复制器
func NewMetadataReplicator(fs filesystem.Client, targetRoot string, logger *zap.Logger, opts ...MetadataReplicatorOption) ports.MetadataReplicator {
	absRoot, err := filepath.Abs(targetRoot)
	if err != nil {
		logger.Warn("无法获取绝对路径，使用清理后的路径",
			zap.String("target_root", targetRoot),
			zap.Error(err))
		absRoot = filepath.Clean(targetRoot)
	}

	r := &MetadataReplicator{
		fs:          fs,
		targetRoot:  absRoot,
		logger:      logger,
		preferMount: true, // 默认优先挂载路径
	}

	// 应用选项
	for _, opt := range opts {
		opt(r)
	}

	r.logger.Info("创建元数据复制器",
		zap.String("target_root", r.targetRoot),
		zap.Bool("prefer_mount", r.preferMount))

	return r
}

// Apply 执行元数据文件复制/下载（创建/更新/删除元数据文件）
func (r *MetadataReplicator) Apply(ctx context.Context, items <-chan ports.SyncPlanItem) (succeeded int, failed int, err error) {
	r.logger.Info("开始处理元数据计划项",
		zap.String("result", "开始处理元数据计划项"),
		zap.String("source", "同步引擎.元数据复制"))
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			r.logger.Warn("元数据复制被取消",
				zap.String("result", "元数据复制被取消"),
				zap.String("source", "同步引擎.元数据复制"),
				zap.Int("succeeded", succeeded),
				zap.Int("failed", failed),
				zap.Duration("elapsed", elapsed),
				zap.Error(ctx.Err()))
			return succeeded, failed, ctx.Err()

		case item, ok := <-items:
			if !ok {
				// 通道关闭，所有项目已处理完成
				elapsed := time.Since(startTime)
				r.logger.Info("元数据复制完成",
					zap.String("result", "元数据复制完成"),
					zap.String("source", "同步引擎.元数据复制"),
					zap.Int("succeeded", succeeded),
					zap.Int("failed", failed),
					zap.Duration("elapsed", elapsed))
				return succeeded, failed, nil
			}

			// 只处理元数据类型的计划项
			if item.Kind != ports.PlanItemMetadata {
				r.logger.Debug("跳过非元数据计划项",
					zap.String("kind", item.Kind.String()),
					zap.String("source_path", item.SourcePath))
				continue
			}

			// 处理单个元数据项
			if err := r.applyItem(ctx, &item); err != nil {
				r.logger.Error("元数据项处理失败",
					zap.String("op", item.Op.String()),
					zap.String("source_path", item.SourcePath),
					zap.String("target_path", item.TargetMetaPath),
					zap.Error(err))
				r.emitMetaEvent(ctx, &item, "failed", err.Error())
				failed++
			} else {
				r.logger.Debug("元数据项处理成功",
					zap.String("op", item.Op.String()),
					zap.String("source_path", item.SourcePath),
					zap.String("target_path", item.TargetMetaPath))
				r.emitMetaEvent(ctx, &item, "success", "")
				succeeded++
			}
		}
	}
}

func (r *MetadataReplicator) emitMetaEvent(ctx context.Context, item *ports.SyncPlanItem, status string, errMsg string) {
	if r == nil || r.eventSink == nil || item == nil {
		return
	}
	op := "copy"
	switch item.Op {
	case ports.SyncOpUpdate:
		op = "update"
	case ports.SyncOpDelete:
		op = "delete"
	case ports.SyncOpCreate:
		op = "copy"
	}
	r.eventSink.OnMetaEvent(ctx, MetaEvent{
		Op:           op,
		Status:       status,
		SourcePath:   item.SourcePath,
		TargetPath:   item.TargetMetaPath,
		ErrorMessage: errMsg,
	})
}

// applyItem 处理单个元数据计划项
func (r *MetadataReplicator) applyItem(ctx context.Context, item *ports.SyncPlanItem) error {
	// 验证目标路径
	if err := r.validatePath(item.TargetMetaPath); err != nil {
		return fmt.Errorf("校验目标路径失败: %w", err)
	}

	switch item.Op {
	case ports.SyncOpCreate, ports.SyncOpUpdate:
		return r.copyOrDownload(ctx, item)
	case ports.SyncOpDelete:
		return r.deleteMeta(item.TargetMetaPath)
	default:
		return fmt.Errorf("未知操作: %v", item.Op)
	}
}

// copyOrDownload 根据策略选择复制或下载
func (r *MetadataReplicator) copyOrDownload(ctx context.Context, item *ports.SyncPlanItem) error {
	r.logger.Debug("开始复制/下载元数据文件",
		zap.String("source", item.SourcePath),
		zap.String("target", item.TargetMetaPath),
		zap.Bool("prefer_mount", r.preferMount))

	if r.preferMount {
		// 策略1: 复制模式，仅使用访问路径复制（不触发API下载）
		accessPath, err := r.fs.ResolveAccessPath(ctx, item.SourcePath)
		if err != nil {
			return fmt.Errorf("解析访问路径失败: %w", err)
		}
		r.logger.Debug("使用访问路径复制",
			zap.String("access_path", accessPath),
			zap.String("target", item.TargetMetaPath))
		return r.copyLocal(accessPath, item.TargetMetaPath, item.ModTime)
	}

	// 策略2: 下载模式，优先API下载，失败后回退访问路径复制
	if err := r.downloadToFile(ctx, item.SourcePath, item.TargetMetaPath, item.ModTime); err != nil {
		r.logger.Debug("API下载失败，尝试访问路径复制",
			zap.String("source", item.SourcePath),
			zap.Error(err))

		if accessPath, err2 := r.fs.ResolveAccessPath(ctx, item.SourcePath); err2 == nil {
			r.logger.Debug("使用访问路径复制（回退）",
				zap.String("access_path", accessPath),
				zap.String("target", item.TargetMetaPath))
			if copyErr := r.copyLocal(accessPath, item.TargetMetaPath, item.ModTime); copyErr != nil {
				return fmt.Errorf("API 下载失败: %w；回退复制失败: %v", err, copyErr)
			}
			return nil
		}

		return err
	}
	return nil
}

// copyLocal 从本地挂载路径复制文件
func (r *MetadataReplicator) copyLocal(srcPath, dstPath string, modTime time.Time) error {
	r.logger.Debug("本地复制开始",
		zap.String("src", srcPath),
		zap.String("dst", dstPath))

	startTime := time.Now()

	// 确保目标目录存在
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败 %s: %w", dstDir, err)
	}

	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("源文件不存在: %s", srcPath)
		}
		return fmt.Errorf("打开源文件失败 %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	// 获取源文件信息
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("获取源文件信息失败 %s: %w", srcPath, err)
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败 %s: %w", dstPath, err)
	}

	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		dstFile.Close()
		_ = os.Remove(dstPath)
		return fmt.Errorf("复制文件失败 %s -> %s: %w", srcPath, dstPath, err)
	}

	if err := dstFile.Close(); err != nil {
		_ = os.Remove(dstPath)
		return fmt.Errorf("关闭目标文件失败 %s: %w", dstPath, err)
	}

	// 设置修改时间
	if !modTime.IsZero() {
		if err := os.Chtimes(dstPath, modTime, modTime); err != nil {
			r.logger.Warn("设置文件时间失败",
				zap.String("path", dstPath),
				zap.Error(err))
		}
	}

	elapsed := time.Since(startTime)
	r.logger.Info(fmt.Sprintf("本地复制完成：%s -> %s", srcPath, dstPath),
		zap.String("src", srcPath),
		zap.String("dst", dstPath),
		zap.Int64("bytes", written),
		zap.Int64("src_size", srcInfo.Size()),
		zap.Duration("elapsed", elapsed))

	return nil
}

// downloadToFile 通过API下载文件
func (r *MetadataReplicator) downloadToFile(ctx context.Context, remotePath, dstPath string, modTime time.Time) error {
	r.logger.Debug("API下载开始",
		zap.String("remote", remotePath),
		zap.String("dst", dstPath))

	startTime := time.Now()

	// 确保目标目录存在
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败 %s: %w", dstDir, err)
	}

	// 创建临时文件
	tmpPath := dstPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败 %s: %w", tmpPath, err)
	}

	// 下载文件内容
	if err := r.fs.Download(ctx, remotePath, tmpFile); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("API 下载失败 %s: %w", remotePath, err)
	}

	// 获取下载的文件大小
	tmpInfo, _ := tmpFile.Stat()
	written := tmpInfo.Size()

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("关闭临时文件失败 %s: %w", tmpPath, err)
	}

	// 原子性地重命名临时文件为目标文件
	if err := os.Rename(tmpPath, dstPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名临时文件失败 %s -> %s: %w", tmpPath, dstPath, err)
	}

	// 设置修改时间
	if !modTime.IsZero() {
		if err := os.Chtimes(dstPath, modTime, modTime); err != nil {
			r.logger.Warn("设置文件时间失败",
				zap.String("path", dstPath),
				zap.Error(err))
		}
	}

	elapsed := time.Since(startTime)
	r.logger.Info(fmt.Sprintf("API下载完成：%s -> %s", remotePath, dstPath),
		zap.String("remote", remotePath),
		zap.String("dst", dstPath),
		zap.Int64("bytes", written),
		zap.Duration("elapsed", elapsed))

	return nil
}

// deleteMeta 删除元数据文件
func (r *MetadataReplicator) deleteMeta(path string) error {
	r.logger.Debug("删除元数据文件", zap.String("path", path))

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		r.logger.Debug("文件不存在，跳过删除", zap.String("path", path))
		return nil
	}

	// 删除文件
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("删除文件失败 %s: %w", path, err)
	}

	r.logger.Info(fmt.Sprintf("元数据文件已删除：%s", path), zap.String("path", path))
	return nil
}

// validatePath 验证目标路径在允许的根目录内
func (r *MetadataReplicator) validatePath(targetPath string) error {
	if r.targetRoot == "" {
		// 未配置根目录，不做验证
		return nil
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败: %w", err)
	}

	rel, err := filepath.Rel(r.targetRoot, absTarget)
	if err != nil {
		return fmt.Errorf("计算相对路径失败: %w", err)
	}

	// 检查是否逃逸到父目录
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("路径 %s 超出根目录 %s", targetPath, r.targetRoot)
	}

	return nil
}
