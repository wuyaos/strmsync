// Package model 定义领域模型
//
// 本包包含系统的核心业务实体，采用 GORM 模型定义。
//
// # 主要实体
//
//   - DataServer: 数据服务器配置（CloudDrive2/OpenList/Local）
//   - Job: 同步任务配置
//   - TaskRun: 任务运行记录
//   - Task: 异步任务队列记录
//
// # 设计原则
//
//   - 领域模型是系统的核心，不依赖任何外层
//   - 使用 GORM 标签定义数据库映射
//   - 包含业务验证逻辑
//
// # 依赖关系
//
//	domain/model 不依赖任何其他内部包
//	所有其他层都可以依赖 domain/model
package model
