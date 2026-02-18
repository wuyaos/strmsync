// Package job 实现 Job 业务逻辑服务
//
// 本包提供 Job（同步任务）的业务逻辑实现，包括任务启动、停止、配置验证等。
//
// # 主要功能
//
//   - 启动同步任务（创建 TaskRun 并执行）
//   - 停止正在运行的任务
//   - 验证 Job 配置有效性
//   - 查询任务运行状态
//
// # Job 生命周期
//
//	创建 → 验证配置 → 启动执行 → 运行中 → 停止/完成
//
// # 实现接口
//
//	实现 ports.JobService 接口
//
// # 依赖关系
//
//   - 依赖 ports 接口定义
//   - 依赖 TaskRunService 管理任务运行
//   - 依赖 TaskExecutor 执行同步任务
//   - 依赖 domain/model 数据模型
package job
