// Package mediaserver 实现媒体服务器客户端
package mediaserver

import (
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	defaultTimeout     = 10 * time.Second
	maxResponseBodyLen = 4096 // 4KB，用于错误消息读取
)

// serverAdapter 定义服务器特定的行为
type serverAdapter interface {
	endpointPath(path string) string
	setAuthHeader(req *http.Request, apiKey string)
}

// newAdapter 创建服务器适配器
func newAdapter(t Type) (serverAdapter, error) {
	switch t {
	case TypeEmby:
		return embyAdapter{}, nil
	case TypeJellyfin:
		return jellyfinAdapter{}, nil
	default:
		return nil, fmt.Errorf("mediaserver: unsupported server type: %s", t)
	}
}

// Client 媒体服务器客户端实现
type clientImpl struct {
	serverType Type
	baseURL    *url.URL
	apiKey     string
	timeout    time.Duration
	httpClient *http.Client
	logger     *zap.Logger
	adapter    serverAdapter
}

// Option 配置Client的可选参数
type Option func(*clientImpl)

// WithHTTPClient 注入自定义HTTP客户端（用于测试）
// 注意：如果自定义客户端没有设置超时，将使用配置的超时时间
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientImpl) {
		if client != nil {
			// 如果自定义客户端没有设置超时，使用配置的超时
			if client.Timeout == 0 {
				client.Timeout = c.timeout
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

// NewClient 创建媒体服务器客户端
func NewClient(config Config, opts ...Option) (Client, error) {
	// 验证服务器类型
	if !config.Type.IsValid() {
		return nil, fmt.Errorf("mediaserver: invalid server type: %s", config.Type)
	}

	// 验证Base URL
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("mediaserver: base_url is required")
	}
	parsedURL, err := url.Parse(strings.TrimSpace(config.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("mediaserver: parse base_url: %w", err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("mediaserver: invalid base_url (missing scheme or host): %s", config.BaseURL)
	}
	// 统一去除 Path 的尾部斜杠
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")

	// 验证API Key
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("mediaserver: api_key is required")
	}

	// 设置超时
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	// 创建服务器适配器
	adapter, err := newAdapter(config.Type)
	if err != nil {
		return nil, err
	}

	// 创建客户端实例
	client := &clientImpl{
		serverType: config.Type,
		baseURL:    parsedURL,
		apiKey:     strings.TrimSpace(config.APIKey),
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger.L(), // 默认使用全局logger
		adapter:    adapter,
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
		client.logger = logger.L()
	}

	return client, nil
}

// TestConnection 测试连接
func (c *clientImpl) TestConnection(ctx context.Context) error {
	endpoint := c.endpointPath("/System/Info")
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("mediaserver: test connection: %w", err)
	}

	c.logger.Info("测试媒体服务器连接",
		zap.String("type", c.serverType.String()),
		zap.String("endpoint", endpoint))

	if err := c.do(req, http.StatusOK); err != nil {
		c.logger.Error("媒体服务器连接失败",
			zap.String("type", c.serverType.String()),
			zap.Error(err))
		return fmt.Errorf("mediaserver: test connection failed: %w", err)
	}

	c.logger.Info("媒体服务器连接成功",
		zap.String("type", c.serverType.String()))
	return nil
}

// Scan 触发媒体库扫描（fire-and-forget，不等待扫描完成）
func (c *clientImpl) Scan(ctx context.Context, libraryPath string) error {
	if strings.TrimSpace(libraryPath) == "" {
		return fmt.Errorf("mediaserver: scan: libraryPath cannot be empty")
	}

	// 构建扫描请求
	body := scanRequest{
		Updates: []scanUpdate{
			{
				Path:       libraryPath,
				UpdateType: "Created",
			},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("mediaserver: scan: marshal request: %w", err)
	}

	endpoint := c.endpointPath("/Library/Media/Updated")
	req, err := c.newRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("mediaserver: scan: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Info("触发媒体库刷新",
		zap.String("type", c.serverType.String()),
		zap.String("path", libraryPath))

	// 接受 200 OK 或 204 No Content
	if err := c.do(req, http.StatusOK, http.StatusNoContent); err != nil {
		c.logger.Error("媒体库刷新失败",
			zap.String("type", c.serverType.String()),
			zap.String("path", libraryPath),
			zap.Error(err))
		return fmt.Errorf("mediaserver: scan failed: %w", err)
	}

	c.logger.Info("媒体库刷新请求已发送",
		zap.String("type", c.serverType.String()),
		zap.String("path", libraryPath))
	return nil
}

// scanRequest 扫描请求体
type scanRequest struct {
	Updates []scanUpdate `json:"Updates"`
}

// scanUpdate 扫描更新项
type scanUpdate struct {
	Path       string `json:"Path"`
	UpdateType string `json:"UpdateType"` // "Created", "Modified", "Deleted"
}

// endpointPath 根据服务器类型生成完整的API路径
// 委托给服务器适配器
func (c *clientImpl) endpointPath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.adapter.endpointPath(path)
}

// newRequest 创建HTTP请求
func (c *clientImpl) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("mediaserver: endpoint cannot be empty")
	}

	// 防御 nil context
	if ctx == nil {
		ctx = context.Background()
	}

	// 构建完整URL
	urlStr, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, fmt.Errorf("mediaserver: create request: %w", err)
	}

	// 设置认证头
	c.setAuthHeader(req)

	return req, nil
}

// buildURL 构建完整的URL（保留 baseURL 的所有字段）
func (c *clientImpl) buildURL(endpoint string) (string, error) {
	// 解析 endpoint（可能包含 query 参数）
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("mediaserver: parse endpoint: %w", err)
	}

	// 复制 baseURL（保留 User/Query/Fragment 等字段）
	result := *c.baseURL

	// 拼接 Path（保留 baseURL 的路径前缀）
	result.Path = c.baseURL.Path + endpointURL.Path

	// 合并 Query 参数（baseURL 的 query + endpoint 的 query）
	if endpointURL.RawQuery != "" {
		if result.RawQuery == "" {
			result.RawQuery = endpointURL.RawQuery
		} else {
			// 合并两个 query（endpoint 的参数优先）
			baseQuery := c.baseURL.Query()
			endpointQuery := endpointURL.Query()
			for k, v := range endpointQuery {
				baseQuery[k] = v
			}
			result.RawQuery = baseQuery.Encode()
		}
	}

	// 保留 endpoint 的 Fragment（如果有）
	if endpointURL.Fragment != "" {
		result.Fragment = endpointURL.Fragment
	}

	return result.String(), nil
}

// setAuthHeader 设置认证头（委托给服务器适配器）
func (c *clientImpl) setAuthHeader(req *http.Request) {
	c.adapter.setAuthHeader(req, c.apiKey)
}

// do 执行HTTP请求并验证响应状态码
func (c *clientImpl) do(req *http.Request, expectedStatuses ...int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mediaserver: http request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码是否符合预期
	if isExpectedStatus(resp.StatusCode, expectedStatuses) {
		return nil
	}

	// 读取错误响应体（限制大小）
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyLen))
	bodyStr := strings.TrimSpace(string(body))
	if bodyStr == "" {
		bodyStr = "<empty>"
	}

	return fmt.Errorf("mediaserver: unexpected status code %d: %s", resp.StatusCode, bodyStr)
}

// isExpectedStatus 检查状态码是否在预期列表中
func isExpectedStatus(code int, expected []int) bool {
	for _, v := range expected {
		if code == v {
			return true
		}
	}
	return false
}

// embyAdapter Emby服务器适配器
type embyAdapter struct{}

func (embyAdapter) endpointPath(path string) string {
	return path
}

func (embyAdapter) setAuthHeader(req *http.Request, apiKey string) {
	req.Header.Set("X-Emby-Token", apiKey)
}

// jellyfinAdapter Jellyfin服务器适配器
type jellyfinAdapter struct{}

func (jellyfinAdapter) endpointPath(path string) string {
	return path
}

func (jellyfinAdapter) setAuthHeader(req *http.Request, apiKey string) {
	if strings.TrimSpace(apiKey) == "" {
		return
	}
	// Jellyfin支持两种认证方式，为了兼容性同时设置
	req.Header.Set("X-MediaBrowser-Token", apiKey)
	req.Header.Set("Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, apiKey))
}
