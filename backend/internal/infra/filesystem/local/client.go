// Package filesystem 提供本地文件系统 Provider 实现。
//
// 说明：
// - 该子包与父包共享同一命名空间，通过 init() 自动注册 Provider
// - 仅应以副作用方式导入（完成注册）
//
// 用法：
//
//	import _ "github.com/strmsync/strmsync/internal/infra/filesystem/local"
//
// 本地 Provider 直接访问本地文件系统，仅支持挂载路径 STRM 模式（不支持 HTTP 流）。
//
// 导出：
// - NewLocalProvider：创建 Provider 实例（用于注册）
package filesystem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	syncengine "github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"github.com/karrick/godirwalk"
	"go.uber.org/zap"
)

// localProvider 本地文件系统实现
type localProvider struct {
	config filesystem.Config
	logger *zap.Logger
}

// NewLocalProvider 创建本地文件系统 Provider
func NewLocalProvider(c *filesystem.ClientImpl) (filesystem.Provider, error) {
	// 验证本地路径
	if c.Config.MountPath == "" {
		return nil, fmt.Errorf("filesystem: 本地模式需要 mount_path")
	}

	return &localProvider{
		config: c.Config,
		logger: c.Logger,
	}, nil
}

// Scan 流式扫描本地目录内容
func (p *localProvider) Scan(ctx context.Context, listPath string, recursive bool, maxDepth int) (<-chan filesystem.RemoteFile, <-chan error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 规范化listPath为相对路径
	normalizedListPath, err := normalizeListPath(listPath)
	if err != nil {
		errCh := make(chan error, 1)
		fileCh := make(chan filesystem.RemoteFile)
		errCh <- err
		close(fileCh)
		close(errCh)
		return fileCh, errCh
	}

	mountRoot := strings.TrimSpace(p.config.MountPath)
	if mountRoot == "" {
		mountRoot = strings.TrimSpace(p.config.StrmMountPath)
	}
	if mountRoot == "" {
		errCh := make(chan error, 1)
		fileCh := make(chan filesystem.RemoteFile)
		errCh <- fmt.Errorf("local: mount_path 不能为空: %w", syncengine.ErrInvalidInput)
		close(fileCh)
		close(errCh)
		return fileCh, errCh
	}
	mountRoot = filepath.Clean(mountRoot)

	// 构建完整路径
	fullPath := filepath.Join(mountRoot, normalizedListPath)

	// 验证路径在挂载点内
	if err := ensureUnderMount(mountRoot, fullPath); err != nil {
		errCh := make(chan error, 1)
		fileCh := make(chan filesystem.RemoteFile)
		errCh <- err
		close(fileCh)
		close(errCh)
		return fileCh, errCh
	}

	fileCh := make(chan filesystem.RemoteFile)
	errCh := make(chan error, 1)

	go func() {
		defer close(fileCh)
		defer close(errCh)

		// 非递归：只读取当前目录
		if !recursive || maxDepth == 0 {
			dirents, err := godirwalk.ReadDirents(fullPath, nil)
			if err != nil {
				errCh <- fmt.Errorf("filesystem: 读取目录失败: %w", err)
				return
			}
			for _, de := range dirents {
				if ctx.Err() != nil {
					errCh <- ctx.Err()
					return
				}
				relPath := filepath.ToSlash(filepath.Join(normalizedListPath, de.Name()))
				virtualPath := path.Join("/", relPath)
				fileCh <- filesystem.RemoteFile{
					Path:  virtualPath,
					Name:  de.Name(),
					IsDir: de.IsDir(),
				}
			}
			p.logger.Info("本地目录扫描完成",
				zap.String("path", fullPath),
				zap.Bool("recursive", false))
			return
		}

		// 递归遍历（带深度控制）
		rootDepth := strings.Count(fullPath, string(filepath.Separator))
		err = godirwalk.Walk(fullPath, &godirwalk.Options{
			Unsorted: true,
			Callback: func(osPathname string, de *godirwalk.Dirent) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if de == nil {
					return nil
				}

				currentDepth := strings.Count(osPathname, string(filepath.Separator)) - rootDepth

				relPath, relErr := filepath.Rel(mountRoot, osPathname)
				if relErr != nil {
					return relErr
				}
				relPath = filepath.ToSlash(relPath)
				virtualPath := path.Clean("/" + relPath)

				select {
				case fileCh <- filesystem.RemoteFile{
					Path:  virtualPath,
					Name:  de.Name(),
					IsDir: de.IsDir(),
				}:
				case <-ctx.Done():
					return ctx.Err()
				}

				if de.IsDir() && maxDepth > 0 && currentDepth >= maxDepth && osPathname != fullPath {
					return filepath.SkipDir
				}
				return nil
			},
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			errCh <- fmt.Errorf("filesystem: 遍历目录失败: %w", err)
			return
		}
		p.logger.Info("本地目录扫描完成",
			zap.String("path", fullPath),
			zap.Bool("recursive", true))
	}()

	return fileCh, errCh
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
		return fmt.Errorf("filesystem: 本地路径不可访问: %w", err)
	}

	if !info.IsDir() {
		p.logger.Error("本地文件系统连接失败：不是目录", zap.String("path", fullPath))
		return fmt.Errorf("filesystem: mount_path 不是目录: %s", fullPath)
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

	mountRoot := strings.TrimSpace(p.config.MountPath)
	if mountRoot == "" {
		mountRoot = strings.TrimSpace(p.config.StrmMountPath)
	}
	if mountRoot == "" {
		return filesystem.RemoteFile{}, fmt.Errorf("local: mount_path 不能为空: %w", syncengine.ErrInvalidInput)
	}
	mountRoot = filepath.Clean(mountRoot)
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
// - 安全性：与 Stat/Scan 保持一致，防止路径逃逸
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

	// 使用与 Stat/Scan 一致的路径规范化逻辑
	normalizedPath, err := normalizeListPath(req.RemotePath)
	if err != nil {
		return syncengine.StrmInfo{}, fmt.Errorf("local: 路径规范化失败: %w", err)
	}

	mountRoot := strings.TrimSpace(p.config.StrmMountPath)
	if mountRoot == "" {
		mountRoot = strings.TrimSpace(p.config.MountPath)
	}
	if mountRoot == "" {
		return syncengine.StrmInfo{}, fmt.Errorf("local: mount_path 不能为空: %w", syncengine.ErrInvalidInput)
	}
	mountRoot = filepath.Clean(mountRoot)
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
		return "", fmt.Errorf("filesystem: list 路径必须是相对路径: %s", listPath)
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
			return "", fmt.Errorf("filesystem: list 路径不能包含 '..': %s", listPath)
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
		return fmt.Errorf("filesystem: 解析 list 路径失败: %w", err)
	}
	if rel == "." {
		return nil
	}
	// 检查是否逃逸到父目录
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("filesystem: list 路径超出挂载点: %s", fullPath)
	}
	return nil
}

// Download local provider不支持Download，只支持挂载路径复制
func (p *localProvider) Download(ctx context.Context, remotePath string, w io.Writer) error {
	return fmt.Errorf("filesystem: 本地驱动不支持 Download: %w", filesystem.ErrNotSupported)
}

func init() {
	filesystem.RegisterProvider(filesystem.TypeLocal, func(c *filesystem.ClientImpl) (filesystem.Provider, error) {
		return NewLocalProvider(c)
	})
}
