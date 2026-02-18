// Package sync 实现 STRM 同步服务
//
// 本包提供完整的 STRM 文件同步流程实现，包括文件监控、同步计划生成和 STRM 文件生成。
//
// # 主要组件
//
//   - Monitor: 文件监控器，监控数据源文件变化
//   - Planner: 同步计划器，根据文件事件生成同步计划
//   - Generator: STRM 生成器，创建/更新/删除 STRM 文件
//   - Executor: 任务执行器，协调整个同步流程
//
// # 工作流程
//
//	Monitor 监控文件变化 → Planner 生成同步计划 → Generator 应用变更
//
// # 依赖关系
//
//   - 依赖 ports 接口定义
//   - 依赖 engine 同步引擎
//   - 依赖 infra/filesystem 文件系统抽象
//
// # 使用场景
//
//   - 定时同步任务
//   - 实时文件监控
//   - 批量 STRM 生成
package sync
