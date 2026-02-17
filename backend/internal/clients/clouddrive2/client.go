// Package clouddrive2 提供 CloudDrive2 gRPC 客户端封装
package clouddrive2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	pb "github.com/strmsync/strmsync/internal/clients/clouddrive2/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client CloudDrive2 gRPC 客户端，封装连接管理和认证
type Client struct {
	target      string              // gRPC 服务器地址（host:port）
	token       string              // API Token（JWT）
	timeout     time.Duration       // 默认超时时间
	dialOptions []grpc.DialOption   // gRPC 拨号选项
	conn        *grpc.ClientConn    // gRPC 连接
	svc         pb.CloudDriveFileSrvClient // gRPC 服务客户端
}

// Option 客户端配置选项
type Option func(*Client)

// WithTimeout 设置默认超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithDialOptions 设置 gRPC 拨号选项
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(c *Client) {
		c.dialOptions = append(c.dialOptions, opts...)
	}
}

// NewClient 创建新的 CloudDrive2 客户端
//
// target: gRPC 服务器地址，格式为 "host:port"
// token: API Token（可选，公开接口无需 token）
// opts: 可选配置
func NewClient(target, token string, opts ...Option) *Client {
	// 自定义 dialer，确保使用 h2c (HTTP/2 cleartext with prior knowledge)
	contextDialer := func(ctx context.Context, addr string) (net.Conn, error) {
		d := &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		return d.DialContext(ctx, "tcp", addr)
	}

	c := &Client{
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
func (c *Client) Connect(ctx context.Context) error {
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
func (c *Client) Close() error {
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
func (c *Client) withAuth(ctx context.Context) (context.Context, context.CancelFunc) {
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
func (c *Client) GetSystemInfo(ctx context.Context) (*pb.CloudDriveSystemInfo, error) {
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
func (c *Client) GetSubFiles(ctx context.Context, path string, forceRefresh bool) ([]*pb.CloudDriveFile, error) {
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
func (c *Client) FindFileByPath(ctx context.Context, parentPath, path string) (*pb.CloudDriveFile, error) {
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
func (c *Client) CreateFolder(ctx context.Context, parentPath, folderName string) (*pb.CreateFolderResult, error) {
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
func (c *Client) RenameFile(ctx context.Context, path, newName string) (*pb.FileOperationResult, error) {
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
func (c *Client) MoveFile(ctx context.Context, filePaths []string, destPath string) (*pb.FileOperationResult, error) {
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
func (c *Client) DeleteFile(ctx context.Context, path string) (*pb.FileOperationResult, error) {
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
func (c *Client) GetMountPoints(ctx context.Context) (*pb.GetMountPointsResult, error) {
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
