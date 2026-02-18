// Package filesystem 实现本地文件系统客户端
package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// localProvider 本地文件系统实现
type localProvider struct {
	config Config
	logger *zap.Logger
}

// newLocalProvider 创建本地文件系统 provider
func newLocalProvider(c *clientImpl) (provider, error) {
	// 验证本地路径
	if c.config.MountPath == "" {
		return nil, fmt.Errorf("filesystem: local mode requires mount_path")
	}

	// 检查路径是否存在
	if _, err := os.Stat(c.config.MountPath); err != nil {
		return nil, fmt.Errorf("filesystem: mount_path not accessible: %w", err)
	}

	return &localProvider{
		config: c.config,
		logger: c.logger,
	}, nil
}

// List 列出本地目录内容
func (p *localProvider) List(ctx context.Context, listPath string, recursive bool, maxDepth int) ([]RemoteFile, error) {
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

	var results []RemoteFile

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
			results = append(results, RemoteFile{
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

			results = append(results, RemoteFile{
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
func (p *localProvider) Watch(ctx context.Context, path string) (<-chan FileEvent, error) {
	// TODO: 可以使用 fsnotify 实现文件监控
	return nil, ErrNotSupported
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
