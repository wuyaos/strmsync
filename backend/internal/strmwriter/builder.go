// Package strmwriter 提供STRM内容生成功能
package strmwriter

import (
	"context"
	"fmt"
)

// ContentBuilder STRM内容生成器接口
//
// 负责根据远程文件路径和服务器配置生成STRM文件的内容。
// 不同的实现生成不同格式的内容：
// - LocalBuilder: 生成本地文件路径
// - URLBuilder: 生成HTTP URL
//
// 设计原则：
// - 接口最小化：只有一个 Build 方法
// - 支持 context 取消和超时
// - 实现必须是线程安全的
// - 验证输入参数，返回清晰的错误信息
type ContentBuilder interface {
	// Build 生成STRM文件内容
	//
	// 参数：
	//   - ctx: 上下文，用于取消和超时控制
	//   - req: 生成请求，包含远程路径和服务器ID
	//
	// 返回：
	//   - string: STRM文件的内容（URL或本地路径）
	//   - error: 生成失败时返回错误
	//
	// 行为：
	// - 尊重 ctx 取消
	// - 验证请求参数
	// - 生成的内容必须是有效的URL或路径
	// - 路径必须经过清理和规范化
	Build(ctx context.Context, req BuildRequest) (string, error)
}

// NewContentBuilder 创建内容生成器（工厂方法）
//
// 根据格式类型创建对应的生成器实现：
// - StrmFormatHTTP → URLBuilder
// - StrmFormatLocal → LocalBuilder
//
// 参数：
//   - format: STRM格式类型
//   - config: 生成配置
//
// 返回：
//   - ContentBuilder: 生成器实例
//   - error: 创建失败时返回错误
//
// 错误情况：
// - 格式无效
// - 配置验证失败
// - 不支持的格式类型
//
// 示例：
//
//	// 创建HTTP格式生成器
//	builder, err := NewContentBuilder(StrmFormatHTTP, BuildConfig{
//	    Format:  StrmFormatHTTP,
//	    BaseURL: "http://192.168.1.100:19798",
//	})
//
//	// 创建Local格式生成器
//	builder, err := NewContentBuilder(StrmFormatLocal, BuildConfig{
//	    Format:    StrmFormatLocal,
//	    LocalRoot: "/mnt/media",
//	})
func NewContentBuilder(format StrmFormat, config BuildConfig) (ContentBuilder, error) {
	// 确保配置的格式字段与参数一致
	config.Format = format

	// 应用默认值
	config = config.WithDefaults()

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("strmwriter: 配置验证失败: %w", err)
	}

	// 根据格式创建对应的生成器
	switch format {
	case StrmFormatHTTP:
		return NewURLBuilder(config)
	case StrmFormatLocal:
		return NewLocalBuilder(config)
	default:
		return nil, fmt.Errorf("strmwriter: 不支持的格式: %s", format)
	}
}
