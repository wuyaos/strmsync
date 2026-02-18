// Package mediaserver 提供媒体服务器客户端
//
// 本包是 pkg/sdk/mediaserver 的适配器层，提供向后兼容的接口。
//
// # 支持的媒体服务器
//
//   - Emby
//   - Jellyfin
//
// # 主要功能
//
//   - 媒体库刷新
//   - 媒体服务器连接测试
//
// # 设计说明
//
// 本包主要作为兼容层存在，实际实现在 pkg/sdk/mediaserver 中。
// 未来可能会将此包移除，直接使用 SDK。
//
// # 依赖关系
//
//	mediaserver → pkg/sdk/mediaserver（重导出）
package mediaserver
