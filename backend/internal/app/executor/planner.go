// Package planner 实现同步计划器
package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/strmsync/strmsync/internal/app/service"
	"github.com/strmsync/strmsync/internal/filesystem"
)

// Planner 同步计划器实现
type Planner struct {
	dataServerClient filesystem.Client
}

// NewPlanner 创建同步计划器
func NewPlanner(dataServerClient filesystem.Client) service.SyncPlanner {
	return &Planner{
		dataServerClient: dataServerClient,
	}
}

// Plan 根据文件事件生成同步计划
func (p *Planner) Plan(ctx context.Context, config *service.JobConfig, events <-chan service.FileEvent) (<-chan service.SyncPlanItem, <-chan error) {
	itemCh := make(chan service.SyncPlanItem)
	errCh := make(chan error, 1)

	go func() {
		defer close(itemCh)
		defer close(errCh)

		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return

			case event, ok := <-events:
				if !ok {
					// Events channel关闭，所有事件已处理完成
					return
				}

				// 处理单个事件
				item, err := p.planItem(ctx, config, &event)
				if err != nil {
					// 跳过错误的项目，继续处理
					// TODO: 记录警告日志
					continue
				}

				// 发送计划项
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				case itemCh <- *item:
				}
			}
		}
	}()

	return itemCh, errCh
}

// planItem 处理单个文件事件，生成同步计划项
func (p *Planner) planItem(ctx context.Context, config *service.JobConfig, event *service.FileEvent) (*service.SyncPlanItem, error) {
	// 1. 过滤目录（strm文件只对应实际文件）
	if event.IsDir {
		return nil, fmt.Errorf("skip directory: %s", event.Path)
	}

	// 2. 过滤扩展名
	if !p.isAllowedExtension(event.Path, config.Extensions) {
		return nil, fmt.Errorf("extension not allowed: %s", event.Path)
	}

	// 3. 确定同步操作类型
	var op service.SyncOperation
	switch event.Type {
	case service.FileEventCreate:
		op = service.SyncOpCreate
	case service.FileEventUpdate:
		op = service.SyncOpUpdate
	case service.FileEventDelete:
		op = service.SyncOpDelete
	default:
		return nil, fmt.Errorf("unknown event type: %v", event.Type)
	}

	// 4. 计算目标strm文件路径
	targetStrmPath, err := p.calculateTargetPath(event.Path, config.SourcePath, config.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("calculate target path: %w", err)
	}

	// 5. 构建流媒体URL（仅对create/update操作）
	var streamURL string
	if op != service.SyncOpDelete {
		// 使用AbsPath（完整的CloudDrive2路径）构建URL
		streamURL, err = p.dataServerClient.BuildStreamURL(ctx, config.DataServerID, event.AbsPath)
		if err != nil {
			return nil, fmt.Errorf("build stream url: %w", err)
		}
	}

	// 6. 构建同步计划项
	item := &service.SyncPlanItem{
		Op:             op,
		SourcePath:     event.AbsPath,
		TargetStrmPath: targetStrmPath,
		StreamURL:      streamURL,
		Size:           event.Size,
		ModTime:        event.ModTime,
	}

	return item, nil
}

// isAllowedExtension 检查文件扩展名是否允许
func (p *Planner) isAllowedExtension(path string, allowedExtensions []string) bool {
	if len(allowedExtensions) == 0 {
		// 未指定扩展名，允许所有文件
		return true
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range allowedExtensions {
		if strings.ToLower(allowed) == ext {
			return true
		}
	}

	return false
}

// calculateTargetPath 计算目标strm文件路径
//
// 示例：
//   sourcePath: /115open/FL/AV/日本/已刮削
//   targetPath: /mnt/media/movies
//   filePath: other/movie.mkv (相对于sourcePath)
//   结果: /mnt/media/movies/other/movie.strm
func (p *Planner) calculateTargetPath(filePath, sourcePath, targetPath string) (string, error) {
	// 1. 使用filePath（相对路径）
	relativePath := filePath

	// 2. 替换原始扩展名为.strm
	withoutExt := strings.TrimSuffix(relativePath, filepath.Ext(relativePath))
	strmName := withoutExt + ".strm"

	// 3. 拼接目标路径
	targetStrmPath := filepath.Join(targetPath, strmName)

	// 4. 清理路径
	targetStrmPath = filepath.Clean(targetStrmPath)

	return targetStrmPath, nil
}
