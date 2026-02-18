// Package syncengine 提供 STRM 同步引擎
//
// 本包是 STRM 同步的核心引擎，负责文件扫描、差异对比和 STRM 文件生成。
//
// # 核心组件
//
//   - Engine: 同步引擎主体
//   - Driver: 文件系统驱动接口
//   - Writer: STRM 文件写入器接口
//
// # 主要功能
//
//   - 扫描数据源文件
//   - 对比本地 STRM 文件
//   - 增量更新（创建/更新/删除）
//   - 并发控制
//   - 统计信息收集
//
// # 工作模式
//
//   - RunOnce: 单次全量同步
//   - 支持递归遍历
//   - 支持文件扩展名过滤
//
// # 设计原则
//
//   - 驱动器抽象：通过 Driver 接口支持多种数据源
//   - 写入器抽象：通过 Writer 接口支持多种写入目标
//   - 可取消：支持 context 取消同步任务
//   - 可测试：核心逻辑与外部依赖隔离
//
// # 依赖关系
//
//	engine 依赖 domain（接口）
//	filesystem 实现 engine.Driver
//	strmwriter 实现 engine.Writer
package syncengine
