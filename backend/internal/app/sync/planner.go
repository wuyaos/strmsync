// Package sync 实现STRM同步服务
package sync

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	appconfig "github.com/strmsync/strmsync/internal/config"
	syncengine "github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"go.uber.org/zap"
)

// Planner 同步计划器实现
type Planner struct {
	dataServerClient filesystem.Client
	logger           *zap.Logger
}

// NewPlanner 创建同步计划器
func NewPlanner(dataServerClient filesystem.Client, logger *zap.Logger) ports.SyncPlanner {
	logger.Info("创建同步计划器")
	return &Planner{
		dataServerClient: dataServerClient,
		logger:           logger,
	}
}

// Plan 根据文件事件生成同步计划
func (p *Planner) Plan(ctx context.Context, config *ports.JobConfig, events <-chan ports.FileEvent) (<-chan ports.SyncPlanItem, <-chan error) {
	itemCh := make(chan ports.SyncPlanItem)
	errCh := make(chan error, 1)

	go func() {
		defer close(itemCh)
		defer close(errCh)

		p.logger.Info("开始生成同步计划",
			zap.String("source_path", config.SourcePath),
			zap.String("target_path", config.TargetPath),
			zap.Int("media_exts", len(config.MediaExtensions)),
			zap.Int("meta_exts", len(config.MetaExtensions)))

		processedCount := 0
		skippedCount := 0
		startTime := time.Now()

		excludeDirs := syncengine.NormalizeExcludeDirs(config.ExcludeDirs)

		for {
			select {
			case <-ctx.Done():
				elapsed := time.Since(startTime)
				p.logger.Warn("计划生成被取消",
					zap.Int("processed", processedCount),
					zap.Int("skipped", skippedCount),
					zap.Duration("elapsed", elapsed),
					zap.Error(ctx.Err()))
				errCh <- ctx.Err()
				return

			case event, ok := <-events:
				if !ok {
					// Events channel关闭，所有事件已处理完成
					elapsed := time.Since(startTime)
					p.logger.Info("计划生成完成",
						zap.Int("processed", processedCount),
						zap.Int("skipped", skippedCount),
						zap.Duration("elapsed", elapsed))
					return
				}

				// 处理单个事件
				item, err := p.planItem(ctx, config, &event, excludeDirs)
				if err != nil {
					// 跳过错误的项目，继续处理
					p.logger.Debug("跳过事件",
						zap.String("path", event.Path),
						zap.String("type", event.Type.String()),
						zap.Error(err))
					skippedCount++
					continue
				}

				processedCount++

				// 发送计划项
				select {
				case <-ctx.Done():
					elapsed := time.Since(startTime)
					p.logger.Warn("计划生成被取消（发送时）",
						zap.Int("processed", processedCount),
						zap.Int("skipped", skippedCount),
						zap.Duration("elapsed", elapsed),
						zap.Error(ctx.Err()))
					errCh <- ctx.Err()
					return
				case itemCh <- *item:
					p.logger.Debug("生成计划项",
						zap.String("kind", item.Kind.String()),
						zap.String("op", item.Op.String()),
						zap.String("source", item.SourcePath))
				}
			}
		}
	}()

	return itemCh, errCh
}

// planItem 处理单个文件事件，生成同步计划项
func (p *Planner) planItem(ctx context.Context, config *ports.JobConfig, event *ports.FileEvent, excludeDirs []string) (*ports.SyncPlanItem, error) {
	// 1. 过滤目录（strm文件和元数据文件只对应实际文件）
	if event.IsDir {
		return nil, fmt.Errorf("skip directory: %s", event.Path)
	}

	// 1.1 排除目录过滤
	if syncengine.IsExcludedPath("/", event.Path, excludeDirs) {
		return nil, fmt.Errorf("excluded path: %s", event.Path)
	}

	// 2. 分类文件类型（媒体文件或元数据文件）
	kind := p.classifyExtension(event.Path, config)
	if kind == 0 {
		return nil, fmt.Errorf("extension not allowed: %s", event.Path)
	}

	// 3. 确定同步操作类型
	var op ports.SyncOperation
	switch event.Type {
	case ports.FileEventCreate:
		op = ports.SyncOpCreate
	case ports.FileEventUpdate:
		op = ports.SyncOpUpdate
	case ports.FileEventDelete:
		op = ports.SyncOpDelete
	default:
		return nil, fmt.Errorf("unknown event type: %v", event.Type)
	}

	// 4. 计算目标路径（根据文件类型）
	var targetStrmPath string
	var targetMetaPath string
	var err error

	switch kind {
	case ports.PlanItemStrm:
		targetStrmPath, err = p.calculateTargetStrmPath(event.Path, config.TargetPath)
		if err != nil {
			return nil, fmt.Errorf("calculate target strm path: %w", err)
		}
	case ports.PlanItemMetadata:
		targetMetaPath, err = p.calculateTargetMetaPath(event.Path, config.TargetPath)
		if err != nil {
			return nil, fmt.Errorf("calculate target meta path: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown plan item kind: %v", kind)
	}

	// 5. 构建STRM内容（仅对STRM文件的create/update操作）
	var streamURL string
	if op != ports.SyncOpDelete && kind == ports.PlanItemStrm {
		streamURL, err = p.buildStrmContent(config, event)
		if err != nil {
			return nil, fmt.Errorf("build strm content: %w", err)
		}
	}

	// 6. 构建同步计划项
	item := &ports.SyncPlanItem{
		Op:             op,
		Kind:           kind,
		SourcePath:     event.AbsPath,
		TargetStrmPath: targetStrmPath,
		TargetMetaPath: targetMetaPath,
		StreamURL:      streamURL,
		Size:           event.Size,
		ModTime:        event.ModTime,
	}

	return item, nil
}

// classifyExtension 分类文件扩展名，判断是媒体文件还是元数据文件
func (p *Planner) classifyExtension(path string, config *ports.JobConfig) ports.PlanItemKind {
	ext := strings.ToLower(filepath.Ext(path))

	if len(config.MediaExtensions) > 0 || len(config.MetaExtensions) > 0 {
		// 检查是否为媒体文件
		for _, allowed := range config.MediaExtensions {
			if strings.ToLower(allowed) == ext {
				return ports.PlanItemStrm
			}
		}

		// 检查是否为元数据文件
		for _, allowed := range config.MetaExtensions {
			if strings.ToLower(allowed) == ext {
				return ports.PlanItemMetadata
			}
		}

		return 0 // 不在允许列表中
	}

	// 如果没有配置任何扩展名，默认使用内置的媒体文件列表
	mediaExts := appconfig.DefaultMediaExtensions()
	for _, mediaExt := range mediaExts {
		if mediaExt == ext {
			return ports.PlanItemStrm
		}
	}

	return 0 // 未知类型，不处理
}

// calculateTargetStrmPath 计算目标strm文件路径
//
// 示例：
//
//	filePath: other/movie.mkv (相对于sourcePath)
//	targetPath: /mnt/media/movies
//	结果: /mnt/media/movies/other/movie.strm
func (p *Planner) calculateTargetStrmPath(filePath, targetPath string) (string, error) {
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

// calculateTargetMetaPath 计算目标元数据文件路径
//
// 示例：
//
//	filePath: other/movie.nfo (相对于sourcePath)
//	targetPath: /mnt/media/movies
//	结果: /mnt/media/movies/other/movie.nfo
func (p *Planner) calculateTargetMetaPath(filePath, targetPath string) (string, error) {
	// 1. 使用filePath（相对路径）
	relativePath := filePath

	// 2. 保持原始扩展名（元数据文件不改名）
	// 3. 拼接目标路径
	targetMetaPath := filepath.Join(targetPath, relativePath)

	// 4. 清理路径
	targetMetaPath = filepath.Clean(targetMetaPath)

	return targetMetaPath, nil
}

func (p *Planner) buildStrmContent(config *ports.JobConfig, event *ports.FileEvent) (string, error) {
	if config == nil || event == nil {
		return "", fmt.Errorf("invalid config or event")
	}

	mediaRel := buildMediaRelativePath(config.AccessPath, config.SourcePath, event.Path)
	if mediaRel == "" {
		return "", fmt.Errorf("empty media path")
	}

	var strm string
	switch config.STRMMode {
	case ports.STRMModeLocal:
		basePath := strings.TrimSpace(config.MountPath)
		if basePath == "" {
			basePath = strings.TrimSpace(config.AccessPath)
		}
		if basePath == "" {
			return "", fmt.Errorf("mount_path or access_path is required")
		}
		strm = filepath.Join(basePath, filepath.FromSlash(mediaRel))
	case ports.STRMModeURL:
		baseURL := strings.TrimSpace(config.BaseURL)
		if baseURL == "" {
			return "", fmt.Errorf("base_url is required for url mode")
		}
		remoteBase := normalizeRemotePath(config.AccessPath)
		if remoteBase == "" {
			remoteBase = normalizeRemotePath(config.SourcePath)
		}
		remotePath := normalizeRemotePath(path.Join(remoteBase, mediaRel))
		urlStr, err := buildStreamURL(baseURL, remotePath)
		if err != nil {
			return "", err
		}
		strm = urlStr
	default:
		return "", fmt.Errorf("invalid strm_mode: %s", config.STRMMode)
	}

	return applyStrmReplaceRules(strm, config.STRMReplaceRules), nil
}

func buildMediaRelativePath(accessPath, sourcePath, eventPath string) string {
	access := normalizeRemotePath(accessPath)
	source := normalizeRemotePath(sourcePath)
	eventRel := normalizeRelativePath(eventPath)
	if access == "" {
		access = source
	}

	relBase := ""
	if access != "" && source != "" {
		switch {
		case source == access:
			relBase = ""
		case strings.HasPrefix(source, access+"/"):
			relBase = strings.TrimPrefix(source, access+"/")
		}
	}

	if relBase == "" {
		return eventRel
	}
	return path.Join(relBase, eventRel)
}

func normalizeRemotePath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	cleaned := strings.ReplaceAll(trimmed, "\\", "/")
	return path.Clean("/" + cleaned)
}

func normalizeRelativePath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	cleaned := strings.ReplaceAll(trimmed, "\\", "/")
	cleaned = path.Clean("/" + cleaned)
	return strings.TrimPrefix(cleaned, "/")
}

func buildStreamURL(baseURL, remotePath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid base_url: %s", baseURL)
	}
	normalized := filesystem.CleanRemotePath(remotePath)
	result := *parsed
	result.Path = filesystem.JoinURLPath(result.Path, "/d")
	result.Path = filesystem.JoinURLPath(result.Path, normalized)
	return result.String(), nil
}

func applyStrmReplaceRules(input string, rules []ports.STRMReplaceRule) string {
	output := input
	for _, rule := range rules {
		from := strings.TrimSpace(rule.From)
		if from == "" {
			continue
		}
		if strings.HasPrefix(output, from) {
			output = rule.To + strings.TrimPrefix(output, from)
		}
	}
	return output
}
