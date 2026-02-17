// Package filesystem 实现远程文件系统客户端
package filesystem

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/strmsync/strmsync/utils"
	"go.uber.org/zap"
)

const (
	defaultTimeout = 10 * time.Second
)

var (
	// ErrNotSupported 表示当前数据源不支持该功能
	ErrNotSupported = errors.New("filesystem: operation not supported")
	// ErrUnauthorized 表示认证失败
	ErrUnauthorized = errors.New("filesystem: unauthorized")
)

// provider 定义具体文件系统的行为
type provider interface {
	List(ctx context.Context, path string, recursive bool) ([]RemoteFile, error)
	Watch(ctx context.Context, path string) (<-chan FileEvent, error)
	TestConnection(ctx context.Context) error
}

type providerFactory func(*clientImpl) (provider, error)

var providerRegistry = map[Type]providerFactory{
	TypeOpenList: newOpenListProvider,
	TypeLocal:    newLocalProvider,
}

// clientImpl 文件系统客户端实现
type clientImpl struct {
	config     Config
	baseURL    *url.URL
	httpClient *http.Client
	logger     *zap.Logger
	provider   provider
}

// Option 客户端可选配置
type Option func(*clientImpl)

// WithHTTPClient 注入自定义HTTP客户端（用于测试）
// 注意：如果自定义客户端没有设置超时，将使用配置的超时时间
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientImpl) {
		if client != nil {
			// 如果自定义客户端没有设置超时，使用配置的超时
			if client.Timeout == 0 {
				client.Timeout = c.httpClient.Timeout
			}
			c.httpClient = client
		}
	}
}

// WithLogger 注入自定义日志器（用于测试）
func WithLogger(logger *zap.Logger) Option {
	return func(c *clientImpl) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// NewClient 创建文件系统客户端
func NewClient(config Config, opts ...Option) (Client, error) {
	// 验证服务器类型
	if !config.Type.IsValid() {
		return nil, fmt.Errorf("filesystem: invalid server type: %s", config.Type)
	}

	// 本地模式不需要 baseURL
	var parsedURL *url.URL
	if config.Type != TypeLocal {
		// 验证 Base URL
		if strings.TrimSpace(config.BaseURL) == "" {
			return nil, fmt.Errorf("filesystem: base_url is required")
		}
		var err error
		parsedURL, err = url.Parse(strings.TrimSpace(config.BaseURL))
		if err != nil {
			return nil, fmt.Errorf("filesystem: parse base_url: %w", err)
		}
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return nil, fmt.Errorf("filesystem: invalid base_url (missing scheme or host): %s", config.BaseURL)
		}
		parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")
	}

	// 验证 STRM 模式
	if !config.STRMMode.IsValid() {
		return nil, fmt.Errorf("filesystem: invalid strm_mode: %s", config.STRMMode)
	}
	if config.STRMMode == STRMModeMount && strings.TrimSpace(config.MountPath) == "" {
		return nil, fmt.Errorf("filesystem: mount_path is required for mount mode")
	}

	// 设置超时
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	// 创建客户端实例
	client := &clientImpl{
		config:     config,
		baseURL:    parsedURL,
		httpClient: &http.Client{Timeout: timeout},
		logger:     utils.L(),
	}

	// 应用可选配置
	for _, opt := range opts {
		opt(client)
	}

	// 确保 httpClient 和 logger 非空
	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: timeout}
	}
	if client.logger == nil {
		client.logger = utils.L()
	}

	// 创建 provider
	provider, err := newProvider(config.Type, client)
	if err != nil {
		return nil, err
	}
	client.provider = provider

	return client, nil
}

// NewDataServerClient 为兼容旧命名提供别名
func NewDataServerClient(config Config, opts ...Option) (Client, error) {
	return NewClient(config, opts...)
}

// newProvider 根据类型创建 provider
func newProvider(t Type, client *clientImpl) (provider, error) {
	factory, ok := providerRegistry[t]
	if !ok {
		return nil, fmt.Errorf("filesystem: provider not registered for type: %s", t)
	}
	return factory(client)
}

// List 列出目录内容
func (c *clientImpl) List(ctx context.Context, listPath string, recursive bool) ([]RemoteFile, error) {
	// 防御 nil context
	if ctx == nil {
		ctx = context.Background()
	}

	// 默认路径为根目录
	if strings.TrimSpace(listPath) == "" {
		listPath = "/"
	}

	if c.provider == nil {
		return nil, fmt.Errorf("filesystem: provider not initialized")
	}
	return c.provider.List(ctx, listPath, recursive)
}

// Watch 监控目录变化
func (c *clientImpl) Watch(ctx context.Context, path string) (<-chan FileEvent, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("filesystem: provider not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.provider.Watch(ctx, path)
}

// BuildStreamURL 构建流媒体URL
func (c *clientImpl) BuildStreamURL(ctx context.Context, serverID uint, filePath string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("filesystem: filePath is required")
	}

	switch c.config.STRMMode {
	case STRMModeHTTP:
		return c.buildHTTPStreamURL(filePath)
	case STRMModeMount:
		return c.buildMountStreamPath(filePath)
	default:
		return "", fmt.Errorf("filesystem: unsupported strm_mode: %s", c.config.STRMMode)
	}
}

// TestConnection 测试连接
func (c *clientImpl) TestConnection(ctx context.Context) error {
	// 防御 nil context
	if ctx == nil {
		ctx = context.Background()
	}

	if c.provider == nil {
		return fmt.Errorf("filesystem: provider not initialized")
	}
	return c.provider.TestConnection(ctx)
}

// ---------- 通用帮助函数 ----------

// buildHTTPStreamURL 构建 HTTP 下载链接
// 格式：http://api_url/d/path/to/file
func (c *clientImpl) buildHTTPStreamURL(filePath string) (string, error) {
	if c.baseURL == nil {
		return "", fmt.Errorf("filesystem: baseURL not initialized for HTTP mode")
	}

	cleanPath := cleanRemotePath(filePath)

	// 复制 baseURL
	result := *c.baseURL

	// 拼接 /d/ 和文件路径
	result.Path = joinURLPath(result.Path, "/d")
	result.Path = joinURLPath(result.Path, cleanPath)

	return result.String(), nil
}

// buildMountStreamPath 构建本地挂载路径
// 将远程路径转换为本地文件系统路径
func (c *clientImpl) buildMountStreamPath(filePath string) (string, error) {
	// 去除开头的斜杠，转换为本地路径分隔符
	cleanPath := strings.TrimPrefix(cleanRemotePath(filePath), "/")
	localPath := filepath.FromSlash(cleanPath)

	// 拼接挂载点和文件路径
	return filepath.Join(c.config.MountPath, localPath), nil
}

// joinURLPath 拼接 URL 路径（保留 baseURL 的前缀）
func joinURLPath(basePath, endpoint string) string {
	if basePath == "" {
		basePath = "/"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return strings.TrimRight(basePath, "/") + endpoint
}

// cleanRemotePath 清理远程路径（统一使用 Unix 路径格式）
func cleanRemotePath(p string) string {
	if strings.TrimSpace(p) == "" {
		return "/"
	}
	// 确保以斜杠开头，使用 path.Clean 清理
	cleaned := path.Clean("/" + p)
	return cleaned
}
