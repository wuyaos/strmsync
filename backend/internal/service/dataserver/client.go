// Package dataserver 实现数据服务器客户端
package dataserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/strmsync/strmsync/internal/service"
	"github.com/strmsync/strmsync/internal/service/types"
	"github.com/strmsync/strmsync/internal/utils"
	"go.uber.org/zap"
)

const (
	defaultTimeout     = 10 * time.Second
	maxResponseBodyLen = 4096 // 4KB，用于错误消息读取
)

var (
	// ErrNotSupported 表示当前数据源不支持该功能
	ErrNotSupported = errors.New("dataserver: operation not supported")
	// ErrUnauthorized 表示认证失败
	ErrUnauthorized = errors.New("dataserver: unauthorized")
)

// Client 数据服务器客户端实现
type Client struct {
	config     types.DataServerConfig
	baseURL    *url.URL
	httpClient *http.Client
	logger     *zap.Logger

	// token 管理（用于 OpenList 认证）
	mu    sync.Mutex
	token string
}

// Option 客户端可选配置
type Option func(*Client)

// WithHTTPClient 注入自定义HTTP客户端（用于测试）
// 注意：如果自定义客户端没有设置超时，将使用配置的超时时间
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
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
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// NewDataServerClient 创建数据服务器客户端
func NewDataServerClient(config types.DataServerConfig, opts ...Option) (service.DataServerClient, error) {
	// 验证服务器类型
	if !config.Type.IsValid() {
		return nil, fmt.Errorf("dataserver: invalid server type: %s", config.Type)
	}

	// 验证 Base URL
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("dataserver: base_url is required")
	}
	parsedURL, err := url.Parse(strings.TrimSpace(config.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("dataserver: parse base_url: %w", err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("dataserver: invalid base_url (missing scheme or host): %s", config.BaseURL)
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")

	// 验证 STRM 模式
	if !config.STRMMode.IsValid() {
		return nil, fmt.Errorf("dataserver: invalid strm_mode: %s", config.STRMMode)
	}
	if config.STRMMode == types.STRMModeMount && strings.TrimSpace(config.MountPath) == "" {
		return nil, fmt.Errorf("dataserver: mount_path is required for mount mode")
	}

	// 设置超时
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	// 创建客户端实例
	client := &Client{
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

	return client, nil
}

// List 列出目录内容
func (c *Client) List(ctx context.Context, listPath string, recursive bool) ([]types.RemoteFile, error) {
	// 防御 nil context
	if ctx == nil {
		ctx = context.Background()
	}

	// 默认路径为根目录
	if strings.TrimSpace(listPath) == "" {
		listPath = "/"
	}

	switch c.config.Type {
	case types.DataServerTypeOpenList:
		return c.listOpenList(ctx, listPath, recursive)
	default:
		return nil, fmt.Errorf("dataserver: list not implemented for type: %s", c.config.Type)
	}
}

// Watch 监控目录变化（OpenList 不支持实时监控）
func (c *Client) Watch(ctx context.Context, path string) (<-chan types.FileEvent, error) {
	return nil, ErrNotSupported
}

// BuildStreamURL 构建流媒体URL
func (c *Client) BuildStreamURL(ctx context.Context, serverID types.DataServerID, filePath string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("dataserver: filePath is required")
	}

	switch c.config.STRMMode {
	case types.STRMModeHTTP:
		return c.buildHTTPStreamURL(filePath)
	case types.STRMModeMount:
		return c.buildMountStreamPath(filePath)
	default:
		return "", fmt.Errorf("dataserver: unsupported strm_mode: %s", c.config.STRMMode)
	}
}

// TestConnection 测试连接
func (c *Client) TestConnection(ctx context.Context) error {
	// 防御 nil context
	if ctx == nil {
		ctx = context.Background()
	}

	switch c.config.Type {
	case types.DataServerTypeOpenList:
		// 尝试列出根目录（需要认证时会触发登录）
		c.logger.Info("测试数据服务器连接", zap.String("type", c.config.Type.String()))
		_, err := c.listOpenListOnce(ctx, "/")
		if err != nil {
			c.logger.Error("数据服务器连接失败", zap.Error(err))
			return fmt.Errorf("dataserver: test connection failed: %w", err)
		}
		c.logger.Info("数据服务器连接成功", zap.String("type", c.config.Type.String()))
		return nil
	default:
		return fmt.Errorf("dataserver: test connection not implemented for type: %s", c.config.Type)
	}
}

// ---------- OpenList 实现 ----------

// openListResponse OpenList API 响应格式
type openListResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// openListListReq OpenList /api/fs/list 请求
type openListListReq struct {
	Path     string `json:"path"`
	Password string `json:"password,omitempty"` // 目录密码（可选）
	Page     int    `json:"page"`
	PerPage  int    `json:"per_page"`
	Refresh  bool   `json:"refresh"`
}

// openListListData OpenList list 响应数据
type openListListData struct {
	Content []openListItem `json:"content"`
	Total   int            `json:"total"`
}

// openListItem OpenList 文件/目录项
type openListItem struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Modified string `json:"modified"` // RFC3339Nano 格式
}

// listOpenList 递归列出 OpenList 目录（使用 BFS）
func (c *Client) listOpenList(ctx context.Context, root string, recursive bool) ([]types.RemoteFile, error) {
	var results []types.RemoteFile

	// 使用 BFS 队列遍历目录树
	queue := []string{cleanRemotePath(root)}

	for len(queue) > 0 {
		// 检查 context 取消
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// 取出队头目录
		dir := queue[0]
		queue = queue[1:]

		// 列出当前目录
		items, err := c.listOpenListOnce(ctx, dir)
		if err != nil {
			return nil, fmt.Errorf("list directory %s: %w", dir, err)
		}

		// 处理每个项目
		for _, item := range items {
			if item.IsDir {
				// 递归模式：将子目录加入队列
				if recursive {
					queue = append(queue, item.Path)
				}
			} else {
				// 只将文件加入结果
				results = append(results, item)
			}
		}
	}

	c.logger.Info("OpenList 目录列出完成",
		zap.String("root", root),
		zap.Bool("recursive", recursive),
		zap.Int("count", len(results)))

	return results, nil
}

// listOpenListOnce 列出 OpenList 单个目录（返回所有文件和目录）
func (c *Client) listOpenListOnce(ctx context.Context, listPath string) ([]types.RemoteFile, error) {
	// 确保有 token（如果需要认证）
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}

	// 构建请求
	reqBody := openListListReq{
		Path:     cleanRemotePath(listPath),
		Password: c.config.Password, // 可能是目录密码
		Page:     1,
		PerPage:  0,     // 0 表示不分页，返回所有结果
		Refresh:  false, // 不强制刷新缓存
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求
	endpoint := c.buildAPIPath("/api/fs/list")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 添加认证 token
	if c.token != "" {
		req.Header.Set("Authorization", c.token)
	}

	// 执行请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// 统一处理非 2xx 响应
	if resp.StatusCode != http.StatusOK {
		// 认证失败：清空 token 以便下次重新登录
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			c.clearToken()
			return nil, ErrUnauthorized
		}
		// 其他错误：读取错误消息
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyLen))
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// 解析响应
	var out openListResponse[openListListData]
	if err := decodeJSON(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if out.Code != http.StatusOK {
		return nil, fmt.Errorf("api error: code=%d message=%s", out.Code, out.Message)
	}

	// 转换为 RemoteFile
	var results []types.RemoteFile
	for _, item := range out.Data.Content {
		fullPath := joinRemotePath(listPath, item.Name)
		modTime := parseTime(item.Modified)

		// 添加到结果（包括文件和目录）
		results = append(results, types.RemoteFile{
			Path:    fullPath,
			Name:    item.Name,
			Size:    item.Size,
			ModTime: modTime,
			IsDir:   item.IsDir,
		})
	}

	return results, nil
}

// ensureToken 确保有有效的认证 token（如果配置了用户名）
func (c *Client) ensureToken(ctx context.Context) error {
	// 未配置用户名，认为无需登录
	if strings.TrimSpace(c.config.Username) == "" {
		return nil
	}

	// 已有 token，直接返回
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" {
		return nil
	}

	// 执行登录
	c.logger.Info("OpenList 登录认证", zap.String("username", c.config.Username))

	loginBody := map[string]string{
		"username": c.config.Username,
		"password": c.config.Password,
		"otp_code": "", // 暂不支持 OTP
	}
	payload, err := json.Marshal(loginBody)
	if err != nil {
		return fmt.Errorf("marshal login request: %w", err)
	}

	// 创建登录请求
	endpoint := c.buildAPIPath("/api/auth/login")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 执行登录请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	// 统一处理非 2xx 响应
	if resp.StatusCode != http.StatusOK {
		// 认证失败
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			c.logger.Error("OpenList 登录失败：认证失败")
			return ErrUnauthorized
		}
		// 其他错误
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyLen))
		return fmt.Errorf("login http status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// 解析登录响应
	var out openListResponse[struct {
		Token string `json:"token"`
	}]
	if err := decodeJSON(resp.Body, &out); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	if out.Code != http.StatusOK || strings.TrimSpace(out.Data.Token) == "" {
		return fmt.Errorf("login failed: code=%d message=%s", out.Code, out.Message)
	}

	// 保存 token
	c.token = out.Data.Token
	c.logger.Info("OpenList 登录成功")

	return nil
}

// clearToken 清空认证 token（用于 401/403 时重新登录）
func (c *Client) clearToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
}

// buildHTTPStreamURL 构建 HTTP 下载链接
// 格式：http://api_url/d/path/to/file
func (c *Client) buildHTTPStreamURL(filePath string) (string, error) {
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
func (c *Client) buildMountStreamPath(filePath string) (string, error) {
	// 去除开头的斜杠，转换为本地路径分隔符
	cleanPath := strings.TrimPrefix(cleanRemotePath(filePath), "/")
	localPath := filepath.FromSlash(cleanPath)

	// 拼接挂载点和文件路径
	return filepath.Join(c.config.MountPath, localPath), nil
}

// buildAPIPath 构建完整的 API 路径
func (c *Client) buildAPIPath(endpoint string) string {
	result := *c.baseURL
	result.Path = joinURLPath(result.Path, endpoint)
	return result.String()
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

// joinRemotePath 拼接远程路径（使用 Unix 路径分隔符）
func joinRemotePath(parent, name string) string {
	parent = cleanRemotePath(parent)
	if parent == "/" {
		return path.Join("/", name)
	}
	return path.Join(parent, name)
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

// parseTime 解析 RFC3339Nano 时间（失败时返回零值）
func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		// 时间解析失败不影响功能，返回零值
		return time.Time{}
	}
	return t
}

// decodeJSON 解码 JSON 响应（不允许未知字段）
func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	// 注释掉 DisallowUnknownFields，因为 OpenList 可能返回未定义字段
	// dec.DisallowUnknownFields()
	return dec.Decode(v)
}
