// Package pkg 提供通用工具库
//
// 本包包含项目内部共享的工具和 SDK，不包含业务逻辑。
//
// # 工具模块
//
//   - crypto: 加密工具（AES 加解密）
//   - hash: 哈希工具（MD5/SHA256）
//   - logger: 日志工具（zap 封装）
//   - path: 路径工具（路径规范化）
//   - requestid: 请求 ID 生成
//
// # SDK 模块
//
//   - sdk/clouddrive2: CloudDrive2 gRPC 客户端
//   - sdk/mediaserver: Emby/Jellyfin 客户端
//   - sdk/openlist: OpenList HTTP 客户端
//
// # 设计原则
//
//   - 不依赖业务层（app/domain）
//   - 可被任何内部包使用
//   - 保持功能独立和通用
//
// # 依赖关系
//
//	pkg 不依赖其他内部包
//	所有内部包都可以依赖 pkg
package pkg
