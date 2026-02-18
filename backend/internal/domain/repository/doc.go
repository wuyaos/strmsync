// Package repository 定义仓储接口
//
// 本包定义数据访问层的接口，遵循 DDD 的仓储模式。
//
// # 主要接口
//
//   - DataServerRepository: 数据服务器仓储接口
//   - JobRepository: Job 仓储接口
//
// # 设计原则
//
//   - 只定义接口，不包含实现
//   - 由 infra/db/repository 提供具体实现
//   - 隔离领域层与数据访问细节
//
// # 依赖关系
//
//	domain/repository 依赖 domain/model
//	infra/db/repository 实现这些接口
package repository
