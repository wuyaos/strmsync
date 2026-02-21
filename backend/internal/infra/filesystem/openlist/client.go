// Package filesystem provides OpenList filesystem operations.
//
// This package uses the OpenList SDK (internal/pkg/sdk/openlist) for API communication
// and provides filesystem-level abstractions (List, Stat, etc.) for the sync engine.
package filesystem

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	openlistsdk "github.com/strmsync/strmsync/internal/pkg/sdk/openlist"
	"go.uber.org/zap"
)

// openListProvider OpenList文件系统实现
type openListProvider struct {
	config  filesystem.Config
	baseURL *url.URL
	client  *openlistsdk.Client
	logger  *zap.Logger
}

// NewOpenListProvider 创建OpenList filesystem.Provider
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
		config:  c.Config,
		baseURL: c.BaseURL,
		client:  client,
		logger:  c.Logger,
	}, nil
}

// List 列出目录内容
func (p *openListProvider) List(ctx context.Context, listPath string, recursive bool, maxDepth int) ([]filesystem.RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(listPath) == "" {
		listPath = "/"
	}
	return p.listOpenList(ctx, listPath, recursive, maxDepth)
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
	_, err := p.client.List(ctx, "/")
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

// listOpenList 递归列出 OpenList 目录（使用 BFS，支持深度限制）
func (p *openListProvider) listOpenList(ctx context.Context, root string, recursive bool, maxDepth int) ([]filesystem.RemoteFile, error) {
	var results []filesystem.RemoteFile

	// 使用 BFS 队列遍历目录树
	type queueItem struct {
		path  string
		depth int
	}
	queue := []queueItem{{path: filesystem.CleanRemotePath(root), depth: 0}}

	for len(queue) > 0 {
		// 检查 context 取消
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// 取出队头目录
		item := queue[0]
		dir := item.path
		queue = queue[1:]

		// 列出当前目录
		items, err := p.client.List(ctx, dir)
		if err != nil {
			return nil, fmt.Errorf("list directory %s: %w", dir, err)
		}

		// 处理每个项目
		for _, sdkItem := range items {
			fullPath := filesystem.JoinRemotePath(dir, sdkItem.Name)

			// 转换为RemoteFile
			remoteFile := filesystem.RemoteFile{
				Path:    fullPath,
				Name:    sdkItem.Name,
				Size:    sdkItem.Size,
				ModTime: sdkItem.Modified,
				IsDir:   sdkItem.IsDir,
			}

			// 将所有项目（文件和目录）加入结果
			results = append(results, remoteFile)

			// 递归模式：将子目录加入队列（深度控制）
			if sdkItem.IsDir && recursive && item.depth+1 < maxDepth {
				queue = append(queue, queueItem{path: fullPath, depth: item.depth + 1})
			}
		}
	}

	p.logger.Info("OpenList 目录列出完成",
		zap.String("root", root),
		zap.Bool("recursive", recursive),
		zap.Int("max_depth", maxDepth),
		zap.Int("count", len(results)))

	return results, nil
}

func init() {
	filesystem.RegisterProvider(filesystem.TypeOpenList, func(c *filesystem.ClientImpl) (filesystem.Provider, error) {
		return NewOpenListProvider(c)
	})
}
