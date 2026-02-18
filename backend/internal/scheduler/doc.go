// Package scheduler 提供 Cron 任务调度器
//
// 本包基于 robfig/cron 实现的任务调度器，负责定时触发 Job 执行。
//
// # 核心功能
//
//   - Cron 表达式解析
//   - 定时任务调度
//   - 任务动态添加/删除
//   - 任务启用/禁用
//   - 启动/停止调度器
//
// # 调度流程
//
//	Scheduler 定时触发 → 创建 Task → 入队到 Queue → Worker 执行
//
// # Cron 表达式支持
//
// 标准 Cron 格式（秒 分 时 日 月 周）：
//   - "0 0 * * * *"     每小时
//   - "0 */30 * * * *"  每30分钟
//   - "@hourly"         每小时（预设）
//
// # 设计特性
//
//   - 热重载：支持运行时更新调度规则
//   - 错误恢复：调度失败不影响其他任务
//   - 并发安全：支持多 goroutine 操作
//
// # 依赖关系
//
//	scheduler 依赖 queue（任务入队）
//	scheduler 依赖 domain/model（Job 实体）
package scheduler
