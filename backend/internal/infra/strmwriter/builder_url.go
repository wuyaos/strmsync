// Package strmwriter 提供HTTP URL格式的STRM内容生成
package strmwriter

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// URLBuilder 生成HTTP URL格式的STRM内容
//
// 适用场景：
// - 通过HTTP API访问数据服务器（如OpenList、CloudDrive2）
// - 媒体服务器通过网络请求获取文件
// - 支持远程访问和跨网络播放
//
// URL构建逻辑：
// 1. 解析BaseURL为 *url.URL
// 2. 保留BaseURL的scheme、host、原有path
// 3. 追加URLPathPrefix（如 "/d"）
// 4. 追加清理后的远程路径
//
// 示例：
//
//	builder, _ := NewURLBuilder(BuildConfig{
//	    Format:        StrmFormatHTTP,
//	    BaseURL:       "http://192.168.1.100:19798",
//	    URLPathPrefix: "/d",
//	})
//	content, _ := builder.Build(ctx, BuildRequest{
//	    RemotePath: "/本地存储/电影/阿凡达.mp4",
//	    ServerID:   1,
//	})
//	// 结果: "http://192.168.1.100:19798/d/本地存储/电影/阿凡达.mp4"
//
// 路径处理说明：
// - 中文和特殊字符保持原样（不进行URL编码）
// - 空格保持原样（不转换为%20）
// - 这是为了兼容现有逻辑，未来可通过配置项支持URL编码
type URLBuilder struct {
	baseURL    *url.URL // 解析后的基础URL
	pathPrefix string   // 路径前缀（如 "/d"）
}

// NewURLBuilder 创建HTTP URL格式生成器
//
// 参数：
//   - cfg: 构建配置，必须包含 BaseURL
//
// 返回：
//   - *URLBuilder: 生成器实例
//   - error: 配置无效时返回错误
//
// 验证：
// - 格式必须是 StrmFormatHTTP
// - BaseURL 必须是有效的URL（包含scheme和host）
// - URLPathPrefix 默认为 "/d"（如果未指定）
//
// 错误情况：
// - BaseURL 解析失败
// - BaseURL 缺少 scheme 或 host
func NewURLBuilder(cfg BuildConfig) (*URLBuilder, error) {
	// 确保格式正确
	cfg.Format = StrmFormatHTTP

	// 应用默认值并验证
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("strmwriter: URLBuilder配置无效: %w", err)
	}

	// 解析BaseURL
	baseURL := strings.TrimSpace(cfg.BaseURL)
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("strmwriter: 无效的base_url (%s): %w", baseURL, err)
	}

	// 验证必需的URL组件
	if u.Scheme == "" {
		return nil, fmt.Errorf("strmwriter: base_url缺少scheme (http/https): %s", baseURL)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("strmwriter: base_url缺少host: %s", baseURL)
	}

	// 清理BaseURL的path（去除尾部斜杠）
	u.Path = strings.TrimRight(u.Path, "/")

	// 清除 RawPath 以避免与后续 Path 修改不一致
	// 如果 BaseURL 包含转义路径，解析后的 u.Path 已经是解码后的值
	// 后续拼接操作直接修改 u.Path，因此需要清除 RawPath
	u.RawPath = ""

	return &URLBuilder{
		baseURL:    u,
		pathPrefix: cfg.URLPathPrefix,
	}, nil
}

// Build 实现 ContentBuilder 接口
//
// 生成逻辑：
// 1. 验证请求参数
// 2. 清理远程路径
// 3. 复制BaseURL（避免修改原始值）
// 4. 拼接路径：baseURL.Path + pathPrefix + remotePath
// 5. 返回完整URL字符串
//
// 参数：
//   - ctx: 上下文（检查取消，nil会自动转为Background）
//   - req: 生成请求
//
// 返回：
//   - string: 完整的HTTP URL
//   - error: 生成失败时返回错误
//
// 注意：
// - 路径拼接使用字符串操作，不对特殊字符进行编码
// - 这是为了保持与现有逻辑一致
// - 中文路径、空格等保持原样
func (b *URLBuilder) Build(ctx context.Context, req BuildRequest) (string, error) {
	// 防御性编程：nil context 自动转为 Background
	if ctx == nil {
		ctx = context.Background()
	}

	// 检查 context 取消
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("strmwriter: context已取消: %w", err)
	}

	// 验证请求
	if err := req.Validate(); err != nil {
		return "", fmt.Errorf("strmwriter: URLBuilder请求验证失败: %w", err)
	}

	// 清理远程路径
	cleanPath := cleanRemotePath(req.RemotePath)

	// 复制BaseURL（避免修改原始值）
	out := *b.baseURL

	// 拼接路径：
	// 1. BaseURL的原有path（如 "/api"）
	// 2. PathPrefix（如 "/d"）
	// 3. 远程路径（如 "/Movies/Avatar.mp4"）
	out.Path = joinURLPath(out.Path, b.pathPrefix)
	out.Path = joinURLPath(out.Path, cleanPath)

	// 返回完整URL
	return out.String(), nil
}

// joinURLPath 安全地拼接URL路径
//
// 拼接规则：
// - 确保basePath以斜杠结尾
// - 确保endpoint以斜杠开头
// - 去除重复的斜杠
//
// 参数：
//   - basePath: 基础路径（可以为空）
//   - endpoint: 要追加的路径
//
// 返回：
//   - string: 拼接后的路径
//
// 示例：
//   - joinURLPath("", "/d") → "/d"
//   - joinURLPath("/api", "/d") → "/api/d"
//   - joinURLPath("/api/", "d") → "/api/d"
//   - joinURLPath("/api", "d") → "/api/d"
func joinURLPath(basePath, endpoint string) string {
	// 基础路径为空时，使用根路径
	if basePath == "" {
		basePath = "/"
	}

	// 确保endpoint以斜杠开头
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	// 去除basePath的尾部斜杠，避免重复
	basePath = strings.TrimRight(basePath, "/")

	// 拼接路径
	return basePath + endpoint
}
