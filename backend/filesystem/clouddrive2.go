// Package clouddrive2 提供 CloudDrive2 gRPC 客户端封装
package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"path"
	"strings"
	"time"

	pb "github.com/strmsync/strmsync/filesystem/clouddrive2_proto"
	"github.com/strmsync/strmsync/syncengine"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Client CloudDrive2 gRPC 客户端，封装连接管理和认证
type CloudDrive2Client struct {
	target      string              // gRPC 服务器地址（host:port）
	token       string              // API Token（JWT）
	timeout     time.Duration       // 默认超时时间
	dialOptions []grpc.DialOption   // gRPC 拨号选项
	conn        *grpc.ClientConn    // gRPC 连接
	svc         pb.CloudDriveFileSrvClient // gRPC 服务客户端
}

// Option 客户端配置选项
type CloudDrive2Option func(*CloudDrive2Client)

// WithTimeout 设置默认超时时间
func WithTimeout(timeout time.Duration) CloudDrive2Option {
	return func(c *CloudDrive2Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithDialOptions 设置 gRPC 拨号选项
func WithDialOptions(opts ...grpc.DialOption) CloudDrive2Option {
	return func(c *CloudDrive2Client) {
		c.dialOptions = append(c.dialOptions, opts...)
	}
}

// NewCloudDrive2Client 创建新的 CloudDrive2 客户端
//
// target: gRPC 服务器地址，格式为 "host:port"
// token: API Token（可选，公开接口无需 token）
// opts: 可选配置
func NewCloudDrive2Client(target, token string, opts ...CloudDrive2Option) *CloudDrive2Client {
	// 自定义 dialer，确保使用 h2c (HTTP/2 cleartext with prior knowledge)
	contextDialer := func(ctx context.Context, addr string) (net.Conn, error) {
		d := &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		return d.DialContext(ctx, "tcp", addr)
	}

	c := &CloudDrive2Client{
		target:  target,
		token:   token,
		timeout: 10 * time.Second, // 默认 10 秒超时
		dialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(contextDialer),
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Connect 建立 gRPC 连接
//
// 如果连接已存在则直接返回。支持自动重连。
func (c *CloudDrive2Client) Connect(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}

	if c.target == "" {
		return errors.New("clouddrive2: target address is empty")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// 如果 context 没有 deadline，使用默认超时
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	conn, err := grpc.DialContext(ctx, c.target, c.dialOptions...)
	if err != nil {
		return fmt.Errorf("clouddrive2: failed to connect to %s: %w", c.target, err)
	}

	c.conn = conn
	c.svc = pb.NewCloudDriveFileSrvClient(conn)
	return nil
}

// Close 关闭 gRPC 连接
func (c *CloudDrive2Client) Close() error {
	if c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil
	c.svc = nil
	return err
}

// withAuth 为 context 添加认证信息
//
// 如果设置了 token，会在 metadata 中添加 "authorization: Bearer <token>"
// 如果 context 没有 deadline，会自动添加默认超时
func (c *CloudDrive2Client) withAuth(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	var cancel context.CancelFunc = func() {}

	// 添加超时
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
	}

	// 添加认证
	if c.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
	}

	return ctx, cancel
}

// GetSystemInfo 获取系统信息（公开接口，无需认证）
//
// 用于健康检查和登录状态查询
func (c *CloudDrive2Client) GetSystemInfo(ctx context.Context) (*pb.CloudDriveSystemInfo, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	// GetSystemInfo是公开接口，不需要认证
	if ctx == nil {
		ctx = context.Background()
	}

	// 添加超时
	var cancel context.CancelFunc = func() {}
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
	}
	defer cancel()

	info, err := c.svc.GetSystemInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("clouddrive2: GetSystemInfo failed: %w", err)
	}

	return info, nil
}

// GetSubFiles 列出目录内容（服务端流式）
//
// path: 目录路径，例如 "/115/Movies"
// forceRefresh: 是否强制刷新缓存
func (c *CloudDrive2Client) GetSubFiles(ctx context.Context, path string, forceRefresh bool) ([]*pb.CloudDriveFile, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := c.withAuth(ctx)
	defer cancel()

	req := &pb.ListSubFileRequest{
		Path:         path,
		ForceRefresh: forceRefresh,
	}

	stream, err := c.svc.GetSubFiles(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("clouddrive2: GetSubFiles(%s) failed: %w", path, err)
	}

	var files []*pb.CloudDriveFile
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("clouddrive2: GetSubFiles(%s) recv failed: %w", path, err)
		}
		files = append(files, reply.GetSubFiles()...)
	}

	return files, nil
}

// FindFileByPath 获取文件或目录信息
//
// parentPath: 父目录路径
// path: 文件或目录路径
func (c *CloudDrive2Client) FindFileByPath(ctx context.Context, parentPath, path string) (*pb.CloudDriveFile, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := c.withAuth(ctx)
	defer cancel()

	req := &pb.FindFileByPathRequest{
		ParentPath: parentPath,
		Path:       path,
	}

	info, err := c.svc.FindFileByPath(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("clouddrive2: FindFileByPath(%s/%s) failed: %w", parentPath, path, err)
	}

	return info, nil
}

// CreateFolder 创建目录
//
// parentPath: 父目录路径
// folderName: 新目录名称
func (c *CloudDrive2Client) CreateFolder(ctx context.Context, parentPath, folderName string) (*pb.CreateFolderResult, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := c.withAuth(ctx)
	defer cancel()

	req := &pb.CreateFolderRequest{
		ParentPath: parentPath,
		FolderName: folderName,
	}

	resp, err := c.svc.CreateFolder(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("clouddrive2: CreateFolder(%s/%s) failed: %w", parentPath, folderName, err)
	}

	if result := resp.GetResult(); result != nil && !result.GetSuccess() {
		return resp, fmt.Errorf("clouddrive2: CreateFolder(%s/%s) error: %s", parentPath, folderName, result.GetErrorMessage())
	}

	return resp, nil
}

// RenameFile 重命名文件或目录
//
// path: 源路径
// newName: 新名称（不含路径）
func (c *CloudDrive2Client) RenameFile(ctx context.Context, path, newName string) (*pb.FileOperationResult, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := c.withAuth(ctx)
	defer cancel()

	req := &pb.RenameFileRequest{
		TheFilePath: path,
		NewName:     newName,
	}

	resp, err := c.svc.RenameFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("clouddrive2: RenameFile(%s -> %s) failed: %w", path, newName, err)
	}

	if !resp.GetSuccess() {
		return resp, fmt.Errorf("clouddrive2: RenameFile(%s -> %s) error: %s", path, newName, resp.GetErrorMessage())
	}

	return resp, nil
}

// MoveFile 移动文件或目录
//
// filePaths: 源路径列表
// destPath: 目标路径
func (c *CloudDrive2Client) MoveFile(ctx context.Context, filePaths []string, destPath string) (*pb.FileOperationResult, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := c.withAuth(ctx)
	defer cancel()

	req := &pb.MoveFileRequest{
		TheFilePaths: filePaths,
		DestPath:     destPath,
	}

	resp, err := c.svc.MoveFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("clouddrive2: MoveFile(%v -> %s) failed: %w", filePaths, destPath, err)
	}

	if !resp.GetSuccess() {
		return resp, fmt.Errorf("clouddrive2: MoveFile(%v -> %s) error: %s", filePaths, destPath, resp.GetErrorMessage())
	}

	return resp, nil
}

// DeleteFile 删除文件或目录
//
// path: 要删除的路径
func (c *CloudDrive2Client) DeleteFile(ctx context.Context, path string) (*pb.FileOperationResult, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := c.withAuth(ctx)
	defer cancel()

	req := &pb.FileRequest{
		Path: path,
	}

	resp, err := c.svc.DeleteFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("clouddrive2: DeleteFile(%s) failed: %w", path, err)
	}

	if !resp.GetSuccess() {
		return resp, fmt.Errorf("clouddrive2: DeleteFile(%s) error: %s", path, resp.GetErrorMessage())
	}

	return resp, nil
}

// GetMountPoints 获取挂载点列表
//
// 返回所有云盘的挂载信息，用于判断是否使用本地挂载模式或 API 模式
func (c *CloudDrive2Client) GetMountPoints(ctx context.Context) (*pb.GetMountPointsResult, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := c.withAuth(ctx)
	defer cancel()

	resp, err := c.svc.GetMountPoints(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("clouddrive2: GetMountPoints failed: %w", err)
	}

	return resp, nil
}

// ---------- CloudDrive2 Provider Implementation ----------

// cloudDrive2Provider CloudDrive2文件系统实现
type cloudDrive2Provider struct {
	config Config
	logger *zap.Logger
	client *CloudDrive2Client
}

// newCloudDrive2Provider 创建CloudDrive2 provider
func newCloudDrive2Provider(c *clientImpl) (provider, error) {
	// 从 BaseURL 解析 host:port
	if c.baseURL == nil {
		return nil, fmt.Errorf("filesystem: baseURL is required for CloudDrive2")
	}

	target := c.baseURL.Host
	if target == "" {
		return nil, fmt.Errorf("filesystem: invalid baseURL host for CloudDrive2")
	}

	// 创建CloudDrive2客户端（Password字段存储API Token）
	client := NewCloudDrive2Client(target, c.config.Password, WithTimeout(c.config.Timeout))

	return &cloudDrive2Provider{
		config: c.config,
		logger: c.logger,
		client: client,
	}, nil
}

// List 列出目录内容
func (p *cloudDrive2Provider) List(ctx context.Context, listPath string, recursive bool, maxDepth int) ([]RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(listPath) == "" {
		listPath = "/"
	}
	return p.listCloudDrive2(ctx, listPath, recursive, maxDepth)
}

// Watch 监控目录变化（CloudDrive2不支持）
func (p *cloudDrive2Provider) Watch(ctx context.Context, path string) (<-chan FileEvent, error) {
	return nil, ErrNotSupported
}

// TestConnection 测试连接
func (p *cloudDrive2Provider) TestConnection(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.logger.Info("测试CloudDrive2连接", zap.String("type", TypeCloudDrive2.String()))

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
//   - RemoteFile: 文件/目录的元数据
//   - error: 查询失败或路径不存在时返回错误
func (p *cloudDrive2Provider) Stat(ctx context.Context, targetPath string) (RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cleanPath := cleanRemotePath(targetPath)
	p.logger.Debug("CloudDrive2 Stat",
		zap.String("path", cleanPath))

	// 特殊处理：根目录
	if cleanPath == "/" {
		return RemoteFile{
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
		return RemoteFile{}, fmt.Errorf("clouddrive2: stat %s 失败: %w", cleanPath, err)
	}
	if info == nil {
		return RemoteFile{}, fmt.Errorf("clouddrive2: stat %s 返回空结果", cleanPath)
	}

	// 构建完整路径
	fullPath := joinRemotePath(parentPath, info.Name)
	modTime := parseProtoTimestamp(info.WriteTime)

	result := RemoteFile{
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

	cleanPath := cleanRemotePath(req.RemotePath)
	host := strings.TrimSpace(p.client.target)
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
func (p *cloudDrive2Provider) listCloudDrive2(ctx context.Context, root string, recursive bool, maxDepth int) ([]RemoteFile, error) {
	var results []RemoteFile

	// 使用BFS队列遍历目录树，队列中存储路径和当前深度
	type queueItem struct {
		path  string
		depth int
	}
	queue := []queueItem{{path: cleanRemotePath(root), depth: 0}}

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

			fullPath := joinRemotePath(dir, file.Name)
			modTime := parseProtoTimestamp(file.WriteTime)

			// 转换为RemoteFile
			remoteFile := RemoteFile{
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
