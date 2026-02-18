// Package worker 提供任务执行器（Worker Pool）
//
// 本包实现固定大小的 Worker 池，负责从队列领取任务并执行。
//
// # 核心组件
//
//   - WorkerPool: Worker 池管理器
//   - TaskExecutor: 具体任务执行器接口
//
// # 主要功能
//
//   - 并发执行任务（可配置并发数）
//   - 从队列轮询任务
//   - 任务执行超时控制
//   - 优雅关闭（等待任务完成）
//   - 执行统计和监控
//
// # 执行流程
//
//	Worker 从 Queue 领取任务 → 调用 TaskExecutor → 更新任务状态 → 循环
//
// # 错误处理
//
//   - 可重试错误：任务重新入队
//   - 永久错误：标记任务失败
//   - 超时：视为可重试错误
//
// # 设计特性
//
//   - 并发控制：固定数量的 Worker
//   - 心跳机制：Worker 健康检查
//   - 优雅关闭：等待正在执行的任务
//
// # 依赖关系
//
//	worker 依赖 queue（任务领取）
//	worker 依赖 ports.TaskExecutor（任务执行）
package worker
