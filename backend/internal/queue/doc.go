// Package syncqueue 提供持久化任务队列
//
// 本包基于数据库实现的持久化任务队列，支持优先级、重试、去重等特性。
//
// # 核心功能
//
//   - 任务入队（支持优先级和去重）
//   - 任务领取（支持工作者 ID 绑定）
//   - 任务状态管理（pending/running/completed/failed）
//   - 自动重试（可配置重试策略）
//   - 任务查询和过滤
//
// # 任务状态流转
//
//	pending → running → completed
//	pending → running → failed (retryable) → pending (retry)
//	pending → running → failed (permanent)
//
// # 重试策略
//
//   - 指数退避算法
//   - 最大重试次数限制
//   - 区分可重试和永久失败
//
// # 使用场景
//
//   - 异步任务处理
//   - 定时任务调度
//   - 失败重试机制
//
// # 依赖关系
//
//	queue 依赖 domain/model（Task 实体）
//	scheduler/worker 使用 queue 管理任务
package syncqueue
