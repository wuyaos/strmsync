// Package http 提供 HTTP 传输层（REST API）
//
// 本包实现系统的 HTTP 接口，提供 RESTful API 供前端调用。
//
// # 主要 Handler
//
//   - FileHandler: 文件管理相关接口
//   - JobHandler: Job 管理接口
//   - TaskRunHandler: TaskRun 查询接口
//   - DataServerHandler: 数据服务器配置接口
//   - MediaServerHandler: 媒体服务器管理接口
//
// # 设计原则
//
//   - Handler 只负责 HTTP 请求/响应处理
//   - 业务逻辑委托给 app 层服务
//   - 不直接访问数据库或外部系统
//
// # 中间件
//
//   - CORS: 跨域支持
//   - RequestID: 请求追踪
//   - Logger: 访问日志
//   - Recovery: Panic 恢复
//
// # 依赖关系
//
//	transport → app/ports（调用服务接口）
//	transport 不应该 → infra（应通过 app 层）
package http
