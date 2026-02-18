// Package filesystem 实现OpenList文件系统客户端
package filesystem

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

	"github.com/strmsync/strmsync/syncengine"
	"go.uber.org/zap"
)

const openListMaxResponseBodyLen = 4096 // 4KB，用于错误消息读取

// openListProvider OpenList文件系统实现
type openListProvider struct {
	config     Config
	baseURL    *url.URL
	httpClient *http.Client
	logger     *zap.Logger

	// token 管理
	mu    sync.Mutex
	token string
}

// newOpenListProvider 创建OpenList provider
func newOpenListProvider(c *clientImpl) (provider, error) {
	return &openListProvider{
		config:     c.config,
		baseURL:    c.baseURL,
		httpClient: c.httpClient,
		logger:     c.logger,
	}, nil
}

// List 列出目录内容
func (p *openListProvider) List(ctx context.Context, listPath string, recursive bool, maxDepth int) ([]RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(listPath) == "" {
		listPath = "/"
	}
	return p.listOpenList(ctx, listPath, recursive, maxDepth)
}

// Watch 监控目录变化（OpenList不支持）
func (p *openListProvider) Watch(ctx context.Context, path string) (<-chan FileEvent, error) {
	return nil, ErrNotSupported
}

// TestConnection 测试连接
func (p *openListProvider) TestConnection(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.logger.Info("测试OpenList连接", zap.String("type", TypeOpenList.String()))
	_, err := p.listOpenListOnce(ctx, "/")
	if err != nil {
		p.logger.Error("OpenList连接失败", zap.Error(err))
		return fmt.Errorf("filesystem: test connection failed: %w", err)
	}
	p.logger.Info("OpenList连接成功")
	return nil
}

// Stat 获取单个路径的元数据
//
// 实现说明：
// - OpenList 没有专用的 Stat API，使用降级策略
// - 列出父目录内容，然后匹配目标文件/目录
// - 路径 "/" 特殊处理为根目录
//
// 参数：
//   - ctx: 上下文，用于取消和超时控制
//   - targetPath: 要查询的远程路径（Unix 格式）
//
// 返回：
//   - RemoteFile: 文件/目录的元数据
//   - error: 查询失败或路径不存在时返回错误
func (p *openListProvider) Stat(ctx context.Context, targetPath string) (RemoteFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cleanPath := cleanRemotePath(targetPath)
	p.logger.Debug("OpenList Stat",
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

	// 列出父目录
	items, err := p.listOpenListOnce(ctx, parentPath)
	if err != nil {
		return RemoteFile{}, fmt.Errorf("openlist: stat 列出父目录 %s 失败: %w", parentPath, err)
	}

	// 查找匹配项（按 Name 或 Path 匹配）
	for _, item := range items {
		if item.Name == baseName || item.Path == cleanPath {
			p.logger.Debug("OpenList Stat 完成",
				zap.String("path", item.Path),
				zap.Bool("is_dir", item.IsDir),
				zap.Int64("size", item.Size))
			return item, nil
		}
	}

	return RemoteFile{}, fmt.Errorf("openlist: 路径不存在: %s", cleanPath)
}

// BuildStrmInfo 构建结构化的 STRM 信息
//
// 实现说明：
// - 生成基于 HTTP/HTTPS 的流媒体 URL
// - OpenList 使用 Web API 访问文件
// - scheme 从 baseURL 配置中获取（支持 http 和 https）
// - 返回的 StrmInfo 包含 RawURL、BaseURL 和 Path 字段
//
// 参数：
//   - ctx: 上下文（当前未使用，保留用于未来扩展）
//   - req: BuildStrmRequest 包含 ServerID、RemotePath 和可选的 RemoteMeta
//
// 返回：
//   - StrmInfo: 结构化的 STRM 元数据
//   - error: 输入无效或构建失败时返回错误
func (p *openListProvider) BuildStrmInfo(ctx context.Context, req syncengine.BuildStrmRequest) (syncengine.StrmInfo, error) {
	_ = ctx // 保留用于未来的取消或追踪

	// 验证输入
	if strings.TrimSpace(req.RemotePath) == "" {
		return syncengine.StrmInfo{}, fmt.Errorf("openlist: remote path 不能为空: %w", syncengine.ErrInvalidInput)
	}

	cleanPath := cleanRemotePath(req.RemotePath)

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
		zap.String("remote_path", cleanPath),
		zap.String("scheme", scheme),
		zap.String("raw_url", rawURL))

	return syncengine.StrmInfo{
		RawURL:  rawURL,
		BaseURL: &url.URL{Scheme: scheme, Host: host},
		Path:    cleanPath,
	}, nil
}

// ---------- OpenList 内部实现 ----------

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

// listOpenList 递归列出 OpenList 目录（使用 BFS，支持深度限制）
func (p *openListProvider) listOpenList(ctx context.Context, root string, recursive bool, maxDepth int) ([]RemoteFile, error) {
	var results []RemoteFile

	// 使用 BFS 队列遍历目录树，队列中存储路径和当前深度
	type queueItem struct {
		path  string
		depth int
	}
	queue := []queueItem{{path: cleanRemotePath(root), depth: 0}}

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
		items, err := p.listOpenListOnce(ctx, dir)
		if err != nil {
			return nil, fmt.Errorf("list directory %s: %w", dir, err)
		}

		// 处理每个项目
		for _, remoteFile := range items {
			// 将所有项目（文件和目录）加入结果
			results = append(results, remoteFile)

			// 递归模式：将子目录加入队列（深度控制）
			// 只有当子目录的内容深度(item.depth+2)不超过maxDepth时才入队
			// 即：item.depth + 1 < maxDepth
			if remoteFile.IsDir && recursive && item.depth+1 < maxDepth {
				queue = append(queue, queueItem{path: remoteFile.Path, depth: item.depth + 1})
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

// listOpenListOnce 列出 OpenList 单个目录
func (p *openListProvider) listOpenListOnce(ctx context.Context, listPath string) ([]RemoteFile, error) {
	// 确保有 token（如果需要认证）
	if err := p.ensureToken(ctx); err != nil {
		return nil, err
	}

	// 构建请求
	reqBody := openListListReq{
		Path:     cleanRemotePath(listPath),
		Password: p.config.Password, // 可能是目录密码
		Page:     1,
		PerPage:  0,     // 0 表示不分页，返回所有结果
		Refresh:  false, // 不强制刷新缓存
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建 HTTP 请求
	endpoint := p.buildAPIPath("/api/fs/list")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 添加认证 token
	if p.token != "" {
		req.Header.Set("Authorization", p.token)
	}

	// 执行请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// 统一处理非 2xx 响应
	if resp.StatusCode != http.StatusOK {
		// 认证失败：清空 token 以便下次重新登录
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			p.clearToken()
			return nil, ErrUnauthorized
		}
		// 其他错误：读取错误消息
		body, _ := io.ReadAll(io.LimitReader(resp.Body, openListMaxResponseBodyLen))
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
	var results []RemoteFile
	for _, item := range out.Data.Content {
		fullPath := joinRemotePath(listPath, item.Name)
		modTime := parseTime(item.Modified)

		// 添加到结果（包括文件和目录）
		results = append(results, RemoteFile{
			Path:    fullPath,
			Name:    item.Name,
			Size:    item.Size,
			ModTime: modTime,
			IsDir:   item.IsDir,
		})
	}

	return results, nil
}

// ensureToken 确保有有效的认证 token
func (p *openListProvider) ensureToken(ctx context.Context) error {
	// 未配置用户名，认为无需登录
	if strings.TrimSpace(p.config.Username) == "" {
		return nil
	}

	// 已有 token，直接返回
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" {
		return nil
	}

	// 执行登录
	p.logger.Info("OpenList 登录认证", zap.String("username", p.config.Username))

	loginBody := map[string]string{
		"username": p.config.Username,
		"password": p.config.Password,
		"otp_code": "", // 暂不支持 OTP
	}
	payload, err := json.Marshal(loginBody)
	if err != nil {
		return fmt.Errorf("marshal login request: %w", err)
	}

	// 创建登录请求
	endpoint := p.buildAPIPath("/api/auth/login")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 执行登录请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	// 统一处理非 2xx 响应
	if resp.StatusCode != http.StatusOK {
		// 认证失败
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			p.logger.Error("OpenList 登录失败：认证失败")
			return ErrUnauthorized
		}
		// 其他错误
		body, _ := io.ReadAll(io.LimitReader(resp.Body, openListMaxResponseBodyLen))
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
	p.token = out.Data.Token
	p.logger.Info("OpenList 登录成功")

	return nil
}

// clearToken 清空认证 token
func (p *openListProvider) clearToken() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.token = ""
}

// buildAPIPath 构建完整的 API 路径
func (p *openListProvider) buildAPIPath(endpoint string) string {
	result := *p.baseURL
	result.Path = joinURLPath(result.Path, endpoint)
	return result.String()
}

// joinRemotePath 拼接远程路径（使用 Unix 路径分隔符）
func joinRemotePath(parent, name string) string {
	parent = cleanRemotePath(parent)
	if parent == "/" {
		return path.Join("/", name)
	}
	return path.Join(parent, name)
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

// decodeJSON 解码 JSON 响应
func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	return dec.Decode(v)
}
