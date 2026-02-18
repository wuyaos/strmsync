// Package sync 实现STRM同步服务
package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	"go.uber.org/zap"
)

// Generator strm文件生成器实现
type Generator struct {
	targetRoot string      // 目标根路径（绝对路径），用于限制空目录删除和路径校验
	logger     *zap.Logger // 日志器
}

// NewGenerator 创建strm文件生成器
// targetRoot: 目标根路径，应为绝对路径，用于限制操作范围
func NewGenerator(targetRoot string, logger *zap.Logger) ports.StrmGenerator {
	// 转换为绝对路径并Clean
	absRoot, err := filepath.Abs(targetRoot)
	if err != nil {
		logger.Warn("无法获取绝对路径，使用清理后的路径",
			zap.String("target_root", targetRoot),
			zap.Error(err))
		absRoot = filepath.Clean(targetRoot)
	}

	logger.Info("创建STRM生成器", zap.String("target_root", absRoot))

	return &Generator{
		targetRoot: absRoot,
		logger:     logger,
	}
}

// Apply 执行同步计划
func (g *Generator) Apply(ctx context.Context, items <-chan ports.SyncPlanItem) (succeeded int, failed int, err error) {
	g.logger.Info("开始处理STRM计划项")
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			g.logger.Warn("STRM生成被取消",
				zap.Int("succeeded", succeeded),
				zap.Int("failed", failed),
				zap.Duration("elapsed", elapsed),
				zap.Error(ctx.Err()))
			return succeeded, failed, ctx.Err()

		case item, ok := <-items:
			if !ok {
				// Channel关闭，所有项目已处理完成
				elapsed := time.Since(startTime)
				g.logger.Info("STRM生成完成",
					zap.Int("succeeded", succeeded),
					zap.Int("failed", failed),
					zap.Duration("elapsed", elapsed))
				return succeeded, failed, nil
			}

			// 只处理STRM类型的计划项
			if item.Kind != ports.PlanItemStrm {
				g.logger.Debug("跳过非STRM计划项",
					zap.String("kind", item.Kind.String()),
					zap.String("source_path", item.SourcePath))
				continue
			}

			// 处理单个同步项
			if err := g.applyItem(ctx, &item); err != nil {
				g.logger.Error("STRM项处理失败",
					zap.String("op", item.Op.String()),
					zap.String("source_path", item.SourcePath),
					zap.String("target_path", item.TargetStrmPath),
					zap.Error(err))
				failed++
			} else {
				g.logger.Debug("STRM项处理成功",
					zap.String("op", item.Op.String()),
					zap.String("source_path", item.SourcePath),
					zap.String("target_path", item.TargetStrmPath))
				succeeded++
			}
		}
	}
}

// applyItem 处理单个同步项
func (g *Generator) applyItem(ctx context.Context, item *ports.SyncPlanItem) error {
	// 验证目标路径在targetRoot内（防止路径穿越攻击）
	if err := g.validatePath(item.TargetStrmPath); err != nil {
		return err
	}

	switch item.Op {
	case ports.SyncOpCreate, ports.SyncOpUpdate:
		return g.createOrUpdateStrm(ctx, item)
	case ports.SyncOpDelete:
		return g.deleteStrm(ctx, item)
	default:
		return fmt.Errorf("unknown sync operation: %v", item.Op)
	}
}

// validatePath 验证目标路径在targetRoot内
func (g *Generator) validatePath(targetPath string) error {
	if g.targetRoot == "" {
		return nil // 未设置targetRoot，不限制
	}

	// 将targetPath转换为绝对路径并Clean
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("validate path: cannot get absolute path: %w", err)
	}
	absTarget = filepath.Clean(absTarget)

	// 使用filepath.Rel检查相对路径是否以..开头
	rel, err := filepath.Rel(g.targetRoot, absTarget)
	if err != nil {
		return fmt.Errorf("validate path: %w", err)
	}

	// 如果相对路径以..开头，说明targetPath在targetRoot之外
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("path %s is outside targetRoot %s", targetPath, g.targetRoot)
	}

	return nil
}

// createOrUpdateStrm 创建或更新strm文件
func (g *Generator) createOrUpdateStrm(ctx context.Context, item *ports.SyncPlanItem) error {
	g.logger.Debug("创建/更新STRM文件",
		zap.String("target", item.TargetStrmPath),
		zap.String("url", item.StreamURL))

	startTime := time.Now()

	// 1. 确保目标目录存在
	targetDir := filepath.Dir(item.TargetStrmPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory %s: %w", targetDir, err)
	}

	// 2. 写入strm文件
	if err := os.WriteFile(item.TargetStrmPath, []byte(item.StreamURL), 0o644); err != nil {
		return fmt.Errorf("write strm file %s: %w", item.TargetStrmPath, err)
	}

	// 3. 设置文件修改时间（可选，保持与源文件一致）
	if !item.ModTime.IsZero() {
		if err := os.Chtimes(item.TargetStrmPath, item.ModTime, item.ModTime); err != nil {
			g.logger.Warn("设置STRM文件时间失败",
				zap.String("path", item.TargetStrmPath),
				zap.Error(err))
		}
	}

	elapsed := time.Since(startTime)
	g.logger.Info("STRM文件写入成功",
		zap.String("target", item.TargetStrmPath),
		zap.Int("bytes", len(item.StreamURL)),
		zap.Duration("elapsed", elapsed))

	return nil
}

// deleteStrm 删除strm文件
func (g *Generator) deleteStrm(ctx context.Context, item *ports.SyncPlanItem) error {
	g.logger.Debug("删除STRM文件", zap.String("path", item.TargetStrmPath))

	// 1. 检查文件是否存在
	if _, err := os.Stat(item.TargetStrmPath); os.IsNotExist(err) {
		g.logger.Debug("STRM文件不存在，跳过删除", zap.String("path", item.TargetStrmPath))
		return nil
	}

	// 2. 删除文件
	if err := os.Remove(item.TargetStrmPath); err != nil {
		return fmt.Errorf("delete strm file %s: %w", item.TargetStrmPath, err)
	}

	g.logger.Info("STRM文件已删除", zap.String("path", item.TargetStrmPath))

	// 3. 尝试删除空目录（可选）
	targetDir := filepath.Dir(item.TargetStrmPath)
	if err := g.removeEmptyDirs(targetDir); err != nil {
		g.logger.Debug("删除空目录失败（非致命）",
			zap.String("dir", targetDir),
			zap.Error(err))
	}

	return nil
}

// removeEmptyDirs 递归删除空目录（限制在targetRoot范围内）
func (g *Generator) removeEmptyDirs(dir string) error {
	// 如果已达到targetRoot，不再继续向上删除
	if g.targetRoot != "" {
		cleanDir := filepath.Clean(dir)
		cleanRoot := filepath.Clean(g.targetRoot)
		if cleanDir == cleanRoot || !strings.HasPrefix(cleanDir, cleanRoot+string(filepath.Separator)) {
			return nil
		}
	}

	// 读取目录内容
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// 如果目录不为空，不删除
	if len(entries) > 0 {
		return nil
	}

	// 删除空目录
	if err := os.Remove(dir); err != nil {
		return err
	}

	// 递归删除父目录（如果也为空）
	parentDir := filepath.Dir(dir)
	if parentDir != "" && parentDir != "/" && parentDir != "." {
		_ = g.removeEmptyDirs(parentDir) // 忽略错误
	}

	return nil
}
