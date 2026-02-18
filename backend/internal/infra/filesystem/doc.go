// Package filesystem 提供文件系统抽象层
//
// 本包定义统一的文件系统接口，支持多种数据源类型。
//
// # 支持的文件系统类型
//
//   - local: 本地文件系统
//   - clouddrive2: CloudDrive2 云盘
//   - openlist: OpenList 网盘列表
//
// # 核心接口
//
//   - Provider: 文件系统提供者接口
//   - Client: 文件系统客户端
//
// # 主要功能
//
//   - 统一的文件列表查询
//   - 文件元数据获取
//   - STRM 信息构建
//   - 文件变化监控（可选）
//
// # 适配器模式
//
//	filesystem.Client → filesystem.Provider → 具体实现（local/clouddrive2/openlist）
//
// # 依赖关系
//
//   - 依赖 pkg/sdk 第三方 SDK
//   - 实现 engine.Driver 接口
//   - 被 app 层服务使用
package filesystem
