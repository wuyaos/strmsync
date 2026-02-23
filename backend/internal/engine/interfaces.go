// Package syncengine 定义统一驱动层接口。
//
// Driver 接口为不同数据源提供一致抽象，使同步引擎可统一处理
// CloudDrive2/OpenList/本地文件系统而无需关心细节实现。
package syncengine

import "context"

// Driver 是所有数据源驱动必须实现的统一接口。
//
// 实现要求：
// - 正确处理 context 取消
// - 使用 fmt.Errorf("op: %w", err) 包装错误
// - 不支持的能力返回 ErrNotSupported
// - 可并发安全调用
type Driver interface {
	// Type 返回驱动类型标识（用于日志与能力判断）。
	Type() DriverType

	// Capabilities 返回驱动支持的能力集合。
	Capabilities() DriverCapability

	// Scan 流式扫描指定路径下的条目。
	// 返回条目通道与错误通道，扫描完成或取消时必须关闭。
	Scan(ctx context.Context, path string, opt ListOptions) (<-chan RemoteEntry, <-chan error)

	// Watch 订阅指定路径的变更事件（不支持则返回 ErrNotSupported）。
	Watch(ctx context.Context, path string, opt WatchOptions) (<-chan DriverEvent, error)

	// Stat 获取单个路径的元数据。
	Stat(ctx context.Context, path string) (RemoteEntry, error)

	// BuildStrmInfo 构建 STRM 内容与结构化元信息。
	BuildStrmInfo(ctx context.Context, req BuildStrmRequest) (StrmInfo, error)

	// CompareStrm 比较现有 STRM 内容与期望内容。
	CompareStrm(ctx context.Context, input CompareInput) (CompareResult, error)

	// TestConnection 测试数据源连接是否健康。
	TestConnection(ctx context.Context) error
}
