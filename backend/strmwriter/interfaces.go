// Package strmwriter 定义 STRM 文件写入器的抽象接口
//
// 本包提供统一的文件写入接口，用于：
// 1. 抽象文件系统操作，便于测试（可mock）
// 2. 支持未来扩展（如远程文件系统写入）
// 3. 统一错误处理和路径验证
package strmwriter

import (
	"context"
	"time"
)

// StrmWriter 定义 STRM 文件的读写删除操作
//
// 设计原则：
// - 所有方法接受 context 用于取消和超时控制
// - 路径必须是绝对路径或相对于 writer 根路径的路径
// - 实现必须是并发安全的
// - 错误必须包含路径上下文以便调试
//
// 实现示例：
// - LocalWriter: 本地文件系统写入
// - RemoteWriter: 远程文件系统写入（未来扩展）
// - MockWriter: 测试用 mock 实现
type StrmWriter interface {
	// Read 读取 STRM 文件的原始内容
	//
	// 参数：
	//   - ctx: 上下文，用于取消
	//   - path: STRM 文件的绝对路径或相对路径
	//
	// 返回：
	//   - string: 文件的原始内容（不做任何处理）
	//   - error: 文件不存在时返回 os.IsNotExist(err)==true 的错误
	//
	// 行为：
	// - 尊重 ctx 取消
	// - 文件不存在时返回错误（不返回空字符串）
	// - 不处理编码，返回原始字节转换的字符串
	Read(ctx context.Context, path string) (string, error)

	// Write 创建或覆盖 STRM 文件
	//
	// 参数：
	//   - ctx: 上下文，用于取消
	//   - path: STRM 文件的绝对路径或相对路径
	//   - content: 要写入的内容（通常是URL）
	//   - modTime: 可选的修改时间（Zero 值表示使用当前时间）
	//
	// 返回：
	//   - error: 写入失败或路径无效时返回错误
	//
	// 行为：
	// - 如果父目录不存在，自动创建
	// - 文件存在则覆盖
	// - 如果 modTime 非零，设置文件的修改时间
	// - 尊重 ctx 取消
	//
	// 用途：
	// - 设置 modTime 可保持与源文件的时间一致性
	// - 便于增量同步（基于 mtime 判断）
	Write(ctx context.Context, path string, content string, modTime time.Time) error

	// Delete 删除 STRM 文件
	//
	// 参数：
	//   - ctx: 上下文，用于取消
	//   - path: STRM 文件的绝对路径或相对路径
	//
	// 返回：
	//   - error: 删除失败时返回错误（文件不存在不算错误）
	//
	// 行为：
	// - 文件不存在时不返回错误（幂等操作）
	// - 尊重 ctx 取消
	// - 不递归删除目录
	//
	// 用途：
	// - 清理孤儿文件（源文件已删除但 STRM 仍存在）
	Delete(ctx context.Context, path string) error
}
