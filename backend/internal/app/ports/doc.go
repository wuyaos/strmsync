// Package ports 定义应用层的端口（Ports）
//
// 在六边形架构（Hexagonal Architecture）中，Ports 是应用核心与外部世界的接口层：
//
// # 职责
//
//   - 定义应用层对外提供的服务接口（入站端口）
//   - 定义应用层依赖的基础设施接口（出站端口）
//   - 包含跨领域的共享类型和 DTO
//
// # 设计原则
//
//   - 只定义接口和类型，不包含实现
//   - 不依赖任何基础设施层（infra）
//   - 由 app 层的服务实现这些接口
//   - 由 infra 层的适配器实现依赖接口
//
// # 主要接口
//
//   - FileService: 文件服务接口
//   - JobService: Job 业务逻辑服务接口
//   - TaskRunService: TaskRun 管理服务接口
//   - TaskExecutor: 任务执行器接口
//   - FileMonitor: 文件监控器接口
//
// # 依赖关系
//
//	app → ports（实现接口）
//	infra → ports（实现接口）
//	transport → ports（调用接口）
package ports
