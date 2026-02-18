// Package filesystem provides CloudDrive2 filesystem provider implementation.
//
// This is an internal sub-package that shares the parent filesystem package namespace.
// It automatically registers the CloudDrive2 provider via init() and should only be
// imported for side effects (provider registration).
//
// Usage:
//   import _ "github.com/strmsync/strmsync/internal/filesystem/clouddrive2"
//
// The CloudDrive2 provider uses gRPC to communicate with CloudDrive2 server.
// It supports both streaming and HTTP STRM modes.
//
// Exports:
//   - NewCloudDrive2Provider: Creates a Provider implementation (used by registration)
//
// Note: The gRPC client is available in a separate subpackage:
//   github.com/strmsync/strmsync/internal/filesystem/clouddrive2/grpc
package filesystem

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	syncengine "github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/filesystem"
	cd2sdk "github.com/strmsync/strmsync/internal/pkg/sdk/clouddrive2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------- CloudDrive2 Provider Implementation ----------

// cloudDrive2Provider CloudDrive2文件系统实现
type cloudDrive2Provider struct {
	config filesystem.Config
	logger *zap.Logger
	client *cd2sdk.CloudDrive2Client
}

// NewCloudDrive2Provider 创建CloudDrive2 filesystem.Provider
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
		config: c.Config,
		logger: c.Logger,
		client: client,
	}, nil
}

// List 列出目录内容
func (p *cloudDrive2Provider) List(ctx context.Context, listPath string, recursive bool, maxDepth int) ([]filesystem.RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(listPath) == "" {
		listPath = "/"
	}
	return p.listCloudDrive2(ctx, listPath, recursive, maxDepth)
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
	_, err := p.client.GetSystemInfo(ctx)
	if err != nil {
		p.logger.Error("CloudDrive2连接失败", zap.Error(err))
		return fmt.Errorf("filesystem: test connection failed: %w", err)
	}

	p.logger.Info("CloudDrive2连接成功")
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
		zap.String("remote_path", cleanPath),
		zap.String("scheme", scheme),
		zap.String("raw_url", rawURL))

	return syncengine.StrmInfo{
		RawURL:  rawURL,
		BaseURL: &url.URL{Scheme: scheme, Host: host},
		Path:    cleanPath,
	}, nil
}

// listCloudDrive2 递归列出CloudDrive2目录（使用BFS，支持深度限制）
func (p *cloudDrive2Provider) listCloudDrive2(ctx context.Context, root string, recursive bool, maxDepth int) ([]filesystem.RemoteFile, error) {
	var results []filesystem.RemoteFile

	// 使用BFS队列遍历目录树，队列中存储路径和当前深度
	type queueItem struct {
		path  string
		depth int
	}
	queue := []queueItem{{path: filesystem.CleanRemotePath(root), depth: 0}}

	for len(queue) > 0 {
		// 检查context取消
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// 取出队头目录
		item := queue[0]
		dir := item.path
		queue = queue[1:]

		// 调用GetSubFiles列出当前目录
		files, err := p.client.GetSubFiles(ctx, dir, false)
		if err != nil {
			return nil, fmt.Errorf("list directory %s: %w", dir, err)
		}

		// 处理每个项目
		for _, file := range files {
			if file == nil {
				continue
			}

			fullPath := filesystem.JoinRemotePath(dir, file.Name)
			modTime := parseProtoTimestamp(file.WriteTime)

			// 转换为RemoteFile
			remoteFile := filesystem.RemoteFile{
				Path:    fullPath,
				Name:    file.Name,
				Size:    file.Size,
				ModTime: modTime,
				IsDir:   file.IsDirectory,
			}

			// 将所有项目（文件和目录）加入结果
			results = append(results, remoteFile)

			// 递归模式：将子目录加入队列（深度控制）
			// 只有当子目录的内容深度(item.depth+2)不超过maxDepth时才入队
			// 即：item.depth + 1 < maxDepth
			if file.IsDirectory && recursive && item.depth+1 < maxDepth {
				queue = append(queue, queueItem{path: fullPath, depth: item.depth + 1})
			}
		}
	}

	p.logger.Info("CloudDrive2目录列出完成",
		zap.String("root", root),
		zap.Bool("recursive", recursive),
		zap.Int("max_depth", maxDepth),
		zap.Int("count", len(results)))

	return results, nil
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
