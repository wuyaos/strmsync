// Package taskrun 实现 TaskRun 记录管理服务
//
// 本包提供 TaskRun（任务运行记录）的生命周期管理，记录每次任务执行的详细信息。
//
// # 主要功能
//
//   - 创建并启动 TaskRun
//   - 更新任务进度
//   - 标记任务完成（记录统计信息）
//   - 标记任务失败（记录错误信息）
//   - 取消任务（幂等操作）
//
// # TaskRun 状态流转
//
//	running → completed
//	running → failed
//	running → cancelled
//
// # 实现接口
//
//	实现 ports.TaskRunService 接口
//
// # 依赖关系
//
//   - 依赖 ports 接口定义
//   - 依赖 domain/model 数据模型
//   - 自动更新关联 Job 的 last_run_at
package taskrun
