// Package file 实现文件服务
//
// 本包提供文件列表查询功能，支持从不同类型的数据服务器获取文件列表。
//
// # 主要功能
//
//   - 查询数据服务器文件列表
//   - 支持递归遍历目录
//   - 支持深度限制
//   - 类型转换（infra → ports）
//
// # 支持的数据源
//
//   - 本地文件系统
//   - CloudDrive2
//   - OpenList
//
// # 实现接口
//
//	实现 ports.FileService 接口
//
// # 依赖关系
//
//   - 依赖 ports 接口定义
//   - 依赖 infra/filesystem 文件系统抽象
//   - 依赖 domain/model 数据模型
package file
