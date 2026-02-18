// Package clouddrive2 provides CloudDrive2 gRPC SDK client implementation.
//
// This package encapsulates all gRPC communication with CloudDrive2 server,
// including connection management, authentication, and API method calls.
//
// The client supports:
//   - HTTP/2 cleartext (h2c) protocol
//   - JWT bearer token authentication
//   - Automatic connection management
//   - Context-based timeout control
//   - Streaming responses for file listing
package clouddrive2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	pb "github.com/strmsync/strmsync/internal/pkg/sdk/clouddrive2/proto"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CloudDrive2Client CloudDrive2 gRPC 客户端
//
// 封装与 CloudDrive2 服务器的 gRPC 通信，提供连接管理、认证和 API 调用功能。
type CloudDrive2Client struct {
	target      string                     // gRPC 服务器地址（host:port）
	token       string                     // API Token（JWT）
	timeout     time.Duration              // 默认超时时间
	dialOptions []grpcpkg.DialOption       // gRPC 拨号选项
	conn        *grpcpkg.ClientConn        // gRPC 连接
	svc         pb.CloudDriveFileSrvClient // gRPC 服务客户端
}

// CloudDrive2Option 客户端配置选项
//
// 使用函数选项模式（Functional Options Pattern）提供灵活的配置方式。
type CloudDrive2Option func(*CloudDrive2Client)

// WithTimeout 设置默认超时时间
//
// timeout: 超时时长，如果 <= 0 则忽略
func WithTimeout(timeout time.Duration) CloudDrive2Option {
	return func(c *CloudDrive2Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithDialOptions 设置 gRPC 拨号选项
//
// opts: 额外的 gRPC 拨号选项，会追加到默认选项之后
func WithDialOptions(opts ...grpcpkg.DialOption) CloudDrive2Option {
	return func(c *CloudDrive2Client) {
		c.dialOptions = append(c.dialOptions, opts...)
	}
}

// NewCloudDrive2Client 创建新的 CloudDrive2 客户端
//
// 参数：
//   - target: gRPC 服务器地址，格式为 "host:port"
//   - token: API Token（可选，公开接口无需 token）
//   - opts: 可选配置（如超时、拨号选项等）
//
// 返回：
//   - *CloudDrive2Client: 新创建的客户端实例（未建立连接）
func NewCloudDrive2Client(target, token string, opts ...CloudDrive2Option) *CloudDrive2Client {
	// 自定义 dialer，确保使用 h2c (HTTP/2 cleartext with prior knowledge)
	// CloudDrive2 使用 h2c 协议（未加密的 HTTP/2），不是标准 HTTPS
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
		dialOptions: []grpcpkg.DialOption{
			grpcpkg.WithTransportCredentials(insecure.NewCredentials()),
			grpcpkg.WithContextDialer(contextDialer),
		},
	}

	// 应用所有配置选项
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Target 返回配置的 gRPC 目标地址
//
// 此方法用于外部获取客户端连接的服务器地址，主要用于 BuildStrmInfo 构建 URL。
func (c *CloudDrive2Client) Target() string {
	return c.target
}

// Connect 建立 gRPC 连接
//
// 如果连接已存在则直接返回，支持自动重连。
// 连接建立后会缓存，直到显式调用 Close() 或连接失败。
//
// 参数：
//   - ctx: 上下文（可以为 nil，会自动创建）
//
// 返回：
//   - error: 连接失败时返回错误
func (c *CloudDrive2Client) Connect(ctx context.Context) error {
	// 连接已存在，直接返回
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

	conn, err := grpcpkg.DialContext(ctx, c.target, c.dialOptions...)
	if err != nil {
		return fmt.Errorf("clouddrive2: failed to connect to %s: %w", c.target, err)
	}

	c.conn = conn
	c.svc = pb.NewCloudDriveFileSrvClient(conn)
	return nil
}

// Close 关闭 gRPC 连接
//
// 释放连接资源。关闭后需要重新 Connect() 才能使用。
//
// 返回：
//   - error: 关闭失败时返回错误
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
// 内部辅助方法，用于：
//   1. 添加 Bearer Token 到 gRPC metadata
//   2. 为没有 deadline 的 context 添加默认超时
//
// 参数：
//   - ctx: 原始上下文（可以为 nil）
//
// 返回：
//   - context.Context: 添加了认证和超时的新 context
//   - context.CancelFunc: 取消函数（必须 defer 调用）
func (c *CloudDrive2Client) withAuth(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	var cancel context.CancelFunc = func() {}

	// 添加超时
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
	}

	// 添加认证 token
	if c.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
	}

	return ctx, cancel
}

// GetSystemInfo 获取系统信息（公开接口，无需认证）
//
// 用于健康检查和登录状态查询。此接口不需要 token 认证。
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//
// 返回：
//   - *pb.CloudDriveSystemInfo: 系统信息
//   - error: 调用失败时返回错误
func (c *CloudDrive2Client) GetSystemInfo(ctx context.Context) (*pb.CloudDriveSystemInfo, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	// GetSystemInfo 是公开接口，不需要认证
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
// 使用 gRPC 流式响应获取目录下的所有文件和子目录。
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//   - path: 目录路径，例如 "/115/Movies"
//   - forceRefresh: 是否强制刷新缓存
//
// 返回：
//   - []*pb.CloudDriveFile: 文件和目录列表
//   - error: 调用失败时返回错误
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
// 查询指定路径的文件或目录元数据。
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//   - parentPath: 父目录路径
//   - path: 文件或目录名称（不含父路径）
//
// 返回：
//   - *pb.CloudDriveFile: 文件或目录信息
//   - error: 调用失败或路径不存在时返回错误
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
// 在指定父目录下创建新目录。
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//   - parentPath: 父目录路径
//   - folderName: 新目录名称
//
// 返回：
//   - *pb.CreateFolderResult: 创建结果
//   - error: 创建失败时返回错误
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
// 在同一父目录下修改文件或目录名称。
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//   - path: 源路径（完整路径）
//   - newName: 新名称（不含路径）
//
// 返回：
//   - *pb.FileOperationResult: 操作结果
//   - error: 操作失败时返回错误
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
// 将一个或多个文件/目录移动到目标路径。
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//   - filePaths: 源路径列表
//   - destPath: 目标路径
//
// 返回：
//   - *pb.FileOperationResult: 操作结果
//   - error: 操作失败时返回错误
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
// 删除指定路径的文件或目录（目录会递归删除）。
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//   - path: 要删除的路径
//
// 返回：
//   - *pb.FileOperationResult: 操作结果
//   - error: 操作失败时返回错误
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
// 返回所有云盘的挂载信息，用于判断是否使用本地挂载模式或 API 模式。
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//
// 返回：
//   - *pb.GetMountPointsResult: 挂载点列表
//   - error: 调用失败时返回错误
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
