# 后端架构说明

> 本文档说明 STRM Sync 后端项目的架构设计和模块组织

## 📊 架构概览

```
backend/
├── cmd/server/                 # 应用入口
├── internal/
│   ├── app/                    # 应用层（业务逻辑编排）
│   │   ├── ports/              # 端口（接口和契约）
│   │   ├── sync/               # 同步服务
│   │   ├── file/               # 文件服务
│   │   ├── job/                # Job 服务
│   │   └── taskrun/            # TaskRun 服务
│   ├── domain/                 # 领域层（核心业务实体）
│   │   ├── model/              # 领域模型
│   │   └── repository/         # 仓储接口
│   ├── infra/                  # 基础设施层（外部系统适配）
│   │   ├── db/                 # 数据库配置
│   │   ├── filesystem/         # 文件系统抽象
│   │   └── mediaserver/        # 媒体服务器客户端
│   ├── engine/                 # STRM 同步引擎（核心算法）
│   ├── queue/                  # 任务队列（持久化队列）
│   ├── scheduler/              # 任务调度器（Cron 调度）
│   ├── worker/                 # Worker 池（任务执行）
│   ├── strmwriter/             # STRM 文件写入器
│   ├── pkg/                    # 通用工具库
│   │   ├── crypto/hash/logger/...
│   │   └── sdk/                # 第三方 SDK
│   └── transport/              # 传输层（HTTP API）
└── tests/e2e/                  # E2E 测试
```

## 🏗️ 分层架构

### 核心分层（纵向依赖）

```
┌─────────────┐
│  transport  │  外部接口适配层（HTTP/gRPC）
└──────┬──────┘
       │
       ↓
┌─────────────┐
│     app     │  应用层（用例编排）
└──────┬──────┘
       │
       ↓
┌─────────────┐
│   domain    │  领域层（核心实体）
└─────────────┘
       ↑
       │
┌──────┴──────┐
│    infra    │  基础设施层（外部适配）
└─────────────┘
```

### 横向能力（平台基础设施）

```
┌──────────────────────────────────────┐
│  engine / queue / scheduler / worker │  任务执行平台
│  strmwriter                           │  I/O 工具
│  pkg                                  │  通用工具
└──────────────────────────────────────┘
```

## 📦 模块职责

### 🎯 核心业务层

#### app/ - 应用层
- **职责**: 业务逻辑编排、用例实现
- **包含**:
  - `ports/`: 应用层接口定义（Ports & Adapters 模式）
  - `sync/`: STRM 同步服务实现
  - `file/`: 文件列表服务
  - `job/`: Job 业务逻辑
  - `taskrun/`: TaskRun 管理
- **依赖**: → domain, ports

#### domain/ - 领域层
- **职责**: 核心业务实体、仓储接口
- **包含**:
  - `model/`: GORM 数据模型
  - `repository/`: 仓储接口定义
- **依赖**: 无（最内层）

#### infra/ - 基础设施层
- **职责**: 外部系统适配、实现 ports 接口
- **包含**:
  - `db/`: 数据库连接管理
  - `filesystem/`: 文件系统抽象（支持 local/clouddrive2/openlist）
  - `mediaserver/`: 媒体服务器客户端
- **依赖**: → domain, ports（实现接口）

### ⚙️ 横向能力层

#### engine/ - STRM 同步引擎
- **职责**: 文件扫描、差异对比、增量同步
- **特点**: 核心算法，独立子系统
- **依赖**: → domain（接口）

#### queue/ - 任务队列
- **职责**: 持久化任务队列、重试机制
- **特点**: 通用基础设施，支持优先级和去重
- **依赖**: → domain/model（Task 实体）

#### scheduler/ - 任务调度器
- **职责**: Cron 定时调度、任务触发
- **特点**: 热重载，动态管理
- **依赖**: → queue, domain

#### worker/ - Worker 池
- **职责**: 并发执行任务、从队列消费
- **特点**: 固定大小池，优雅关闭
- **依赖**: → queue, ports.TaskExecutor

#### strmwriter/ - STRM 写入器
- **职责**: STRM 文件构建和写入
- **特点**: 纯工具库，无业务依赖
- **依赖**: 无

#### pkg/ - 通用工具
- **职责**: 加密、哈希、日志、第三方 SDK
- **特点**: 纯工具，可被任何包使用
- **依赖**: 无

### 🌐 接口层

#### transport/ - 传输层
- **职责**: HTTP/gRPC 接口、请求/响应处理
- **特点**: 只负责协议转换，不包含业务逻辑
- **依赖**: → app/ports（调用服务）

## 🔗 依赖规则

### ✅ 允许的依赖方向

```
transport  →  app/ports
app        →  domain, ports
infra      →  domain, ports（实现接口）
worker     →  queue, ports
scheduler  →  queue, domain
engine     →  domain（接口）
```

### ❌ 禁止的依赖方向

```
domain     ✗→  任何层（领域层必须独立）
app        ✗→  infra（应通过 ports 接口）
engine     ✗→  app（引擎是独立子系统）
queue      ✗→  app（队列是通用基础设施）
transport  ✗→  infra（应通过 app 层）
pkg        ✗→  app/domain/infra（工具库不依赖业务）
```

## 🎨 设计模式

### Ports & Adapters（六边形架构）
- **Ports** (`app/ports/`): 定义应用层接口
- **Adapters**:
  - 入站: `transport/` 实现 HTTP 适配
  - 出站: `infra/` 实现数据库、文件系统适配

### Repository Pattern（仓储模式）
- 接口定义: `domain/repository/`
- 实现: `infra/db/repository/`

### Strategy Pattern（策略模式）
- `filesystem.Provider`: 不同文件系统实现
- `engine.Driver`: 不同数据源驱动

### Factory Pattern（工厂模式）
- `filesystem.NewClient()`: 创建文件系统客户端
- `strmwriter.NewLocalWriter()`: 创建写入器

## 📝 命名约定

### 包名规则
- 包名与目录名一致（如 `internal/app/sync` → `package sync`）
- 使用小写单数形式（如 `model` 而非 `models`）
- 避免泛化命名（如 `util`、`common`）

### 接口命名
- 服务接口：`XxxService`（如 `FileService`）
- 仓储接口：`XxxRepository`（如 `JobRepository`）
- 能力接口：名词或动词（如 `Provider`、`Executor`）

### 类型命名
- 实现类：简洁名词（如 `fileService`、`LocalProvider`）
- DTO/请求：`XxxRequest`、`XxxResponse`
- 配置：`XxxConfig`、`XxxOptions`

## 🧪 测试策略

### 单元测试
- 位置: 与源文件同目录（`xxx_test.go`）
- 覆盖: engine、queue、strmwriter 等核心模块

### 集成测试
- 位置: `tests/e2e/`
- 覆盖: HTTP API、数据库操作

### 测试原则
- 使用 table-driven tests
- Mock 外部依赖
- 保持测试独立和可重复

## 📚 文档位置

- **包级文档**: 各包的 `doc.go` 文件
- **接口文档**: 接口定义处的注释
- **API 文档**: `docs/` 目录（OpenAPI）

## 🔄 扩展指南

### 添加新的数据源
1. 在 `internal/infra/filesystem/` 创建新包
2. 实现 `filesystem.Provider` 接口
3. 在 `filesystem.Type` 添加常量
4. 注册 Provider

### 添加新的应用服务
1. 在 `internal/app/ports/` 定义接口
2. 在 `internal/app/` 创建服务包
3. 实现接口
4. 在 `cmd/server/` 注入依赖

### 添加新的 HTTP 接口
1. 在 `internal/transport/` 创建 Handler
2. 依赖 `app/ports` 接口
3. 在路由中注册

## 🎯 架构目标

- ✅ **清晰的分层边界**
- ✅ **明确的依赖方向**
- ✅ **高内聚低耦合**
- ✅ **易于测试**
- ✅ **便于扩展**

## 📊 架构评分

- **当前评分**: 8.5 / 10
- **项目规模**: 约 60 个 Go 文件
- **状态**: 适合当前规模，无需过度设计

---

**最后更新**: 2026-02-18
**架构版本**: v2（Phase 5 & 6 完成）
