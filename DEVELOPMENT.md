# STRMSync - 自动化STRM媒体文件管理系统

> 基于Go + Vue 3的高性能STRM文件管理系统，支持CloudDrive2 gRPC集成

[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org)
[![Vue Version](https://img.shields.io/badge/Vue-3.x-green.svg)](https://vuejs.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## 🎯 项目状态

当前开发阶段：**Phase 2 - 全链路集成与优化**

### 已完成
- ✅ 数据库模型层（GORM + SQLite）
- ✅ Handler层和API路由
- ✅ CloudDrive2 gRPC集成（proto 0.9.24）
- ✅ Service层核心组件（Job、TaskRun、Executor、Planner、StrmGenerator）
- ✅ 并发安全优化（竞态窗口消除、Cancel幂等性）
- ✅ 前端页面和组件重构（Vue 3 + Composition API）
- ✅ 全链路测试和代码清理
- ✅ Filesystem客户端完善（provider模式扩展）

### 待开发
- ⏳ Docker部署方案
- ⏳ 完整的E2E自动化测试

---

## 🏗️ 技术架构

### 后端
- **语言**: Go 1.24.0
- **框架**: Gin（HTTP）+ gRPC
- **数据库**: SQLite + GORM
- **日志**: Zap（结构化日志）
- **并发**: errgroup + context管理

### 前端
- **框架**: Vue 3（Composition API + `<script setup>`）
- **UI库**: Element Plus
- **构建**: Vite 5
- **HTTP客户端**: Axios
- **路由**: Vue Router

### CloudDrive2集成
- **协议**: gRPC (h2c)
- **Proto**: v0.9.24
- **功能**: 文件操作、路径遍历

---

## 📂 项目结构

```
strm/
├── VERSION                     # 统一版本号（前后端共享）
├── backend/                    # Go 后端
│   ├── cmd/                    # 命令行入口
│   │   └── server/             # HTTP 服务器
│   │       └── main.go         # 应用入口
│   ├── internal/               # 内部包（不对外暴露）
│   ├── go.mod
│   └── go.sum
│
├── frontend/                   # Vue 3 前端
│   ├── src/
│   ├── index.html
│   ├── vite.config.js
│   └── package.json
│
├── dist/                       # 构建产物（可执行文件 + web_statics + .env）
│   ├── strmsync
│   ├── web_statics/
│   └── .env
│
├── docs/                       # 文档
│   ├── CloudDrive2_Integration.md
│   ├── CloudDrive2_API.md
│   ├── Emby_API.md
│   ├── Jellyfin_API.md
│   ├── OpenList_API.md
│   └── README.md               # 文档索引
│
├── scripts/                    # 脚本
│   ├── prod-start.sh           # 启动生产环境
│   ├── prod-stop.sh            # 停止生产环境
│   └── prod-restart.sh         # 重启生产环境
│
├── tests/                      # 开发/测试目录
│   ├── .env.test               # 测试环境变量
│   ├── .air.toml               # 测试环境 Air 配置
│   ├── data/                   # 测试数据库
│   ├── logs/                   # 测试日志
│   ├── media/                  # 测试媒体文件
│   └── out/                    # 测试输出
│
├── .env.example                # 环境变量示例
├── Makefile                    # 构建脚本
└── DEVELOPMENT.md              # 本文件
```

---

## 🚀 快速开始

### 环境准备

**系统要求**:
- Go 1.24+
- Node.js 18+（Vite 5要求）
- Make（可选）

### 一键开发（推荐）

```bash
make dev
```

开发环境默认读取 `tests/.env.test`，日志与数据库目录位于 `tests/` 下。

### Git Hooks 设置（必需）

为保证 README 版本徽章与 `VERSION` 同步，请在首次克隆后执行一次：

```bash
./scripts/setup-hooks.sh
```

该脚本会：
- 设置 `git config core.hooksPath .githooks`
- 确保 `.githooks/pre-commit` 可执行

### 合并部署构建

```bash
make build
```

构建产物位于 `dist/`：
- `dist/strmsync`（可执行文件）
- `dist/web_statics/`（前端静态文件）
- `dist/.env`（由项目根目录 `.env` 复制，若不存在则从 `.env.example` 生成）
- `dist/prod-start.sh`（启动脚本）

运行（工作目录为可执行文件所在目录）：

```bash
./dist/prod-start.sh
# 或
make run
```

### 后端开发（可选）

```bash
cd backend

# 安装依赖
go mod download

# 运行服务（默认端口 6786）
go run ./cmd/server

# 构建
go build ./cmd/server
```

### 前端开发（可选）

```bash
cd frontend

# 安装依赖
npm install

# 开发模式
npm run dev
```

### 一键启动（开发环境）

```bash
./scripts/dev-start.sh
```

或：

```bash
make dev
```

### build/ 目录说明

```
build/
  go/      # Go 编译缓存/临时二进制
  vue/     # Vue 构建产物/缓存
  air/     # air 热重载临时文件
```

---

## 🧪 测试

### 后端测试

```bash
cd backend

# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/app/job
go test ./internal/app/sync
```

### 前端开发验证

```bash
cd frontend

# 开发模式（热重载）
npm run dev

# 生产构建测试
npm run build
```

---

## 📚 文档索引

### API文档
- [后端 API 文档](backend/README.md) - 后端 HTTP API 详细说明
- [CloudDrive2 API](docs/CloudDrive2_API.md) - CloudDrive2 gRPC API参考
- [Emby API](docs/Emby_API.md) - Emby 媒体服务器 API 参考
- [Jellyfin API](docs/Jellyfin_API.md) - Jellyfin 媒体服务器 API 参考
- [OpenList API](docs/OpenList_API.md) - OpenList API参考

### 集成文档
- [CloudDrive2集成文档](docs/CloudDrive2_Integration.md) - gRPC集成详细说明
- [CloudDrive2 gRPC设置](docs/CloudDrive2_gRPC_Setup.md) - gRPC配置指南

### 运维文档
- [部署文档](docs/DEPLOYMENT.md) - 生产环境部署指南

---

## 🔧 API概览

### HTTP API（端口：6786）

**数据服务器**
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/servers/data` | GET | 获取数据服务器列表 |
| `/api/servers/data` | POST | 创建数据服务器 |
| `/api/servers/data/:id` | GET | 获取数据服务器详情 |
| `/api/servers/data/:id` | PUT | 更新数据服务器 |
| `/api/servers/data/:id` | DELETE | 删除数据服务器 |
| `/api/servers/data/:id/test` | POST | 测试数据服务器连接 |
| `/api/servers/data/test` | POST | 临时测试数据服务器 |

**媒体服务器**
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/servers/media` | GET | 获取媒体服务器列表 |
| `/api/servers/media` | POST | 创建媒体服务器 |
| `/api/servers/media/:id` | GET | 获取媒体服务器详情 |
| `/api/servers/media/:id` | PUT | 更新媒体服务器 |
| `/api/servers/media/:id` | DELETE | 删除媒体服务器 |
| `/api/servers/media/:id/test` | POST | 测试媒体服务器连接 |
| `/api/servers/media/test` | POST | 临时测试媒体服务器 |

**服务器类型**
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/servers/types` | GET | 获取服务器类型列表 |
| `/api/servers/types/:type` | GET | 获取服务器类型详情 |

**任务管理**
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/jobs` | GET | 获取任务列表 |
| `/api/jobs` | POST | 创建任务 |
| `/api/jobs/:id` | GET | 获取任务详情 |
| `/api/jobs/:id` | PUT | 更新任务 |
| `/api/jobs/:id` | DELETE | 删除任务 |
| `/api/jobs/:id/run` | POST | 触发任务执行 |
| `/api/jobs/:id/stop` | POST | 停止任务 |
| `/api/jobs/:id/enable` | PUT | 启用任务 |
| `/api/jobs/:id/disable` | PUT | 禁用任务 |

**运行记录**
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/runs` | GET | 获取运行记录列表 |
| `/api/runs/:id` | GET | 获取运行记录详情 |
| `/api/runs/:id/cancel` | POST | 取消运行中的任务 |
| `/api/runs/stats` | GET | 获取运行统计 |

**文件浏览**
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/files/directories` | GET | 获取目录列表 |
| `/api/files/list` | POST | 获取文件列表 |

**系统**
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/health` | GET | 健康检查 |
| `/api/logs` | GET | 获取日志 |
| `/api/logs/cleanup` | POST | 清理日志 |
| `/api/settings` | GET | 获取系统设置 |
| `/api/settings` | PUT | 更新系统设置 |

**日志环境变量（摘录）**
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LOG_DEBUG` | `false` | 是否强制开启 debug（覆盖 LOG_LEVEL） |
| `LOG_DEBUG_MODULES` | `engine,worker,filesystem` | debug 模块白名单 |
| `LOG_DEBUG_RPS` | `10` | debug 速率限制（每秒最大条数） |

详细API文档请参考 [backend/README.md](backend/README.md)

---

## 🎨 核心特性

### 分层架构

采用清晰的分层架构：

1. **Transport层** (`internal/transport`) - HTTP请求处理
   - 路由注册和请求验证
   - 请求/响应数据转换
   - 错误处理和状态码映射

2. **App层** (`internal/app`) - 业务逻辑
   - **job**: Job生命周期管理（并发安全）
   - **taskrun**: TaskRun记录管理
   - **sync**: 同步执行器和计划器
   - **file**: 文件处理和STRM生成

3. **Domain层** (`internal/domain`) - 领域模型
   - 数据模型定义（GORM）
   - 仓库接口
   - 业务规则验证

4. **Infra层** (`internal/infra`) - 基础设施
   - **filesystem**: 文件系统客户端（Provider模式）
   - **mediaserver**: 媒体服务器客户端（Adapter模式）
   - **db**: 数据库配置和连接管理

### Filesystem Provider模式

统一的文件系统抽象，支持多种数据源：
- **Local**: 本地文件系统
- **CloudDrive2**: gRPC集成（h2c）
- **OpenList**: HTTP API集成
- **WebDAV**: WebDAV协议支持

### MediaServer Adapter模式

统一的媒体服务器接口，支持多种媒体服务器：
- **Emby**: Emby Server适配器
- **Jellyfin**: Jellyfin Server适配器
- **Plex**: Plex Media Server适配器（规划中）

### 并发安全保障

- **统一context管理**: 整个Run生命周期共享cancelFunc
- **原子操作**: placeholder机制防止竞态
- **幂等性**: Cancel操作支持重复调用
- **防御性检查**: ensureTaskRunCancelled兜底
- **路径验证**: Abs+Clean+Rel防止路径穿越

### CloudDrive2集成

- **gRPC h2c连接**: 支持HTTP/2明文通信
- **Proto v0.9.24**: 最新协议版本
- **健康检查**: SystemReady + HasError双重验证
- **完整测试**: 11项功能测试全面覆盖

---

## 🤝 贡献指南

### 开发规范

- **代码风格**: Go使用gofmt，Vue使用ESLint
- **提交规范**: Conventional Commits
  ```
  feat: 新功能
  fix: 修复
  docs: 文档
  refactor: 重构
  test: 测试
  chore: 构建/工具
  ```
- **分支策略**:
  - `master`: 稳定版本
  - `develop`: 开发分支
  - `feature/*`: 功能分支
- **测试**: 核心逻辑需有单元测试

---

## 📝 更新日志

### Phase 2 (2026-02-19)

**前端重构**
- 完成所有页面组件重构（Vue 3 Composition API）
- 实现响应式列表标准化（normalizeListResponse）
- 添加用户体验优化（tooltip、局部loading状态）
- 前后端API字段对齐（cron/status/enabled格式）

**代码质量**
- 删除未使用的组件和文件
- 统一错误日志规范
- Go依赖整理（go mod tidy）
- .gitignore规则完善

**文档完善**
- 更新README（项目结构、API列表、环境要求）
- 创建HTTP API文档
- 创建部署文档

### Phase 1 (2026-02-18)

**架构重构**
- 完成App层核心组件（job/taskrun/sync/file）
- 消除并发竞态窗口（3次Codex review迭代）
- 实现Cancel幂等性和防御性检查
- 路径验证强化（防路径穿越）

**CloudDrive2集成**
- 升级proto 0.6.4-beta → 0.9.24
- 升级gRPC v1.56.3 → v1.79.1
- 完成11项功能测试（100%通过）
- 创建完整集成文档

**技术债修复**
- Transport层类型安全（自定义枚举、sentinel errors）
- 数据库并发控制（SELECT FOR UPDATE、uniqueIndex）
- 批量更新优化（单SQL + RowsAffected检查）

### Phase 0 (2024-02-16)

- 项目初始化和架构设计
- 数据库模型设计（GORM）
- Transport基础框架

---

## 📄 许可证

MIT License

---

## 🙏 致谢

- [Gin](https://github.com/gin-gonic/gin) - HTTP框架
- [GORM](https://gorm.io/) - ORM库
- [gRPC-Go](https://github.com/grpc/grpc-go) - gRPC框架
- [Vue 3](https://vuejs.org/) - 前端框架
- [Element Plus](https://element-plus.org/) - UI库
- [CloudDrive2](https://www.clouddrive2.com/) - 云盘挂载

---

**Author**: STRMSync Team
**Current Phase**: Phase 2 - Integration & Optimization
**Last Update**: 2026-02-19
**Go Version**: 1.24.0
**Vue Version**: 3.x
