// Package filemonitor 实现本地文件系统监控器
package filemonitor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/strmsync/strmsync/internal/service"
	"github.com/strmsync/strmsync/internal/service/types"
	"github.com/strmsync/strmsync/internal/utils"
	"go.uber.org/zap"
)

const (
	eventBufferSize = 256 // 事件通道缓冲大小
	errorBufferSize = 16  // 错误通道缓冲大小
)

// Monitor 本地文件系统监控器实现
type Monitor struct {
	logger        *zap.Logger
	tempExts      map[string]struct{} // 临时文件扩展名
	tempPrefixSet map[string]struct{} // 临时文件前缀
	tempSuffixSet map[string]struct{} // 临时文件后缀
}

// Option 监控器可选配置
type Option func(*Monitor)

// WithLogger 注入自定义日志器（用于测试）
func WithLogger(logger *zap.Logger) Option {
	return func(m *Monitor) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// NewMonitor 创建文件监控器
func NewMonitor(opts ...Option) service.FileMonitor {
	m := &Monitor{
		logger: utils.L(),
		// 临时文件扩展名（小写）
		tempExts: map[string]struct{}{
			".tmp":        {},
			".part":       {},
			".partial":    {},
			".temp":       {},
			".swp":        {},
			".swx":        {},
			".download":   {},
			".crdownload": {},
		},
		// 临时文件前缀
		tempPrefixSet: map[string]struct{}{
			"~": {},
		},
		// 临时文件后缀
		tempSuffixSet: map[string]struct{}{
			"~": {},
		},
	}

	// 应用可选配置
	for _, opt := range opts {
		opt(m)
	}

	// 确保 logger 非空
	if m.logger == nil {
		m.logger = utils.L()
	}

	return m
}

// Watch 监控文件变化（返回事件通道和错误通道，ctx取消时停止并关闭通道）
func (m *Monitor) Watch(ctx context.Context, config *types.JobConfig) (<-chan types.FileEvent, <-chan error) {
	eventCh := make(chan types.FileEvent, eventBufferSize)
	errCh := make(chan error, errorBufferSize)

	// 输入验证失败：立即返回并发送错误
	if config == nil {
		sendErr(errCh, fmt.Errorf("filemonitor: config is nil"))
		close(eventCh)
		close(errCh)
		return eventCh, errCh
	}
	if strings.TrimSpace(config.SourcePath) == "" {
		sendErr(errCh, fmt.Errorf("filemonitor: source_path is required"))
		close(eventCh)
		close(errCh)
		return eventCh, errCh
	}

	// 启动监控 goroutine
	go m.runWatch(ctx, config, eventCh, errCh)

	return eventCh, errCh
}

// runWatch 执行监控逻辑（在独立goroutine中运行）
func (m *Monitor) runWatch(ctx context.Context, config *types.JobConfig, eventCh chan types.FileEvent, errCh chan error) {
	defer close(eventCh)
	defer close(errCh)

	// 获取绝对路径
	rootAbs, err := filepath.Abs(config.SourcePath)
	if err != nil {
		rootAbs = filepath.Clean(config.SourcePath)
	}

	// 验证源路径是目录
	info, err := os.Stat(rootAbs)
	if err != nil {
		sendErr(errCh, fmt.Errorf("filemonitor: stat source_path: %w", err))
		return
	}
	if !info.IsDir() {
		sendErr(errCh, fmt.Errorf("filemonitor: source_path is not a directory: %s", rootAbs))
		return
	}

	// 创建 fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		sendErr(errCh, fmt.Errorf("filemonitor: create watcher: %w", err))
		return
	}
	defer watcher.Close()

	// 规范化扩展名
	extSet := normalizeExts(config.Extensions)

	// 记录已监控的目录
	watchedDirs := make(map[string]struct{})

	// 添加监控目录的辅助函数（仅在当前goroutine中调用，无需加锁）
	addWatch := func(path string) {
		if _, ok := watchedDirs[path]; ok {
			return // 已经在监控中
		}
		if err := watcher.Add(path); err != nil {
			sendErr(errCh, fmt.Errorf("filemonitor: add watch %s: %w", path, err))
			return
		}
		watchedDirs[path] = struct{}{}
		m.logger.Debug("添加监控目录", zap.String("path", path))
	}

	// 移除监控目录的辅助函数
	removeWatch := func(path string) {
		if _, ok := watchedDirs[path]; !ok {
			return // 未在监控中
		}
		if err := watcher.Remove(path); err != nil {
			// Remove 失败通常是目录已被删除，不算错误
			m.logger.Debug("移除监控失败（可能目录已删除）", zap.String("path", path), zap.Error(err))
		}
		delete(watchedDirs, path)
		m.logger.Debug("移除监控目录", zap.String("path", path))
	}

	// 初始化监控目录
	if config.Recursive {
		// 递归模式：遍历并监控所有子目录
		err = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// 跳过符号链接目录（避免循环）
			if d.Type()&os.ModeSymlink != 0 {
				if d.IsDir() {
					m.logger.Debug("跳过符号链接目录", zap.String("path", path))
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				addWatch(path)
			}
			return nil
		})
		if err != nil && err != context.Canceled {
			sendErr(errCh, fmt.Errorf("filemonitor: init recursive watch: %w", err))
			return
		}
	} else {
		// 非递归模式：只监控根目录
		addWatch(rootAbs)
	}

	m.logger.Info("文件监控已启动",
		zap.String("path", rootAbs),
		zap.Bool("recursive", config.Recursive),
		zap.Int("watched_dirs", len(watchedDirs)))

	// 主事件循环
	for {
		select {
		case <-ctx.Done():
			m.logger.Info("文件监控已停止（context取消）")
			return

		case err := <-watcher.Errors:
			if err != nil {
				m.logger.Error("fsnotify错误", zap.Error(err))
				sendErr(errCh, fmt.Errorf("filemonitor: watcher error: %w", err))
			}

		case ev, ok := <-watcher.Events:
			if !ok {
				m.logger.Info("文件监控已停止（watcher关闭）")
				return
			}

			if ev.Name == "" {
				continue
			}

			// 处理文件系统事件
			m.handleEvent(ctx, ev, config, rootAbs, extSet, eventCh, watchedDirs, addWatch, removeWatch)
		}
	}
}

// handleEvent 处理单个文件系统事件
func (m *Monitor) handleEvent(
	ctx context.Context,
	ev fsnotify.Event,
	config *types.JobConfig,
	rootAbs string,
	extSet map[string]struct{},
	eventCh chan types.FileEvent,
	watchedDirs map[string]struct{},
	addWatch func(string),
	removeWatch func(string),
) {
	// 尝试获取文件信息
	info, statErr := os.Stat(ev.Name)
	isDir := false
	if statErr == nil {
		isDir = info.IsDir()
	}

	// 处理目录的创建/删除/重命名
	if isDir {
		if config.Recursive && ev.Op&fsnotify.Create != 0 {
			// 新建目录：动态添加监控
			addWatch(ev.Name)
		}
		// 目录删除/重命名：移除监控
		if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			if _, ok := watchedDirs[ev.Name]; ok {
				removeWatch(ev.Name)
			}
		}
		// 目录事件不向下游发送
		return
	}

	// 处理文件的删除/重命名（检查是否误判为文件的已删除目录）
	if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		// 如果在 watchedDirs 中，说明实际上是目录
		if _, ok := watchedDirs[ev.Name]; ok {
			removeWatch(ev.Name)
			return
		}
	}

	// 过滤临时文件
	if m.isTempFile(ev.Name) {
		return
	}

	// 过滤扩展名
	if !matchesExtension(ev.Name, extSet) {
		return
	}

	// 映射事件类型
	eventType, ok := mapEventType(ev.Op)
	if !ok {
		return
	}

	// 构造相对路径
	absPath := ev.Name
	relPath := relPath(rootAbs, absPath)

	// 获取文件大小和修改时间
	var size int64
	var modTime time.Time
	if statErr == nil {
		size = info.Size()
		modTime = info.ModTime()
	}

	// 构造事件
	fileEvent := types.FileEvent{
		Type:    eventType,
		Path:    relPath,
		AbsPath: absPath,
		ModTime: modTime,
		Size:    size,
		IsDir:   false,
	}

	// 发送事件（非阻塞）
	sendEvent(ctx, eventCh, fileEvent)
}

// Scan 执行一次性扫描
func (m *Monitor) Scan(ctx context.Context, config *types.JobConfig) ([]types.FileEvent, error) {
	// 输入验证
	if config == nil {
		return nil, fmt.Errorf("filemonitor: config is nil")
	}
	if strings.TrimSpace(config.SourcePath) == "" {
		return nil, fmt.Errorf("filemonitor: source_path is required")
	}

	// 获取绝对路径
	rootAbs, err := filepath.Abs(config.SourcePath)
	if err != nil {
		rootAbs = filepath.Clean(config.SourcePath)
	}

	// 验证源路径是目录
	info, err := os.Stat(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("filemonitor: stat source_path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("filemonitor: source_path is not a directory: %s", rootAbs)
	}

	// 规范化扩展名
	extSet := normalizeExts(config.Extensions)

	m.logger.Info("开始扫描文件",
		zap.String("path", rootAbs),
		zap.Bool("recursive", config.Recursive))

	var results []types.FileEvent

	// 遍历目录
	walkErr := filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// 检查 context 取消
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 非递归模式：跳过子目录
		if !config.Recursive && d.IsDir() && path != rootAbs {
			return filepath.SkipDir
		}

		// 跳过符号链接目录
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				m.logger.Debug("跳过符号链接目录", zap.String("path", path))
				return filepath.SkipDir
			}
			return nil
		}

		// 跳过目录本身
		if d.IsDir() {
			return nil
		}

		// 过滤临时文件
		if m.isTempFile(path) {
			return nil
		}

		// 过滤扩展名
		if !matchesExtension(path, extSet) {
			return nil
		}

		// 获取文件信息
		info, err := d.Info()
		if err != nil {
			m.logger.Warn("获取文件信息失败", zap.String("path", path), zap.Error(err))
			return nil // 跳过这个文件，继续扫描
		}

		// 添加到结果
		results = append(results, types.FileEvent{
			Type:    types.FileEventCreate,
			Path:    relPath(rootAbs, path),
			AbsPath: path,
			ModTime: info.ModTime(),
			Size:    info.Size(),
			IsDir:   false,
		})

		return nil
	})

	if walkErr != nil && walkErr != context.Canceled {
		return nil, fmt.Errorf("filemonitor: scan failed: %w", walkErr)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	m.logger.Info("文件扫描完成",
		zap.String("path", rootAbs),
		zap.Int("count", len(results)))

	return results, nil
}

// isTempFile 判断是否为临时文件
func (m *Monitor) isTempFile(path string) bool {
	name := filepath.Base(path)
	if name == "" {
		return false
	}

	nameLower := strings.ToLower(name)

	// 检查前缀
	for prefix := range m.tempPrefixSet {
		if strings.HasPrefix(nameLower, prefix) {
			return true
		}
	}

	// 检查后缀
	for suffix := range m.tempSuffixSet {
		if strings.HasSuffix(nameLower, suffix) {
			return true
		}
	}

	// 检查扩展名
	ext := strings.ToLower(filepath.Ext(name))
	if _, ok := m.tempExts[ext]; ok {
		return true
	}

	return false
}

// normalizeExts 规范化扩展名列表（转小写，添加点前缀）
func normalizeExts(exts []string) map[string]struct{} {
	if len(exts) == 0 {
		return nil // nil 表示不过滤
	}

	out := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		e := strings.ToLower(strings.TrimSpace(ext))
		if e == "" {
			continue
		}
		// 确保扩展名以点开头
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		out[e] = struct{}{}
	}

	return out
}

// matchesExtension 检查文件是否匹配扩展名过滤
func matchesExtension(path string, extSet map[string]struct{}) bool {
	if len(extSet) == 0 {
		return true // 无过滤器，匹配所有文件
	}

	ext := strings.ToLower(filepath.Ext(path))
	_, ok := extSet[ext]
	return ok
}

// mapEventType 将 fsnotify 事件类型映射到 FileEventType
func mapEventType(op fsnotify.Op) (types.FileEventType, bool) {
	switch {
	case op&fsnotify.Create != 0:
		return types.FileEventCreate, true
	case op&fsnotify.Write != 0:
		return types.FileEventUpdate, true
	case op&(fsnotify.Remove|fsnotify.Rename) != 0:
		return types.FileEventDelete, true
	default:
		return 0, false
	}
}

// relPath 计算相对路径
func relPath(rootAbs, absPath string) string {
	rel, err := filepath.Rel(rootAbs, absPath)
	if err != nil {
		// 如果无法计算相对路径，返回绝对路径
		return absPath
	}
	return rel
}

// sendErr 非阻塞发送错误
func sendErr(errCh chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case errCh <- err:
	default:
		// 通道已满，丢弃错误（避免阻塞）
	}
}

// sendEvent 非阻塞发送事件
func sendEvent(ctx context.Context, eventCh chan<- types.FileEvent, ev types.FileEvent) {
	select {
	case <-ctx.Done():
		return
	case eventCh <- ev:
		return
	default:
		// 通道已满，丢弃事件（避免死锁）
		// 生产环境中可以考虑记录日志
	}
}
