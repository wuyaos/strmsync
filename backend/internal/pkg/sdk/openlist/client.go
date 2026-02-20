// Package openlist provides OpenList HTTP API SDK client implementation.
//
// This package encapsulates all HTTP communication with OpenList server,
// including connection management, authentication, and API method calls.
//
// The client supports:
//   - HTTP/HTTPS protocol
//   - Bearer token authentication
//   - Automatic connection management
//   - Context-based timeout control
package openlist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

const maxResponseBodyLen = 4096 // 4KB，用于错误消息读取

// Client OpenList HTTP API 客户端
//
// 封装与 OpenList 服务器的 HTTP 通信，提供连接管理、认证和 API 调用功能。
type Client struct {
	baseURL             *url.URL
	httpClient          *http.Client
	username            string
	password            string
	downloadPathPattern string

	// token 管理
	mu    sync.Mutex
	token string
}

// Config OpenList客户端配置
type Config struct {
	BaseURL             string        // 服务器基础URL（如 "http://127.0.0.1:5244"）
	Username            string        // 登录用户名（可选）
	Password            string        // 登录密码
	Timeout             time.Duration // HTTP请求超时时间
	HTTPClient          *http.Client  // 可选的自定义HTTP客户端
	DownloadPathPattern string        // 下载路径模板，默认为"/d{path}"
}

// NewClient 创建新的 OpenList 客户端
//
// 参数：
//   - cfg: 客户端配置
//
// 返回：
//   - *Client: 新创建的客户端实例
//   - error: 配置无效时返回错误
func NewClient(cfg Config) (*Client, error) {
	// 解析BaseURL
	baseURL, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("openlist: invalid base_url: %s", cfg.BaseURL)
	}

	// 设置HTTP客户端
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	// 设置下载路径模板
	downloadPattern := strings.TrimSpace(cfg.DownloadPathPattern)
	if downloadPattern == "" {
		downloadPattern = "/d{path}" // 默认使用 /d{path} 格式
	}

	return &Client{
		baseURL:             baseURL,
		httpClient:          httpClient,
		username:            strings.TrimSpace(cfg.Username),
		password:            cfg.Password,
		downloadPathPattern: downloadPattern,
	}, nil
}

// FileItem 表示 OpenList 返回的文件/目录项
type FileItem struct {
	Name     string    // 文件/目录名称
	Size     int64     // 文件大小（字节）
	IsDir    bool      // 是否为目录
	Modified time.Time // 修改时间
}

// List 列出目录内容
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//   - path: 目录路径（Unix格式）
//
// 返回：
//   - []FileItem: 文件/目录列表
//   - error: 调用失败时返回错误
func (c *Client) List(ctx context.Context, listPath string) ([]FileItem, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 确保有 token（如果需要认证）
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}

	// 构建请求
	reqBody := listRequest{
		Path:     cleanPath(listPath),
		Password: c.password, // 可能是目录密码
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
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	if token != "" {
		req.Header.Set("Authorization", token)
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
			return nil, fmt.Errorf("openlist: unauthorized")
		}
		// 其他错误：读取错误消息
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyLen))
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// 解析响应
	var out response[listData]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if out.Code != http.StatusOK {
		return nil, fmt.Errorf("api error: code=%d message=%s", out.Code, out.Message)
	}

	// 转换为 FileItem
	var results []FileItem
	for _, item := range out.Data.Content {
		results = append(results, FileItem{
			Name:     item.Name,
			Size:     item.Size,
			IsDir:    item.IsDir,
			Modified: parseTime(item.Modified),
		})
	}

	return results, nil
}

// Download 下载文件内容到writer
//
// 参数：
//   - ctx: 上下文（可以为 nil）
//   - filePath: 文件路径（Unix格式）
//   - w: 目标writer
//
// 返回：
//   - error: 下载失败时返回错误
func (c *Client) Download(ctx context.Context, filePath string, w io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("openlist: file path cannot be empty")
	}

	// 确保有 token（如果需要认证）
	if err := c.ensureToken(ctx); err != nil {
		return err
	}

	// 构建下载URL（使用配置的路径模板）
	cleanedPath := cleanPath(filePath)
	downloadPath := strings.ReplaceAll(c.downloadPathPattern, "{path}", cleanedPath)
	downloadURL := c.buildAPIPath(downloadPath)

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}

	// 添加认证 token
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	// 执行请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		// 认证失败：清空 token
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			c.clearToken()
			return fmt.Errorf("openlist: download unauthorized")
		}
		// 其他错误：读取错误消息
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyLen))
		return fmt.Errorf("download status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// 将响应写入writer
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy download body: %w", err)
	}

	return nil
}

// ensureToken 确保有有效的认证 token
func (c *Client) ensureToken(ctx context.Context) error {
	// 未配置用户名，认为无需登录
	if c.username == "" {
		return nil
	}

	// 已有 token，直接返回
	c.mu.Lock()
	if c.token != "" {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// 执行登录
	loginBody := map[string]string{
		"username": c.username,
		"password": c.password,
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
			return fmt.Errorf("openlist: login unauthorized")
		}
		// 其他错误
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyLen))
		return fmt.Errorf("login http status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// 解析登录响应
	var out response[struct {
		Token string `json:"token"`
	}]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	if out.Code != http.StatusOK || strings.TrimSpace(out.Data.Token) == "" {
		return fmt.Errorf("login failed: code=%d message=%s", out.Code, out.Message)
	}

	// 保存 token
	c.mu.Lock()
	c.token = out.Data.Token
	c.mu.Unlock()

	return nil
}

// clearToken 清空认证 token
func (c *Client) clearToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
}

// buildAPIPath 构建完整的 API 路径
func (c *Client) buildAPIPath(endpoint string) string {
	result := *c.baseURL
	result.Path = joinURLPath(result.Path, endpoint)
	return result.String()
}

// ---------- Internal Types ----------

// response OpenList API 响应格式
type response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// listRequest OpenList /api/fs/list 请求
type listRequest struct {
	Path     string `json:"path"`
	Password string `json:"password,omitempty"` // 目录密码（可选）
	Page     int    `json:"page"`
	PerPage  int    `json:"per_page"`
	Refresh  bool   `json:"refresh"`
}

// listData OpenList list 响应数据
type listData struct {
	Content []listItem `json:"content"`
	Total   int        `json:"total"`
}

// listItem OpenList 文件/目录项
type listItem struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Modified string `json:"modified"` // RFC3339Nano 格式
}

// ---------- Utility Functions ----------

// cleanPath 清理路径
func cleanPath(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return path.Clean(trimmed)
}

// joinURLPath 拼接URL路径
func joinURLPath(basePath, endpoint string) string {
	if basePath == "" {
		basePath = "/"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return strings.TrimRight(basePath, "/") + endpoint
}

// parseTime 解析 RFC3339Nano 时间（失败时返回零值）
func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}
