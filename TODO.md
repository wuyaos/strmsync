# TODO（按优先级）

> 原则：全新系统、无旧数据兼容、不引入 Feature Flag。

## P0 - Phase 2：Executor 适配（全强类型）

目标：Executor 全面使用强类型配置对象，不再解析 JSON 字符串。

拆分项：
- 审计 Executor / Engine / Worker 路径中是否仍存在 JSON 解析逻辑（Options/Config）
- 统一输入为强类型结构体（JobOptions / DataServerOptions / MediaServerOptions）
- 移除所有运行时 JSON 解析分支与容错逻辑
- 为强类型对象补齐必填校验（防止空字段进入执行层）

验收：
- 执行层无任何 JSON 解析
- 通过 API 入参校验即可发现配置错误

## P1 - Phase 4：QoS（Per-Server limiter + 并发策略）

目标：按数据源控制并发与速率，避免压垮弱 API。

拆分项：
- 设计 Per-Server 限流器（token bucket）
- 为不同 ServerType 提供默认并发/速率配置
- Executor 在访问前强制限流（I/O 前置）
- 运行时指标：限流命中次数、等待耗时

验收：
- 可按 server 单独限制并发/速率
- 429/503 显著下降

## P1 - 执行历史 & 系统日志自动刷新

目标：在页面内恢复自动刷新，离开页面自动停止。

拆分项：
- 执行历史页面（TaskRuns）定时刷新与可开关
- 系统日志页面（Logs）定时刷新与可开关
- 页面 deactivated 时停止刷新

验收：
- 页面停留时刷新生效；离开不再后台刷新

## P1 - MainLayout 图标路径

目标：Header GitHub 图标从 /assets/icons/ 加载（符合资源规范）。

拆分项：
- 调整 MainLayout 图标资源路径
- 确保生产构建资源可用

## P2 - 后端日志模块改进（zap + gorm）

目标：提升性能、易用性与可观测性。

拆分项：
- 移除全局锁争用（ReplaceGlobals / 原子指针）
- 引入 SugaredLogger（Infof/Errorf）
- WithContext 自动注入 request_id/trace_id
- DB 日志批量写入（batch insert + 定时 flush）
- GORM Logger 适配器接入
- 控制台恢复 caller/function 输出
- TraceOperation 操作耗时装饰器

验收：
- 高并发下无锁争用瓶颈
- 业务日志可按 request_id/operation 聚合
- DB 日志写入吞吐稳定

---

## P1 - 系统日志去重与格式规范

目标：避免同一任务完成的重复日志，统一日志列展示格式（时间/级别/模块/操作/结果/详情）。

拆分项：
- 后端：queue 侧“任务完成”日志降级为 Debug 或改名为“队列状态更新”
- 后端：统一操作命名（例如 STRM同步任务(任务名)）与 source 字段中文化
- 前端：系统日志页面拆分列（操作/结果/详情）与去重规则
- 前端：按 task_id + action + 时间窗口 去重（可配置）

验收：
- 任务完成只出现一条主日志（worker）
- 操作列为高层动作名，结果列为具体结果
- 详情列显示结构化字段

---

## P1 - 系统日志显示规则（前端）

目标：日志界面统一为「时间/级别/模块/操作/结果/详情」六列，操作名与结果规范化展示。

规则：
- 模块名称映射：
  - system → 系统
  - api → API
  - worker → 任务执行器
  - queue → 任务队列
  - scheduler → 调度器
  - engine → 同步引擎
  - filesystem → 文件系统
  - mediaserver → 媒体服务
- 操作名默认规则：
  - 连接测试相关 → 测试连通性
  - STRM 任务执行 → STRM同步任务(<任务名>)
  - 元数据流程 → 元数据同步(<任务名>)
  - 系统启动 → 软件启动（显示 Logo）
  - 环境变量 → 环境变量加载
- 结果列规则：
  - 默认取 message 原文（作为“结果”）
  - 系统启动：结果为「版本信息 vX.Y.Z；工作目录=...；日志=...；数据库=...」
  - 环境变量：结果为「xx=xx; xx=xx; ...」
- 详情列规则：
  - 结构化字段以 key=value 形式拼接
  - caller 统一映射为中文 source（例如 系统.软件启动）

示例：
- 软件启动：
  时间 | INFO | 系统 | [LOGO] 软件启动 | 版本信息 v1.1.0；工作目录=...；日志=...；数据库=... | source=系统.软件启动
- 环境变量加载：
  时间 | INFO | 系统 | 环境变量加载 | DB_PATH=...; ENCRYPTION_KEY=...; ... | source=系统.环境变量

---

## 技术方案（全新系统版本）

### 总体原则
- 强类型为主，拒绝动态 JSON 配置
- API 强校验，非法配置直接失败
- Executor 只接受强类型配置
- 无旧数据迁移与兼容代码

### 1) 类型安全配置（高优先级）
- JobOptions / DataServerOptions / MediaServerOptions 强类型结构体
- Options 在 API 层完成解析并持久化为结构化字段（或 JSON 但必须能反序列化）
- Executor 直接使用强类型对象，不再解析 JSON

### 2) 优雅停机（高优先级）
- 全局 context.Context 贯穿 WorkerPool / Executor
- WorkerPool 支持 GracePeriod
- 退出前持久化任务状态

### 3) 并发管理与 QoS（中优先级）
- Per-Server limiter（token bucket）
- API 类型配置可设置默认并发/速率
- Executor 在访问前强制限流

### 4) 错误语义与重试（中优先级）
- RetryableError 接口 + errors.Is/As
- 驱动层负责标注错误语义

### 5) 构建与部署解耦（低优先级）
- Dev: Vite + Proxy
- Prod: Nginx + Go API
