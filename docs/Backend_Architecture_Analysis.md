# STRMSync 后端架构分析报告

## 📋 概述

本文档对 STRMSync 后端项目的文件夹结构、模块命名和代码组织进行全面分析，识别潜在问题并提供改进建议。

**分析日期**: 2026-02-18
**项目语言**: Go
**主要功能**: STRM 文件同步系统，支持 CloudDrive2/OpenList 文件系统适配

---

## 🗂️ 当前目录结构

```
strm/
├── backend/
│   ├── main.go              ❗ 入口文件位置不符合标准
│   ├── go.mod
│   ├── go.sum
│   ├── core/                ✅ 核心数据层
│   │   ├── config.go
│   │   ├── database.go
│   │   ├── models.go
│   │   ├── data_server_repository.go
│   │   └── job_repository.go
│   ├── handler/             ⚠️ 目录名与包名不一致（package handlers）
│   │   ├── filesystem_server.go
│   │   ├── media_server.go
│   │   ├── job.go
│   │   ├── task_run.go
│   │   ├── file.go
│   │   ├── log.go
│   │   ├── setting.go
│   │   └── helpers.go
│   ├── filesystem/          ✅ 文件系统驱动层
│   │   ├── interfaces.go
│   │   ├── types.go
│   │   ├── client.go
│   │   ├── clouddrive2.go
│   │   ├── openlist.go
│   │   ├── local.go
│   │   ├── driver_adapter.go
│   │   └── clouddrive2_proto/
│   ├── mediaserver/         ✅ 媒体服务器客户端
│   │   ├── interfaces.go
│   │   ├── types.go
│   │   ├── client.go
│   │   ├── emby.go
│   │   └── jellyfin.go
│   ├── scheduler/           ✅ Cron 调度器
│   │   ├── scheduler.go
│   │   ├── types.go
│   │   └── scheduler_test.go
│   ├── syncengine/          ✅ 同步引擎核心
│   │   ├── engine.go
│   │   ├── interfaces.go
│   │   ├── types.go
│   │   ├── errors.go
│   │   └── engine_test.go
│   ├── syncqueue/           ✅ 任务队列
│   │   ├── queue.go
│   │   ├── types.go
│   │   ├── errors.go
│   │   └── syncqueue_test.go
│   ├── worker/              ✅ Worker 池
│   │   ├── worker.go
│   │   ├── executor.go      ⚠️ 与 service/executor.go 命名冲突
│   │   ├── types.go
│   │   └── worker_test.go
│   ├── strmwriter/          ✅ STRM 文件写入器
│   │   ├── interfaces.go
│   │   └── local_writer.go
│   ├── service/             ⚠️ 职责与边界不明确
│   │   ├── executor.go      ⚠️ 与 worker/executor.go 命名冲突
│   │   ├── file.go
│   │   ├── filemonitor.go
│   │   ├── interfaces.go
│   │   ├── job.go
│   │   ├── planner.go
│   │   ├── strm.go
│   │   ├── taskrun.go
│   │   └── types.go
│   ├── utils/               ✅ 工具函数
│   │   ├── logger.go
│   │   ├── crypto.go
│   │   ├── hash.go
│   │   ├── path.go
│   │   └── request_id.go
│   ├── tests/
│   │   └── e2e/             ✅ E2E 测试
│   │       ├── testenv.go
│   │       └── e2e_test.go
│   └── docs/                ✅ 文档目录
```

---

## 🔍 详细分析

### 1. **与 Go 标准项目布局的匹配度**

参考：[golang-standards/project-layout](https://github.com/golang-standards/project-layout)

| 标准目录 | 当前状态 | 问题 |
|---------|---------|------|
| `cmd/` | ❌ 缺失 | `main.go` 直接放在 `backend/` 下 |
| `internal/` | ❌ 缺失 | 没有明确的内部包边界 |
| `pkg/` | ✅ 不需要 | 无可复用外部库（正确） |
| `api/` | ✅ 不需要 | 使用 Gin 框架，不需要单独 API 定义目录 |
| `tests/` | ✅ 存在 | E2E 测试已组织良好 |
| `docs/` | ✅ 存在 | 文档目录已创建 |

**问题**：
- `main.go` 应该移动到 `cmd/strm-server/main.go`
- 所有业务代码应移动到 `internal/` 或保持当前平铺结构但明确边界

**推荐标准结构**：
```
strm/
├── cmd/
│   └── strm-server/
│       └── main.go
├── internal/
│   ├── http/            # handler 层
│   ├── app/             # service 层（用例编排）
│   ├── domain/          # 核心类型、接口
│   ├── sync/            # syncengine, syncqueue, worker
│   ├── filesystem/      # 文件系统驱动
│   ├── mediaserver/     # 媒体服务器客户端
│   ├── storage/         # core -> storage（数据持久化）
│   └── scheduler/
├── tests/
└── docs/
```

---

### 2. **包命名规范问题**

#### ❌ 问题 1：`handler/` 目录与包名不一致

**现状**：
```go
// backend/handler/filesystem_server.go
package handlers  // 包名是复数
```

**目录名**：`handler/`（单数）

**问题**：
- Go 社区规范：目录名应与包名一致
- 当前状态会导致 `import "github.com/strmsync/strmsync/handler"` 但实际使用 `handlers.XXX`

**推荐解决方案**：
1. **方案 A（推荐）**：目录名改为 `handlers/`
2. **方案 B**：包声明改为 `package handler`

**理由**：统一为单数更符合 Go 惯例（如 `net/http` 的 handler 包）

---

#### ⚠️ 问题 2：`core/` 包名过于泛泛

**现状**：
```go
package core
```

**问题**：
- "core" 是模糊概念，容易变成"垃圾桶"包
- 当前 `core/` 实际职责：数据库层 + Repository

**推荐改名**：
- `storage/` - 强调数据持久化职责
- `repository/` - 强调 Repository 模式
- `database/` - 最直观但可能与 `gorm.DB` 混淆

**推荐方案**：改为 `storage/`

---

#### ⚠️ 问题 3：`executor.go` 命名冲突

**现状**：
- `service/executor.go` - 应用层任务执行器（编排 FileMonitor、SyncPlanner、StrmGenerator）
- `worker/executor.go` - 运行时任务执行器（从队列取任务，调用 syncengine）

**问题**：
- 两个 Executor 类型职责完全不同但命名相同
- 容易在代码中混淆 `service.Executor` 和 `worker.Executor`

**推荐解决方案**：
| 包 | 当前名称 | 推荐名称 | 理由 |
|----|---------|---------|------|
| `service/` | `Executor` | `TaskOrchestrator` 或 `JobRunner` | 强调业务编排职责 |
| `worker/` | `Executor` | 保持 `Executor` | Worker 池中的执行器是标准命名 |

---

### 3. **模块职责分析**

#### ✅ 职责清晰的模块

| 模块 | 职责 | 评价 |
|------|------|------|
| `filesystem/` | 文件系统驱动适配（CloudDrive2、OpenList、Local） | 边界清晰，接口设计良好 |
| `mediaserver/` | 媒体服务器客户端（Emby、Jellyfin） | 职责单一 |
| `syncengine/` | 同步引擎核心逻辑 | 核心业务逻辑，依赖关系合理 |
| `syncqueue/` | 基于数据库的任务队列 | 清晰的队列抽象 |
| `worker/` | Worker 池并发执行 | 职责明确（领取任务 → 执行 → 回写状态） |
| `scheduler/` | Cron 调度器 | 职责清晰 |
| `strmwriter/` | STRM 文件写入器 | 单一职责 |
| `utils/` | 工具函数 | 标准工具包 |

#### ⚠️ 职责不明确的模块

##### `service/` 包

**当前文件**：
```
service/
├── executor.go       # 任务执行编排
├── file.go           # 文件服务（未使用？）
├── filemonitor.go    # 文件变更监控
├── job.go            # Job 服务
├── planner.go        # 同步计划生成
├── strm.go           # STRM 生成服务
├── taskrun.go        # TaskRun 服务
├── interfaces.go     # 接口定义
└── types.go          # 类型定义
```

**问题分析**：
1. **职责混杂**：
   - `executor.go`、`planner.go`、`filemonitor.go` - 核心业务逻辑
   - `job.go`、`taskrun.go` - 看起来像 handler 层的业务逻辑
   - `file.go` - 不确定是否实际使用

2. **与其他模块的关系**：
   - 如果 `handler/` 直接调用 `core/repository`，那么 `service/` 的存在意义是什么？
   - 如果 `service/` 是应用层，`handler/` 应该只依赖 `service/`，不应直接依赖 `core/`

**推荐方案**：

**方案 A：保留 service/ 作为应用层**
```
service/
├── orchestrator.go   # 原 executor.go（重命名）
├── sync_planner.go   # 原 planner.go
├── file_monitor.go   # 原 filemonitor.go
├── strm_generator.go # 原 strm.go
└── interfaces.go     # 接口定义
```

同时强制 `handler/` 只能通过 `service/` 访问底层逻辑。

**方案 B：拆分 service/**
- 将 `executor.go`, `planner.go`, `filemonitor.go`, `strm.go` 移入 `syncengine/` 或独立的 `app/` 包
- 删除 `job.go`, `taskrun.go`（这些逻辑应在 handler 或 core 中）

---

### 4. **循环依赖风险分析**

#### 当前依赖关系（推测）

```
handler -> service -> syncengine -> syncqueue
            ↓            ↓             ↓
          core      filesystem      core
                       ↓
                     core
```

#### 潜在风险

| 风险场景 | 严重性 | 描述 |
|---------|-------|------|
| `core` 反向依赖 `service` | 🔴 高 | 如果 `core/` 中的 Repository 依赖 `service/` 的类型，会形成循环 |
| `filesystem` 依赖 `syncengine` | 🟡 中 | 驱动层不应依赖引擎层 |
| `handler` 直接依赖 `core` | 🟡 中 | 跳过 `service` 层导致架构混乱 |

**推荐依赖方向**（从外到内）：
```
handler -> service -> domain <- syncengine -> filesystem
                                    ↓             ↓
                                 storage       mediaserver
```

**关键规则**：
1. 所有包只能依赖 `domain/`（核心类型和接口）
2. `storage/` 实现 `domain/` 的接口
3. `handler` 不直接依赖 `storage`，必须通过 `service`
4. `filesystem/`, `mediaserver/` 只实现 `domain/` 的驱动接口

---

## 🎯 改进建议

### 💡 方案 A：最小调整（推荐快速改进）

适用场景：快速修复明显问题，保持现有结构。

#### 1. 修复命名不一致

```bash
# 1. 统一 handler 包名
mv backend/handler backend/handlers
# 修改所有 import 语句

# 2. 重命名 core -> storage
mv backend/core backend/storage
# 修改所有 import 语句

# 3. 重命名 service/executor.go
# 将 service.Executor 改为 service.TaskOrchestrator
```

#### 2. 明确 service 职责

```go
// service/orchestrator.go (原 executor.go)
type TaskOrchestrator struct {
    fileMonitor   FileMonitor
    syncPlanner   SyncPlanner
    strmGenerator StrmGenerator
    logger        *zap.Logger
}
```

#### 3. 移动 main.go

```bash
mkdir -p backend/cmd/strm-server
mv backend/main.go backend/cmd/strm-server/main.go
```

#### 预期结果

```
strm/
├── backend/
│   ├── cmd/
│   │   └── strm-server/
│   │       └── main.go
│   ├── handlers/          # 改名
│   ├── storage/           # 改名（原 core/）
│   ├── service/
│   │   └── orchestrator.go  # 改名（原 executor.go）
│   ├── worker/
│   │   └── executor.go    # 保持
│   └── ...
```

---

### 💡 方案 B：结构性重构（推荐长期维护）

适用场景：希望项目符合 Go 标准布局，便于团队协作和长期维护。

#### 目标结构

```
strm/
├── cmd/
│   └── strm-server/
│       └── main.go
├── internal/
│   ├── domain/              # 核心类型、接口、错误
│   │   ├── models.go        # Job, TaskRun, DataServer 等
│   │   ├── repository.go    # Repository 接口定义
│   │   ├── driver.go        # Driver 接口定义
│   │   └── errors.go
│   ├── storage/             # 数据持久化层
│   │   ├── database.go
│   │   ├── job_repo.go
│   │   └── dataserver_repo.go
│   ├── http/                # HTTP 传输层
│   │   ├── server.go
│   │   ├── job_handler.go
│   │   ├── dataserver_handler.go
│   │   └── middleware.go
│   ├── app/                 # 应用服务层（用例编排）
│   │   ├── sync_orchestrator.go
│   │   ├── sync_planner.go
│   │   ├── file_monitor.go
│   │   └── strm_generator.go
│   ├── sync/                # 同步系统核心
│   │   ├── engine/          # 同步引擎
│   │   ├── queue/           # 任务队列
│   │   ├── worker/          # Worker 池
│   │   └── scheduler/       # Cron 调度
│   ├── filesystem/          # 文件系统驱动
│   │   ├── clouddrive2/
│   │   ├── openlist/
│   │   └── local/
│   ├── mediaserver/         # 媒体服务器客户端
│   │   ├── emby/
│   │   └── jellyfin/
│   └── infra/               # 基础设施
│       ├── logger/
│       ├── crypto/
│       └── config/
├── tests/
│   ├── e2e/
│   └── fixtures/
├── docs/
├── go.mod
└── README.md
```

#### 重构步骤

1. **创建 domain/ 包**
   ```bash
   mkdir -p internal/domain
   # 将 core/models.go 移入 domain/
   # 提取所有 Repository 接口到 domain/repository.go
   ```

2. **重组分层结构**
   ```bash
   mkdir -p internal/{storage,http,app,sync,infra}
   # 按职责迁移各模块
   ```

3. **建立依赖规则**
   - 所有包导入 `internal/domain`
   - 禁止 `domain` 导入其他内部包
   - 强制 `http` 只依赖 `app` 和 `domain`

4. **渐进式迁移**
   - 第 1 周：迁移 domain 和 storage
   - 第 2 周：迁移 http 和 app
   - 第 3 周：整合 sync 模块
   - 第 4 周：测试和验证

---

## 📊 对比总结

| 维度 | 当前状态 | 方案 A（最小调整） | 方案 B（结构性重构） |
|------|---------|-----------------|-------------------|
| **符合 Go 标准** | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **包职责清晰度** | ⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **命名一致性** | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **循环依赖风险** | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **实施成本** | - | 🟢 低（1-2天） | 🔴 高（2-4周） |
| **破坏性变更** | - | 🟢 小 | 🔴 大 |
| **长期可维护性** | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

---

## ✅ 具体行动清单

### 🚀 优先级 P0（立即修复）

- [ ] **修复 handler/ 包名不一致**
  - 将目录 `handler/` 改为 `handlers/`
  - 或将包声明改为 `package handler`（推荐单数）

- [ ] **消除 executor 命名冲突**
  - `service/executor.go` → `service/orchestrator.go`
  - `service.Executor` → `service.TaskOrchestrator`

- [ ] **移动 main.go 到标准位置**
  - 创建 `cmd/strm-server/` 目录
  - 移动 `backend/main.go` → `cmd/strm-server/main.go`

### 📋 优先级 P1（近期改进）

- [ ] **重命名 core/ 为 storage/**
  - 提高包职责清晰度
  - 避免"垃圾桶"包

- [ ] **明确 service/ 职责**
  - 文档化 service 包的边界
  - 强制 handler 只通过 service 访问底层

- [ ] **添加依赖检查**
  - 使用 `go mod graph` 检测循环依赖
  - 配置 CI 检查依赖方向

### 🎯 优先级 P2（长期规划）

- [ ] **评估结构性重构的必要性**
  - 如果团队规模扩大（> 5 人）
  - 如果项目复杂度持续增长
  - 如果需要提取可复用 SDK

- [ ] **引入 domain/ 层**
  - 集中管理核心类型和接口
  - 解耦业务逻辑和实现细节

---

## 📚 参考资料

- [golang-standards/project-layout](https://github.com/golang-standards/project-layout) - Go 项目标准布局
- [Effective Go](https://go.dev/doc/effective_go) - Go 官方最佳实践
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) - Google Go 团队代码审查指南
- [Clean Architecture in Go](https://github.com/bxcodec/go-clean-arch) - Go 清洁架构示例
- [Domain-Driven Design in Go](https://github.com/marcusolsson/goddd) - Go DDD 实践

---

## 📝 变更记录

| 日期 | 版本 | 变更内容 | 作者 |
|------|------|---------|------|
| 2026-02-18 | v1.0 | 初始版本，完成架构分析 | Claude + Codex |

---

## 🤝 贡献

如有补充或修改建议，请通过以下方式反馈：
- 创建 Issue
- 提交 Pull Request
- 联系项目维护者

---

**文档结束**
