// Package filesystem provides local filesystem provider implementation.
//
// This is an internal sub-package that shares the parent filesystem package namespace.
// It automatically registers the Local provider via init() and should only be
// imported for side effects (provider registration).
//
// Usage:
//
//	import _ "github.com/strmsync/strmsync/internal/infra/filesystem/local"
//
// The Local provider accesses files directly from the local filesystem.
// It only supports mount-based STRM mode (no HTTP streaming).
//
// Exports:
//   - NewLocalProvider: Creates a Provider implementation (used by registration)
package filesystem

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"go.uber.org/zap"
)

// localProvider 本地文件系统实现
type localProvider struct {
	config filesystem.Config
	logger *zap.Logger
}

// newLocalProvider 创建本地文件系统 filesystem.Provider
func NewLocalProvider(c *filesystem.ClientImpl) (filesystem.Provider, error) {
	// 验证本地路径
	if c.Config.MountPath == "" {
		return nil, fmt.Errorf("filesystem: local mode requires mount_path")
	}

	return &localProvider{
		config: c.Config,
		logger: c.Logger,
	}, nil
}

// List 列出本地目录内容
func (p *localProvider) List(ctx context.Context, listPath string, recursive bool, maxDepth int) ([]filesystem.RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 规范化listPath为相对路径
	normalizedListPath, err := normalizeListPath(listPath)
	if err != nil {
		return nil, err
	}

	mountRoot := filepath.Clean(p.config.MountPath)

	// 构建完整路径
	fullPath := filepath.Join(mountRoot, normalizedListPath)

	// 验证路径在挂载点内
	if err := ensureUnderMount(mountRoot, fullPath); err != nil {
		return nil, err
	}

	var results []filesystem.RemoteFile

	if recursive {
		// 递归遍历（带深度控制）
		rootDepth := strings.Count(fullPath, string(filepath.Separator))

		err := filepath.WalkDir(fullPath, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				p.logger.Warn("访问路径失败", zap.String("path", filePath), zap.Error(err))
				return nil // 继续遍历
			}

			// 检查 context 取消
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// 计算当前深度（相对于fullPath）
			currentDepth := strings.Count(filePath, string(filepath.Separator)) - rootDepth

			// 获取文件信息
			info, err := d.Info()
			if err != nil {
				p.logger.Warn("获取文件信息失败", zap.String("path", filePath), zap.Error(err))
				return nil
			}

			// 计算相对路径
			relPath, err := filepath.Rel(mountRoot, filePath)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath) // 转换为 Unix 路径
			virtualPath := path.Clean("/" + relPath)

			// 记录当前文件/目录
			results = append(results, filesystem.RemoteFile{
				Path:    virtualPath,
				Name:    info.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
				IsDir:   d.IsDir(),
			})

			// 深度控制：如果是目录且已达最大深度，跳过其子项（但目录本身已被记录）
			if d.IsDir() && currentDepth >= maxDepth && filePath != fullPath {
				return fs.SkipDir
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("filesystem: walk directory: %w", err)
		}
	} else {
		// 只列出当前目录
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return nil, fmt.Errorf("filesystem: read directory: %w", err)
		}

		for _, entry := range entries {
			// 检查 context 取消
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			info, err := entry.Info()
			if err != nil {
				p.logger.Warn("获取文件信息失败", zap.String("name", entry.Name()), zap.Error(err))
				continue
			}

			relPath := filepath.ToSlash(filepath.Join(normalizedListPath, entry.Name()))
			virtualPath := path.Join("/", relPath)

			results = append(results, filesystem.RemoteFile{
				Path:    virtualPath,
				Name:    info.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
				IsDir:   entry.IsDir(),
			})
		}
	}

	p.logger.Info("本地目录列出完成",
		zap.String("path", fullPath),
		zap.Bool("recursive", recursive),
		zap.Int("count", len(results)))

	return results, nil
}

// Watch 监控本地目录变化（暂不支持）
func (p *localProvider) Watch(ctx context.Context, path string) (<-chan filesystem.FileEvent, error) {
	// TODO: 可以使用 fsnotify 实现文件监控
	return nil, filesystem.ErrNotSupported
}

// TestConnection 测试本地文件系统连接
func (p *localProvider) TestConnection(ctx context.Context) error {
	// 检查挂载路径是否可访问
	fullPath := p.config.MountPath
	info, err := os.Stat(fullPath)
	if err != nil {
		p.logger.Error("本地文件系统连接失败", zap.String("path", fullPath), zap.Error(err))
		return fmt.Errorf("filesystem: local path not accessible: %w", err)
	}

	if !info.IsDir() {
		p.logger.Error("本地文件系统连接失败：不是目录", zap.String("path", fullPath))
		return fmt.Errorf("filesystem: mount_path is not a directory: %s", fullPath)
	}

	p.logger.Info("本地文件系统连接成功", zap.String("path", fullPath))
	return nil
}

// Stat 获取单个路径的元数据
//
// 实现说明：
// - 使用 os.Stat 获取本地文件系统信息
// - 路径必须在挂载点范围内（防止路径逃逸）
// - 返回的 Path 字段为虚拟路径（相对于挂载点的 Unix 路径）
//
// 参数：
//   - ctx: 上下文（用于取消，但 os.Stat 本身不支持取消）
//   - targetPath: 要查询的虚拟路径（Unix 格式，相对于挂载点）
//
// 返回：
//   - filesystem.RemoteFile: 文件/目录的元数据
//   - error: 查询失败、路径不存在或路径逃逸时返回错误
func (p *localProvider) Stat(ctx context.Context, targetPath string) (filesystem.RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// 注意：os.Stat 不支持 context 取消，这里检查一次以快速失败
	if ctx.Err() != nil {
		return filesystem.RemoteFile{}, ctx.Err()
	}

	// 规范化路径
	normalizedPath, err := normalizeListPath(targetPath)
	if err != nil {
		return filesystem.RemoteFile{}, fmt.Errorf("local: 路径规范化失败: %w", err)
	}

	mountRoot := filepath.Clean(p.config.MountPath)
	fullPath := filepath.Join(mountRoot, normalizedPath)

	// 安全检查：确保路径在挂载点内
	if err := ensureUnderMount(mountRoot, fullPath); err != nil {
		return filesystem.RemoteFile{}, err
	}

	p.logger.Debug("Local Stat",
		zap.String("virtual_path", targetPath),
		zap.String("physical_path", fullPath))

	// 获取文件信息
	info, err := os.Stat(fullPath)
	if err != nil {
		return filesystem.RemoteFile{}, fmt.Errorf("local: stat %s 失败: %w", fullPath, err)
	}

	// 计算虚拟路径
	relPath, err := filepath.Rel(mountRoot, fullPath)
	if err != nil {
		return filesystem.RemoteFile{}, fmt.Errorf("local: 计算相对路径失败: %w", err)
	}
	relPath = filepath.ToSlash(relPath)
	virtualPath := path.Clean("/" + relPath)

	// 提取文件名
	name := info.Name()
	if virtualPath == "/" {
		name = "/"
	}

	result := filesystem.RemoteFile{
		Path:    virtualPath,
		Name:    name,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}

	p.logger.Debug("Local Stat 完成",
		zap.String("virtual_path", result.Path),
		zap.Bool("is_dir", result.IsDir),
		zap.Int64("size", result.Size))

	return result, nil
}

// BuildStrmInfo 构建结构化的 STRM 信息
//
// 实现说明：
// - 本地文件系统使用挂载路径作为 STRM 内容
// - 不使用 HTTP URL，而是返回本地文件系统的绝对路径
// - BaseURL 字段为 nil（表示本地模式）
// - Path 字段存储完整的本地路径
// - 安全性：与 Stat/List 保持一致，防止路径逃逸
//
// 参数：
//   - ctx: 上下文（当前未使用，保留用于未来扩展）
//   - req: BuildStrmRequest 包含 ServerID、RemotePath 和可选的 RemoteMeta
//
// 返回：
//   - StrmInfo: 结构化的 STRM 元数据（本地路径）
//   - error: 输入无效或路径逃逸时返回错误
func (p *localProvider) BuildStrmInfo(ctx context.Context, req syncengine.BuildStrmRequest) (syncengine.StrmInfo, error) {
	_ = ctx // 保留用于未来的取消或追踪

	// 验证输入
	if strings.TrimSpace(req.RemotePath) == "" {
		return syncengine.StrmInfo{}, fmt.Errorf("local: remote path 不能为空: %w", syncengine.ErrInvalidInput)
	}

	// 使用与 Stat/List 一致的路径规范化逻辑
	normalizedPath, err := normalizeListPath(req.RemotePath)
	if err != nil {
		return syncengine.StrmInfo{}, fmt.Errorf("local: 路径规范化失败: %w", err)
	}

	mountRoot := filepath.Clean(p.config.MountPath)
	localAbsPath := filepath.Join(mountRoot, normalizedPath)

	// 安全检查：确保路径在挂载点内（与 Stat 保持一致）
	if err := ensureUnderMount(mountRoot, localAbsPath); err != nil {
		return syncengine.StrmInfo{}, fmt.Errorf("local: 路径安全检查失败: %w", err)
	}

	// 规范化最终路径
	localAbsPath = filepath.Clean(localAbsPath)

	p.logger.Debug("Local BuildStrmInfo",
		zap.String("virtual_path", req.RemotePath),
		zap.String("local_path", localAbsPath))

	// 返回本地路径作为 STRM 内容
	// 注意：BaseURL 为 nil 表示本地模式
	return syncengine.StrmInfo{
		RawURL:  localAbsPath,
		BaseURL: nil, // 本地模式不使用 BaseURL
		Path:    localAbsPath,
	}, nil
}

// normalizeListPath 规范化listPath为相对路径，防止路径逃逸
func normalizeListPath(listPath string) (string, error) {
	// 空字符串或 "/" 都表示根目录
	if listPath == "" || listPath == "/" {
		return "", nil
	}

	// 拒绝绝对路径
	if filepath.IsAbs(listPath) || filepath.VolumeName(listPath) != "" {
		return "", fmt.Errorf("filesystem: list path must be relative: %s", listPath)
	}

	// 转换为Unix风格路径并去除前导斜杠
	cleaned := filepath.ToSlash(listPath)
	cleaned = strings.TrimLeft(cleaned, "/")
	if cleaned == "" {
		return "", nil
	}

	// 拒绝包含 ".." 的路径（防止路径遍历攻击）
	segments := strings.Split(cleaned, "/")
	for _, segment := range segments {
		if segment == ".." {
			return "", fmt.Errorf("filesystem: list path must not contain '..': %s", listPath)
		}
	}

	// 清理路径
	cleaned = path.Clean(cleaned)
	if cleaned == "." {
		return "", nil
	}

	return cleaned, nil
}

// ensureUnderMount 验证目标路径在挂载点内
func ensureUnderMount(mountRoot, fullPath string) error {
	rel, err := filepath.Rel(mountRoot, fullPath)
	if err != nil {
		return fmt.Errorf("filesystem: resolve list path: %w", err)
	}
	if rel == "." {
		return nil
	}
	// 检查是否逃逸到父目录
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("filesystem: list path escapes mount: %s", fullPath)
	}
	return nil
}

// Download local provider不支持Download，只支持挂载路径复制
func (p *localProvider) Download(ctx context.Context, remotePath string, w io.Writer) error {
	return fmt.Errorf("filesystem: local provider does not support Download: %w", filesystem.ErrNotSupported)
}

func init() {
	filesystem.RegisterProvider(filesystem.TypeLocal, func(c *filesystem.ClientImpl) (filesystem.Provider, error) {
		return NewLocalProvider(c)
	})
}
