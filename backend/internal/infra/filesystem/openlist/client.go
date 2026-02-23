// Package filesystem 提供 OpenList 文件系统操作。
//
// 本包使用 OpenList SDK（internal/pkg/sdk/openlist）进行 API 通信，
// 并为同步引擎提供文件系统层的抽象（Scan/Stat 等）。
package filesystem

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	syncengine "github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"github.com/strmsync/strmsync/internal/pkg/qos"
	"github.com/sourcegraph/conc/pool"
	openlistsdk "github.com/strmsync/strmsync/internal/pkg/sdk/openlist"
	"go.uber.org/zap"
)

// openListProvider OpenList 文件系统实现
type openListProvider struct {
	config          filesystem.Config
	baseURL         *url.URL
	client          *openlistsdk.Client
	logger          *zap.Logger
	apiLimiter      *qos.Limiter
	downloadLimiter *qos.Limiter
}

// NewOpenListProvider 创建 OpenList Provider
func NewOpenListProvider(c *filesystem.ClientImpl) (filesystem.Provider, error) {
	// 创建SDK客户端
	client, err := openlistsdk.NewClient(openlistsdk.Config{
		BaseURL:    c.BaseURL.String(),
		Username:   c.Config.Username,
		Password:   c.Config.Password,
		Timeout:    c.Config.Timeout,
		HTTPClient: c.HTTPClient,
	})
	if err != nil {
		return nil, fmt.Errorf("filesystem: create openlist client: %w", err)
	}

	return &openListProvider{
		config:          c.Config,
		baseURL:         c.BaseURL,
		client:          client,
		logger:          c.Logger,
		apiLimiter:      c.APILimiter,
		downloadLimiter: c.DownloadLimiter,
	}, nil
}

// Scan 流式扫描目录内容
func (p *openListProvider) Scan(ctx context.Context, listPath string, recursive bool, maxDepth int) (<-chan filesystem.RemoteFile, <-chan error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(listPath) == "" {
		listPath = "/"
	}

	entryCh := make(chan filesystem.RemoteFile)
	errCh := make(chan error, 1)

	go func() {
		defer close(entryCh)
		defer close(errCh)

		root := filesystem.CleanRemotePath(listPath)
		if !recursive || maxDepth == 0 {
			if err := p.scanSingleDir(ctx, root, entryCh); err != nil {
				errCh <- err
			}
			return
		}

		maxDepthLimit := maxDepth
		if maxDepthLimit < 0 {
			maxDepthLimit = 0
		}

		workerPool := pool.New().WithErrors().WithMaxGoroutines(5)
		var scanDir func(dir string, depth int)
		scanDir = func(dir string, depth int) {
			workerPool.Go(func() error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				items, err := p.listDir(ctx, dir)
				if err != nil {
					return err
				}
				for _, item := range items {
					fullPath := filesystem.JoinRemotePath(dir, item.Name)
					select {
					case entryCh <- filesystem.RemoteFile{
						Path:    fullPath,
						Name:    item.Name,
						Size:    item.Size,
						ModTime: item.Modified,
						IsDir:   item.IsDir,
					}:
					case <-ctx.Done():
						return ctx.Err()
					}
					if recursive && item.IsDir && depth+1 < maxDepthLimit {
						scanDir(fullPath, depth+1)
					}
				}
				return nil
			})
		}

		scanDir(root, 0)
		if err := workerPool.Wait(); err != nil && err != context.Canceled {
			errCh <- err
			return
		}
	}()

	return entryCh, errCh
}

// Watch 监控目录变化（OpenList不支持）
func (p *openListProvider) Watch(ctx context.Context, path string) (<-chan filesystem.FileEvent, error) {
	return nil, filesystem.ErrNotSupported
}

// TestConnection 测试连接
func (p *openListProvider) TestConnection(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.logger.Info("测试OpenList连接", zap.String("type", filesystem.TypeOpenList.String()))
	release, err := p.acquireAPI(ctx)
	if err != nil {
		return err
	}
	defer release()
	_, err = p.client.List(ctx, "/")
	if err != nil {
		p.logger.Error("OpenList连接失败", zap.Error(err))
		return fmt.Errorf("filesystem: test connection failed: %w", err)
	}
	p.logger.Info("OpenList连接成功")
	return nil
}

// Download 下载文件内容到writer
func (p *openListProvider) Download(ctx context.Context, remotePath string, w io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(remotePath) == "" {
		return fmt.Errorf("openlist: remote path cannot be empty")
	}

	cleanPath := filesystem.CleanRemotePath(remotePath)
	p.logger.Debug("OpenList Download", zap.String("path", cleanPath))

	release, err := p.acquireDownload(ctx)
	if err != nil {
		return err
	}
	defer release()
	return p.client.Download(ctx, cleanPath, w)
}

// Stat 获取单个路径的元数据
func (p *openListProvider) Stat(ctx context.Context, targetPath string) (filesystem.RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cleanPath := filesystem.CleanRemotePath(targetPath)
	p.logger.Debug("OpenList Stat", zap.String("path", cleanPath))

	// 特殊处理：根目录
	if cleanPath == "/" {
		return filesystem.RemoteFile{
			Path:  "/",
			Name:  "/",
			IsDir: true,
		}, nil
	}

	// 分解路径为父目录和文件名
	parentPath := path.Dir(cleanPath)
	if parentPath == "." || parentPath == "" {
		parentPath = "/"
	}
	baseName := path.Base(cleanPath)

	// 列出父目录
	release, err := p.acquireAPI(ctx)
	if err != nil {
		return filesystem.RemoteFile{}, err
	}
	defer release()
	items, err := p.client.List(ctx, parentPath)
	if err != nil {
		return filesystem.RemoteFile{}, fmt.Errorf("openlist: stat 列出父目录 %s 失败: %w", parentPath, err)
	}

	// 查找匹配项
	for _, item := range items {
		if item.Name == baseName {
			fullPath := filesystem.JoinRemotePath(parentPath, item.Name)
			result := filesystem.RemoteFile{
				Path:    fullPath,
				Name:    item.Name,
				Size:    item.Size,
				ModTime: item.Modified,
				IsDir:   item.IsDir,
			}
			p.logger.Debug("OpenList Stat 完成",
				zap.String("path", fullPath),
				zap.Bool("is_dir", result.IsDir),
				zap.Int64("size", result.Size))
			return result, nil
		}
	}

	return filesystem.RemoteFile{}, fmt.Errorf("openlist: 路径不存在: %s", cleanPath)
}

// BuildStrmInfo 构建结构化的 STRM 信息
func (p *openListProvider) BuildStrmInfo(ctx context.Context, req syncengine.BuildStrmRequest) (syncengine.StrmInfo, error) {
	_ = ctx // 保留用于未来的取消或追踪

	// 验证输入
	if strings.TrimSpace(req.RemotePath) == "" {
		return syncengine.StrmInfo{}, fmt.Errorf("openlist: remote path 不能为空: %w", syncengine.ErrInvalidInput)
	}

	cleanPath := filesystem.CleanRemotePath(req.RemotePath)

	// 获取 baseURL
	if p.baseURL == nil {
		return syncengine.StrmInfo{}, fmt.Errorf("openlist: baseURL 未配置")
	}

	host := strings.TrimSpace(p.baseURL.Host)
	if host == "" {
		return syncengine.StrmInfo{}, fmt.Errorf("openlist: 主机地址为空")
	}

	// 从配置中获取 scheme（http 或 https）
	scheme := strings.ToLower(strings.TrimSpace(p.baseURL.Scheme))
	if scheme == "" {
		scheme = "http" // 默认使用 http
	}

	// 构建 URL
	rawURL := fmt.Sprintf("%s://%s%s", scheme, host, cleanPath)

	p.logger.Debug("OpenList BuildStrmInfo",
		zap.String("remote_file_path", cleanPath),
		zap.String("scheme", scheme),
		zap.String("raw_url", rawURL))

	return syncengine.StrmInfo{
		RawURL:  rawURL,
		BaseURL: &url.URL{Scheme: scheme, Host: host},
		Path:    cleanPath,
	}, nil
}

func (p *openListProvider) scanSingleDir(ctx context.Context, dir string, entryCh chan<- filesystem.RemoteFile) error {
	items, err := p.listDir(ctx, dir)
	if err != nil {
		return err
	}
	for _, item := range items {
		fullPath := filesystem.JoinRemotePath(dir, item.Name)
		select {
		case entryCh <- filesystem.RemoteFile{
			Path:    fullPath,
			Name:    item.Name,
			Size:    item.Size,
			ModTime: item.Modified,
			IsDir:   item.IsDir,
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (p *openListProvider) listDir(ctx context.Context, dir string) ([]openlistsdk.FileItem, error) {
	release, err := p.acquireAPI(ctx)
	if err != nil {
		return nil, err
	}
	items, err := p.client.List(ctx, dir)
	release()
	if err != nil {
		return nil, fmt.Errorf("openlist: list directory %s failed: %w", dir, err)
	}
	return items, nil
}

func (p *openListProvider) acquireAPI(ctx context.Context) (func(), error) {
	if p.apiLimiter == nil {
		return func() {}, nil
	}
	return p.apiLimiter.Acquire(ctx)
}

func (p *openListProvider) acquireDownload(ctx context.Context) (func(), error) {
	if p.downloadLimiter != nil {
		return p.downloadLimiter.Acquire(ctx)
	}
	return p.acquireAPI(ctx)
}

func init() {
	filesystem.RegisterProvider(filesystem.TypeOpenList, func(c *filesystem.ClientImpl) (filesystem.Provider, error) {
		return NewOpenListProvider(c)
	})
}
