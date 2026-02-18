// Package strmwriter 提供 STRM 文件写入器
//
// 本包实现 STRM 文件的构建和写入功能，支持本地路径和 HTTP URL 两种格式。
//
// # 核心组件
//
//   - StrmWriter: STRM 写入器接口
//   - LocalWriter: 本地 STRM 写入器
//   - ContentBuilder: STRM 内容构建器
//
// # 支持的 STRM 格式
//
//   - Local: 本地文件路径（mount 模式）
//   - HTTP: HTTP URL（http 模式）
//
// # 主要功能
//
//   - 生成 STRM 文件内容
//   - 写入 STRM 文件到磁盘
//   - 原子性写入（临时文件 + 重命名）
//   - 路径安全检查（防止越界）
//
// # 内容构建
//
//   - LocalBuilder: 本地路径模式
//     格式: /mnt/media/movie.mp4
//
//   - URLBuilder: HTTP URL 模式
//     格式: http://server.com/api/stream?path=/movie.mp4
//
// # 设计特性
//
//   - Builder 模式：灵活的内容构建
//   - 路径规范化：自动处理路径格式
//   - 安全写入：防止目录穿越攻击
//
// # 依赖关系
//
//	strmwriter 不依赖其他内部包（纯工具库）
//	engine 使用 strmwriter.StrmWriter
package strmwriter
