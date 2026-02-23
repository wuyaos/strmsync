// Package filesystem 提供 CloudDrive2 文件系统 Provider 实现。
//
// 说明：
// - 该子包与父包共享同一命名空间，通过 init() 自动注册 Provider
// - 仅应以副作用方式导入（完成注册）
//
// 用法：
//
//	import _ "github.com/strmsync/strmsync/internal/infra/filesystem/clouddrive2"
//
// CloudDrive2 Provider 使用 gRPC 与服务端通信，支持 HTTP STRM 与挂载路径 STRM。
//
// 导出：
// - NewCloudDrive2Provider：创建 Provider 实例（用于注册）
package filesystem

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	syncengine "github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"github.com/strmsync/strmsync/internal/pkg/errutil"
	"github.com/strmsync/strmsync/internal/pkg/qos"
	"github.com/sourcegraph/conc/pool"
	cd2sdk "github.com/strmsync/strmsync/internal/pkg/sdk/clouddrive2"
	pb "github.com/strmsync/strmsync/internal/pkg/sdk/clouddrive2/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------- CloudDrive2 Provider 实现 ----------

// cloudDrive2Provider CloudDrive2 文件系统实现
type cloudDrive2Provider struct {
	config          filesystem.Config
	logger          *zap.Logger
	client          *cd2sdk.CloudDrive2Client
	baseURL         *url.URL
	httpClient      *http.Client
	apiLimiter      *qos.Limiter
	downloadLimiter *qos.Limiter
}

// NewCloudDrive2Provider 创建 CloudDrive2 Provider
func NewCloudDrive2Provider(c *filesystem.ClientImpl) (filesystem.Provider, error) {
	// 从 BaseURL 解析 host:port
	if c.BaseURL == nil {
		return nil, fmt.Errorf("filesystem: baseURL is required for CloudDrive2")
	}

	target := c.BaseURL.Host
	if target == "" {
		return nil, fmt.Errorf("filesystem: invalid baseURL host for CloudDrive2")
	}

	// 创建CloudDrive2客户端（Password字段存储API Token）
	client := cd2sdk.NewCloudDrive2Client(target, c.Config.Password, cd2sdk.WithTimeout(c.Config.Timeout))

	return &cloudDrive2Provider{
		config:          c.Config,
		logger:          c.Logger,
		client:          client,
		baseURL:         c.BaseURL,
		httpClient:      c.HTTPClient,
		apiLimiter:      c.APILimiter,
		downloadLimiter: c.DownloadLimiter,
	}, nil
}

// Scan 流式扫描目录内容
func (p *cloudDrive2Provider) Scan(ctx context.Context, listPath string, recursive bool, maxDepth int) (<-chan filesystem.RemoteFile, <-chan error) {
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

		workerPool := pool.New().WithMaxGoroutines(5)
		var scanDir func(dir string, depth int)
		scanDir = func(dir string, depth int) {
			workerPool.Go(func() error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				files, err := p.listDir(ctx, dir)
				if err != nil {
					return err
				}
				for _, file := range files {
					if file == nil {
						continue
					}
					fullPath := filesystem.JoinRemotePath(dir, file.Name)
					modTime := parseProtoTimestamp(file.WriteTime)
					select {
					case entryCh <- filesystem.RemoteFile{
						Path:    fullPath,
						Name:    file.Name,
						Size:    file.Size,
						ModTime: modTime,
						IsDir:   file.IsDirectory,
					}:
					case <-ctx.Done():
						return ctx.Err()
					}
					if recursive && file.IsDirectory && depth+1 < maxDepthLimit {
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

// Watch 监控目录变化（CloudDrive2不支持）
func (p *cloudDrive2Provider) Watch(ctx context.Context, path string) (<-chan filesystem.FileEvent, error) {
	return nil, filesystem.ErrNotSupported
}

// TestConnection 测试连接
func (p *cloudDrive2Provider) TestConnection(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.logger.Info("测试CloudDrive2连接", zap.String("type", filesystem.TypeCloudDrive2.String()))

	// 使用GetSystemInfo测试连接（公开接口）
	release, err := p.acquireAPI(ctx)
	if err != nil {
		return err
	}
	defer release()
	_, err = p.client.GetSystemInfo(ctx)
	if err != nil {
		p.logger.Error("CloudDrive2连接失败", zap.Error(err))
		return fmt.Errorf("filesystem: test connection failed: %w", err)
	}

	p.logger.Info("CloudDrive2连接成功")
	return nil
}

// Download 下载文件内容到writer
func (p *cloudDrive2Provider) Download(ctx context.Context, remotePath string, w io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(remotePath) == "" {
		return fmt.Errorf("clouddrive2: remote path cannot be empty")
	}
	if p.baseURL == nil || p.httpClient == nil {
		return fmt.Errorf("clouddrive2: baseURL or httpClient not configured")
	}

	cleanPath := filesystem.CleanRemotePath(remotePath)
	p.logger.Debug("CloudDrive2 Download", zap.String("path", cleanPath))

	// 调用gRPC获取下载URL信息
	release, err := p.acquireAPI(ctx)
	if err != nil {
		return err
	}
	info, err := p.client.GetDownloadUrlPath(ctx, cleanPath, false, true, true)
	release()
	if err != nil {
		return fmt.Errorf("clouddrive2: get download url path failed: %w", err)
	}

	// 优先使用directUrl（云存储直链）
	downloadURL := info.GetDirectUrl()
	if downloadURL == "" {
		// 使用downloadUrlPath模板构建URL
		downloadPath := info.GetDownloadUrlPath()
		replacer := strings.NewReplacer(
			"{SCHEME}", p.baseURL.Scheme,
			"{HOST}", p.baseURL.Host,
			"{PREVIEW}", "0",
		)
		downloadPath = replacer.Replace(downloadPath)
		downloadURL = strings.TrimRight(p.baseURL.String(), "/") + downloadPath
	}

	p.logger.Debug("CloudDrive2 Download URL", zap.String("url", downloadURL))

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("clouddrive2: create download request: %w", err)
	}

	// 添加UserAgent和额外headers（如果有）
	if ua := info.GetUserAgent(); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	for k, v := range info.GetAdditionalHeaders() {
		req.Header.Set(k, v)
	}

	// 执行下载
	releaseDownload, err := p.acquireDownload(ctx)
	if err != nil {
		return err
	}
	resp, err := p.httpClient.Do(req)
	releaseDownload()
	if err != nil {
		return fmt.Errorf("clouddrive2: download request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode/100 != 2 {
		err := fmt.Errorf("clouddrive2: download status %d", resp.StatusCode)
		if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return errutil.Retryable(err)
		}
		return err
	}

	// 将响应写入writer
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("clouddrive2: copy download body: %w", err)
	}

	return nil
}

// Stat 获取单个路径的元数据
//
// 实现说明：
// - 使用 CloudDrive2 的 FindFileByPath API 查询文件/目录信息
// - 路径 "/" 特殊处理为根目录
// - 路径分解为父目录和文件名分别传递给 API
//
// 参数：
//   - ctx: 上下文，用于取消和超时控制
//   - targetPath: 要查询的远程路径（Unix 格式）
//
// 返回：
//   - filesystem.RemoteFile: 文件/目录的元数据
//   - error: 查询失败或路径不存在时返回错误
func (p *cloudDrive2Provider) Stat(ctx context.Context, targetPath string) (filesystem.RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cleanPath := filesystem.CleanRemotePath(targetPath)
	p.logger.Debug("CloudDrive2 Stat",
		zap.String("path", cleanPath))

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

	// 调用 CloudDrive2 API
	info, err := p.client.FindFileByPath(ctx, parentPath, baseName)
	if err != nil {
		return filesystem.RemoteFile{}, fmt.Errorf("clouddrive2: stat %s 失败: %w", cleanPath, err)
	}
	if info == nil {
		return filesystem.RemoteFile{}, fmt.Errorf("clouddrive2: stat %s 返回空结果", cleanPath)
	}

	// 构建完整路径
	fullPath := filesystem.JoinRemotePath(parentPath, info.Name)
	modTime := parseProtoTimestamp(info.WriteTime)

	result := filesystem.RemoteFile{
		Path:    fullPath,
		Name:    info.Name,
		Size:    info.Size,
		ModTime: modTime,
		IsDir:   info.IsDirectory,
	}

	p.logger.Debug("CloudDrive2 Stat 完成",
		zap.String("path", fullPath),
		zap.Bool("is_dir", result.IsDir),
		zap.Int64("size", result.Size))

	return result, nil
}

// BuildStrmInfo 构建结构化的 STRM 信息
//
// 实现说明：
// - 生成基于 HTTP 的流媒体 URL
// - CloudDrive2 使用 HTTP 协议直接访问文件
// - scheme 默认使用 http（CloudDrive2 通常不使用 https）
// - 返回的 StrmInfo 包含 RawURL、BaseURL 和 Path 字段
//
// 参数：
//   - ctx: 上下文（当前未使用，保留用于未来扩展）
//   - req: BuildStrmRequest 包含 ServerID、RemotePath 和可选的 RemoteMeta
//
// 返回：
//   - StrmInfo: 结构化的 STRM 元数据
//   - error: 输入无效或构建失败时返回错误
func (p *cloudDrive2Provider) BuildStrmInfo(ctx context.Context, req syncengine.BuildStrmRequest) (syncengine.StrmInfo, error) {
	_ = ctx // 保留用于未来的取消或追踪

	// 验证输入
	if strings.TrimSpace(req.RemotePath) == "" {
		return syncengine.StrmInfo{}, fmt.Errorf("clouddrive2: remote path 不能为空: %w", syncengine.ErrInvalidInput)
	}

	cleanPath := filesystem.CleanRemotePath(req.RemotePath)
	host := strings.TrimSpace(p.client.Target())
	if host == "" {
		return syncengine.StrmInfo{}, fmt.Errorf("clouddrive2: 主机地址为空")
	}

	// CloudDrive2 默认使用 http（gRPC 使用 h2c，HTTP API 也通常是 http）
	scheme := "http"

	// 构建 URL
	rawURL := fmt.Sprintf("%s://%s%s", scheme, host, cleanPath)

	p.logger.Debug("CloudDrive2 BuildStrmInfo",
		zap.String("remote_file_path", cleanPath),
		zap.String("scheme", scheme),
		zap.String("raw_url", rawURL))

	return syncengine.StrmInfo{
		RawURL:  rawURL,
		BaseURL: &url.URL{Scheme: scheme, Host: host},
		Path:    cleanPath,
	}, nil
}

func (p *cloudDrive2Provider) scanSingleDir(ctx context.Context, dir string, entryCh chan<- filesystem.RemoteFile) error {
	files, err := p.listDir(ctx, dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file == nil {
			continue
		}
		fullPath := filesystem.JoinRemotePath(dir, file.Name)
		modTime := parseProtoTimestamp(file.WriteTime)
		select {
		case entryCh <- filesystem.RemoteFile{
			Path:    fullPath,
			Name:    file.Name,
			Size:    file.Size,
			ModTime: modTime,
			IsDir:   file.IsDirectory,
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (p *cloudDrive2Provider) listDir(ctx context.Context, dir string) ([]*pb.CloudDriveFile, error) {
	release, err := p.acquireAPI(ctx)
	if err != nil {
		return nil, err
	}
	files, err := p.client.GetSubFiles(ctx, dir, false)
	release()
	if err != nil {
		return nil, fmt.Errorf("list directory %s: %w", dir, err)
	}
	return files, nil
}

func (p *cloudDrive2Provider) acquireAPI(ctx context.Context) (func(), error) {
	if p.apiLimiter == nil {
		return func() {}, nil
	}
	return p.apiLimiter.Acquire(ctx)
}

func (p *cloudDrive2Provider) acquireDownload(ctx context.Context) (func(), error) {
	if p.downloadLimiter != nil {
		return p.downloadLimiter.Acquire(ctx)
	}
	return p.acquireAPI(ctx)
}

// parseProtoTimestamp 解析protobuf Timestamp为time.Time
func parseProtoTimestamp(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

func init() {
	filesystem.RegisterProvider(filesystem.TypeCloudDrive2, func(c *filesystem.ClientImpl) (filesystem.Provider, error) {
		return NewCloudDrive2Provider(c)
	})
}
