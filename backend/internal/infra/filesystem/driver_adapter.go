// Package filesystem 文件系统数据访问和驱动适配
//
// 本文件提供适配器层，将 filesystem.Client（数据访问层）适配到
// syncengine.Driver 接口（面向同步引擎的驱动层），实现：
// 1. 向后兼容：不修改现有 Client 实现
// 2. 类型转换：RemoteFile -> syncengine.RemoteEntry
// 3. 接口扩展：提供 Stat, BuildStrmInfo, CompareStrm 等新方法
// 4. 能力声明：根据驱动类型返回 DriverCapability
package filesystem

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/strmsync/strmsync/internal/engine"
)

// DriverAdapter 将 filesystem.Client 桥接到 syncengine.Driver
//
// 设计要点：
// - 组合而非继承：持有 Client 实例
// - 类型断言：对于可选方法（如 Stat），通过类型断言检测是否支持
// - 降级策略：不支持的方法可以降级实现（如 Stat 降级为 Scan+过滤）
// - 能力声明：根据 driver type 返回不同的 Capability
type Adapter struct {
	client Client
	typ    syncengine.DriverType
}

// NewAdapter 创建一个新的适配器并进行基本验证
//
// 参数：
//   - client: filesystem.Client 实例（不能为 nil）
//   - typ: 驱动类型（必须是有效的 DriverType）
//
// 返回：
//   - *Adapter: 适配器实例
//   - error: 参数无效时返回错误
func NewAdapter(client Client, typ syncengine.DriverType) (*Adapter, error) {
	if client == nil {
		return nil, fmt.Errorf("filesystem: client 不能为 nil: %w", syncengine.ErrInvalidInput)
	}
	if !typ.IsValid() {
		return nil, fmt.Errorf("filesystem: 无效的驱动类型 %q: %w", typ, syncengine.ErrInvalidInput)
	}
	return &Adapter{client: client, typ: typ}, nil
}

// Type 返回驱动类型标识
func (a *Adapter) Type() syncengine.DriverType {
	return a.typ
}

// Capabilities 返回驱动支持的功能特性
//
// 根据不同的驱动类型返回不同的能力声明：
//
// CloudDrive2:
//   - StrmHTTP: true（支持HTTP流媒体）
//   - Watch, PickCode, SignURL: false（当前未实现）
//
// OpenList:
//   - StrmHTTP: true
//   - Watch, PickCode, SignURL: false
//
// Local:
//   - StrmMount: true（使用本地挂载路径）
//   - Watch: false（当前未实现，未来可通过fsnotify支持）
func (a *Adapter) Capabilities() syncengine.DriverCapability {
	switch a.typ {
	case syncengine.DriverCloudDrive2:
		return syncengine.DriverCapability{
			Watch:     false, // filesystem CloudDrive2 provider 当前不支持 Watch
			StrmHTTP:  true,
			StrmMount: false,
			PickCode:  false, // 未来可扩展支持
			SignURL:   false, // 未来可扩展支持
		}
	case syncengine.DriverOpenList:
		return syncengine.DriverCapability{
			Watch:     false,
			StrmHTTP:  true,
			StrmMount: false,
			PickCode:  false,
			SignURL:   false,
		}
	case syncengine.DriverLocal:
		return syncengine.DriverCapability{
			Watch:     false, // 未来可通过 fsnotify 支持
			StrmHTTP:  false,
			StrmMount: true,
			PickCode:  false,
			SignURL:   false,
		}
	default:
		return syncengine.DriverCapability{}
	}
}

// Scan 使用 filesystem.Client 流式扫描远程文件
//
// 实现说明：
// - 直接调用 filesystem.Client.Scan
// - 将 filesystem.RemoteFile 转换为 syncengine.RemoteEntry
// - 传递 opt.Recursive 和 opt.MaxDepth 参数
func (a *Adapter) Scan(ctx context.Context, listPath string, opt syncengine.ListOptions) (<-chan syncengine.RemoteEntry, <-chan error) {
	normalizedPath := listPath
	if a.typ == syncengine.DriverLocal {
		var err error
		normalizedPath, err = normalizeLocalListPath(a.client, listPath)
		if err != nil {
			errCh := make(chan error, 1)
			entryCh := make(chan syncengine.RemoteEntry)
			errCh <- err
			close(entryCh)
			close(errCh)
			return entryCh, errCh
		}
	}

	fileCh, fileErrCh := a.client.Scan(ctx, normalizedPath, opt.Recursive, opt.MaxDepth)
	entryCh := make(chan syncengine.RemoteEntry)
	errCh := make(chan error, 1)

	go func() {
		defer close(entryCh)
		defer close(errCh)

		for fileCh != nil || fileErrCh != nil {
			select {
			case f, ok := <-fileCh:
				if !ok {
					fileCh = nil
					continue
				}
				entryCh <- syncengine.RemoteEntry{
					Path:    f.Path,
					Name:    f.Name,
					Size:    f.Size,
					ModTime: f.ModTime,
					IsDir:   f.IsDir,
				}
			case err, ok := <-fileErrCh:
				if !ok {
					fileErrCh = nil
					continue
				}
				if err != nil {
					errCh <- fmt.Errorf("filesystem: 扫描 %s 失败: %w", listPath, err)
				}
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}
	}()

	return entryCh, errCh
}

// Watch 订阅远程文件变更事件（如果支持）
//
// 实现说明：
// - 检查 Capabilities().Watch，不支持则返回 ErrNotSupported
// - 调用 filesystem.Client.Watch（如果 client 实现了此方法）
// - 启动 goroutine 转换 filesystem.FileEvent -> syncengine.DriverEvent
// - 返回的 channel 在上游 channel 关闭时自动关闭
//
// 注意：
// - filesystem.Client.Watch 签名没有 recursive 参数，opt.Recursive 当前被忽略
// - 调用者负责消费事件，避免 goroutine 泄漏
func (a *Adapter) Watch(ctx context.Context, watchPath string, opt syncengine.WatchOptions) (<-chan syncengine.DriverEvent, error) {
	if !a.Capabilities().Watch {
		return nil, syncengine.ErrNotSupported
	}

	// filesystem.Client.Watch 签名：Watch(ctx, path) (<-chan FileEvent, error)
	fileEventCh, err := a.client.Watch(ctx, watchPath)
	if err != nil {
		return nil, fmt.Errorf("filesystem: 监听 %s 失败: %w", watchPath, err)
	}

	driverEventCh := make(chan syncengine.DriverEvent)
	go func() {
		defer close(driverEventCh)
		for event := range fileEventCh {
			driverEvent := syncengine.DriverEvent{
				Type:    toDriverEventType(event.Type),
				Path:    event.Path,
				AbsPath: event.AbsPath,
				Size:    event.Size,
				ModTime: event.ModTime,
				IsDir:   event.IsDir,
			}
			select {
			case driverEventCh <- driverEvent:
			case <-ctx.Done():
				return
			}
		}
	}()

	return driverEventCh, nil
}

// Stat 返回单个路径的元数据
//
// 实现策略：
// 1. 优先使用：如果 client 实现了 Stat 方法，则直接调用
// 2. 降级方案：使用 Scan(path, false, 0) 并过滤出精确匹配的路径
//
// 降级方案的成本较高，但保证了接口的完整性
func (a *Adapter) Stat(ctx context.Context, targetPath string) (syncengine.RemoteEntry, error) {
	// 策略1：尝试类型断言，检查是否支持 Stat 方法
	type statProvider interface {
		Stat(context.Context, string) (RemoteFile, error)
	}

	if provider, ok := a.client.(statProvider); ok {
		file, err := provider.Stat(ctx, targetPath)
		if err != nil {
			return syncengine.RemoteEntry{}, fmt.Errorf("filesystem: stat %s 失败: %w", targetPath, err)
		}
		return syncengine.RemoteEntry{
			Path:    file.Path,
			Name:    file.Name,
			Size:    file.Size,
			ModTime: file.ModTime,
			IsDir:   file.IsDir,
		}, nil
	}

	// 策略2：降级使用 Scan + 过滤
	// 注意：如果 targetPath 是文件，需要扫描其父目录并匹配文件名
	parentDir := path.Dir(targetPath)
	if parentDir == "" {
		parentDir = "/"
	}
	baseName := path.Base(targetPath)

	fileCh, errCh := a.client.Scan(ctx, parentDir, false, 0)
	for fileCh != nil || errCh != nil {
		select {
		case file, ok := <-fileCh:
			if !ok {
				fileCh = nil
				continue
			}
			if file.Name == baseName || file.Path == targetPath {
				return syncengine.RemoteEntry{
					Path:    file.Path,
					Name:    file.Name,
					Size:    file.Size,
					ModTime: file.ModTime,
					IsDir:   file.IsDir,
				}, nil
			}
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				return syncengine.RemoteEntry{}, fmt.Errorf("filesystem: stat 降级扫描父目录 %s 失败: %w", parentDir, err)
			}
		case <-ctx.Done():
			return syncengine.RemoteEntry{}, ctx.Err()
		}
	}
	return syncengine.RemoteEntry{}, fmt.Errorf("filesystem: 路径 %s 不存在: %w", targetPath, syncengine.ErrInvalidInput)
}

// BuildStrmInfo 构建结构化的 STRM 信息
//
// 实现策略：
// 1. 优先使用：如果 client 实现了 BuildStrmInfo 方法，则直接调用
// 2. 降级方案：使用 BuildStreamURL 获取字符串 URL，然后解析填充结构
//
// 降级方案会尝试从 URL 中提取：
// - BaseURL (scheme + host)
// - Path
// - PickCode (从 query 参数)
// - Sign (从 query 参数)
// - ExpiresAt (从 expires/e 参数解析)
func (a *Adapter) BuildStrmInfo(ctx context.Context, req syncengine.BuildStrmRequest) (syncengine.StrmInfo, error) {
	// 策略1：尝试类型断言，检查是否支持结构化构建
	type strmInfoBuilder interface {
		BuildStrmInfo(context.Context, syncengine.BuildStrmRequest) (syncengine.StrmInfo, error)
	}

	if builder, ok := a.client.(strmInfoBuilder); ok {
		req = normalizeLocalBuildRequest(a.client, a.typ, req)
		info, err := builder.BuildStrmInfo(ctx, req)
		if err != nil {
			return syncengine.StrmInfo{}, fmt.Errorf("filesystem: 构建 strm 信息失败: %w", err)
		}
		return info, nil
	}

	// 策略2：降级使用 BuildStreamURL + 解析
	rawURL, err := a.client.BuildStreamURL(ctx, req.ServerID, req.RemotePath)
	if err != nil {
		return syncengine.StrmInfo{}, fmt.Errorf("filesystem: 构建流媒体 URL 失败: %w", err)
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return syncengine.StrmInfo{}, fmt.Errorf("filesystem: 解析流媒体 URL 失败: %w", err)
	}

	info := syncengine.StrmInfo{
		RawURL:  rawURL,
		BaseURL: &url.URL{Scheme: parsed.Scheme, Host: parsed.Host},
		Path:    parsed.Path,
	}

	query := parsed.Query()
	// 尝试多种参数名（pickcode 或 pick_code）
	info.PickCode = firstNonEmpty(query.Get("pickcode"), query.Get("pick_code"))
	info.Sign = query.Get("sign")
	info.ExpiresAt = parseExpiry(query.Get("expires"), query.Get("e"))

	return info, nil
}

// normalizeLocalListPath 兼容本地驱动的绝对路径输入
// - 若 listPath 位于 mount_path 之下，则转换为相对路径
// - 若 listPath 以 "/" 开头但不是绝对路径（虚拟路径），移除前导 "/"
func normalizeLocalListPath(client Client, listPath string) (string, error) {
	trimmed := strings.TrimSpace(listPath)
	if trimmed == "" || trimmed == "/" {
		return trimmed, nil
	}

	impl, ok := client.(*ClientImpl)
	if !ok {
		if strings.HasPrefix(trimmed, "/") {
			return strings.TrimLeft(trimmed, "/"), nil
		}
		return trimmed, nil
	}

	mountRoot := strings.TrimSpace(impl.Config.MountPath)
	if mountRoot == "" {
		if strings.HasPrefix(trimmed, "/") {
			return strings.TrimLeft(trimmed, "/"), nil
		}
		return trimmed, nil
	}

	if filepath.IsAbs(trimmed) || filepath.VolumeName(trimmed) != "" {
		cleanMount := filepath.Clean(mountRoot)
		cleanPath := filepath.Clean(trimmed)
		if cleanPath == cleanMount {
			return "/", nil
		}
		prefix := cleanMount + string(filepath.Separator)
		if strings.HasPrefix(cleanPath, prefix) {
			rel := strings.TrimPrefix(cleanPath, prefix)
			rel = filepath.ToSlash(rel)
			rel = strings.TrimLeft(rel, "/")
			if rel == "" {
				return "/", nil
			}
			return rel, nil
		}
		return "", fmt.Errorf("filesystem: list 路径必须位于 mount_path 下: %s", mountRoot)
	}

	if strings.HasPrefix(trimmed, "/") {
		return strings.TrimLeft(trimmed, "/"), nil
	}
	return trimmed, nil
}

// normalizeLocalBuildRequest 兼容本地驱动的绝对路径输入
func normalizeLocalBuildRequest(client Client, driverType syncengine.DriverType, req syncengine.BuildStrmRequest) syncengine.BuildStrmRequest {
	if driverType != syncengine.DriverLocal {
		return req
	}
	impl, ok := client.(*ClientImpl)
	if !ok {
		req.RemotePath = strings.TrimLeft(req.RemotePath, "/")
		return req
	}

	remote := strings.TrimSpace(req.RemotePath)
	if remote == "" || remote == "/" {
		req.RemotePath = strings.TrimLeft(remote, "/")
		return req
	}

	mountRoot := strings.TrimSpace(impl.Config.MountPath)
	if mountRoot == "" {
		req.RemotePath = strings.TrimLeft(remote, "/")
		return req
	}

	if filepath.IsAbs(remote) || filepath.VolumeName(remote) != "" {
		cleanMount := filepath.Clean(mountRoot)
		cleanPath := filepath.Clean(remote)
		if cleanPath == cleanMount {
			req.RemotePath = ""
			return req
		}
		prefix := cleanMount + string(filepath.Separator)
		if strings.HasPrefix(cleanPath, prefix) {
			rel := strings.TrimPrefix(cleanPath, prefix)
			rel = filepath.ToSlash(rel)
			req.RemotePath = strings.TrimLeft(rel, "/")
			return req
		}
	}

	req.RemotePath = strings.TrimLeft(remote, "/")
	return req
}

// CompareStrm 比对现有 STRM 内容与期望内容
//
// 比对规则（按顺序）：
// 1. 空内容 → NeedUpdate
// 2. 解析失败 → NeedUpdate
// 3. BaseURL 不匹配 → NeedUpdate
// 4. Path 不匹配（规范化后） → NeedUpdate
// 5. PickCode 不匹配（如果 PickCode 能力启用）→ NeedUpdate
// 6. Sign 缺失或过期（如果 SignURL 能力启用）→ NeedUpdate
// 7. 所有检查通过 → Equal
//
// 实现细节：
// - 忽略前后空白
// - BaseURL 比对忽略大小写
// - Path 使用 path.Clean 规范化
// - 支持多种过期时间参数名（expires/e）
func (a *Adapter) CompareStrm(ctx context.Context, input syncengine.CompareInput) (syncengine.CompareResult, error) {
	_ = ctx // 保留用于未来的取消或追踪

	actualRaw := strings.TrimSpace(input.ActualRaw)
	if actualRaw == "" {
		return syncengine.CompareResult{
			Equal:      false,
			NeedUpdate: true,
			Reason:     "内容为空",
		}, nil
	}

	actualURL, err := url.Parse(actualRaw)
	if err != nil {
		return syncengine.CompareResult{
			Equal:      false,
			NeedUpdate: true,
			Reason:     fmt.Sprintf("解析实际 URL 失败: %v", err),
		}, nil
	}

	expected := input.Expected
	// 如果 Expected 没有 BaseURL 但有 RawURL，尝试解析
	if expected.BaseURL == nil && strings.TrimSpace(expected.RawURL) != "" {
		expectedURL, parseErr := url.Parse(strings.TrimSpace(expected.RawURL))
		if parseErr != nil {
			return syncengine.CompareResult{}, fmt.Errorf("filesystem: 解析期望 URL 失败: %w", parseErr)
		}
		expected.BaseURL = &url.URL{Scheme: expectedURL.Scheme, Host: expectedURL.Host}
		if expected.Path == "" {
			expected.Path = expectedURL.Path
		}
	}

	// 对于本地模式（StrmMount），BaseURL 可能为 nil，此时只比较 Path
	isLocalMode := expected.BaseURL == nil || (expected.BaseURL.Scheme == "" && expected.BaseURL.Host == "")
	if !isLocalMode && strings.TrimSpace(expected.Path) == "" {
		return syncengine.CompareResult{}, fmt.Errorf("filesystem: 期望内容缺少 Path: %w", syncengine.ErrInvalidInput)
	}

	// 检查 BaseURL（scheme + host）- 仅对非本地模式
	if !isLocalMode && !sameBaseURL(expected.BaseURL, actualURL) {
		return syncengine.CompareResult{
			Equal:      false,
			NeedUpdate: true,
			Reason:     "BaseURL 不匹配",
		}, nil
	}

	// 检查 Path（规范化后）
	expectedPath := cleanRemotePath(expected.Path)
	actualPath := cleanRemotePath(actualURL.Path)
	if expectedPath != actualPath {
		return syncengine.CompareResult{
			Equal:      false,
			NeedUpdate: true,
			Reason:     "路径不匹配",
		}, nil
	}

	cap := a.Capabilities()
	query := actualURL.Query()

	// 检查 PickCode（如果驱动支持）
	if cap.PickCode {
		actualPick := firstNonEmpty(query.Get("pickcode"), query.Get("pick_code"))
		if expected.PickCode != actualPick {
			return syncengine.CompareResult{
				Equal:      false,
				NeedUpdate: true,
				Reason:     "PickCode 不匹配",
			}, nil
		}
	}

	// 检查 Sign（如果驱动支持）
	if cap.SignURL {
		actualSign := query.Get("sign")
		if actualSign == "" {
			return syncengine.CompareResult{
				Equal:      false,
				NeedUpdate: true,
				Reason:     "签名缺失",
			}, nil
		}
		if expected.Sign != "" && expected.Sign != actualSign {
			return syncengine.CompareResult{
				Equal:      false,
				NeedUpdate: true,
				Reason:     "签名不匹配",
			}, nil
		}
		// 检查签名过期时间
		if expiry := parseExpiry(query.Get("expires"), query.Get("e")); !expiry.IsZero() && time.Now().After(expiry) {
			return syncengine.CompareResult{
				Equal:      false,
				NeedUpdate: true,
				Reason:     "签名已过期",
			}, nil
		}
		if !expected.ExpiresAt.IsZero() && time.Now().After(expected.ExpiresAt) {
			return syncengine.CompareResult{
				Equal:      false,
				NeedUpdate: true,
				Reason:     "期望签名已过期",
			}, nil
		}
	}

	return syncengine.CompareResult{
		Equal:      true,
		NeedUpdate: false,
		Reason:     "内容一致",
	}, nil
}

// TestConnection 委托给 filesystem.Client 测试连接
func (a *Adapter) TestConnection(ctx context.Context) error {
	if err := a.client.TestConnection(ctx); err != nil {
		return fmt.Errorf("filesystem: 测试连接失败: %w", err)
	}
	return nil
}

// 辅助函数：转换事件类型
func toDriverEventType(raw string) syncengine.DriverEventType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "create":
		return syncengine.DriverEventCreate
	case "update":
		return syncengine.DriverEventUpdate
	case "delete":
		return syncengine.DriverEventDelete
	default:
		return 0
	}
}

// 辅助函数：比对 BaseURL（忽略大小写）
func sameBaseURL(expected *url.URL, actual *url.URL) bool {
	if expected == nil || actual == nil {
		return false
	}
	return strings.EqualFold(expected.Scheme, actual.Scheme) &&
		strings.EqualFold(expected.Host, actual.Host)
}

// 辅助函数：返回第一个非空字符串
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// 辅助函数：解析过期时间戳
func parseExpiry(values ...string) time.Time {
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(ts, 0)
		}
	}
	return time.Time{}
}
