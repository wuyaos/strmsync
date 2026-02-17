# STRMSync - 自动化STRM媒体文件管理系统

> 基于Go + Vue 3的高性能STRM文件管理系统，支持CloudDrive2 gRPC集成

[![Go Version](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org)
[![Vue Version](https://img.shields.io/badge/Vue-3.x-green.svg)](https://vuejs.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## 🎯 项目状态

当前开发阶段：**Phase 1 - Service业务逻辑层重构**

### 已完成
- ✅ 数据库模型层（GORM + SQLite）
- ✅ Handler层和API路由
- ✅ CloudDrive2 gRPC集成（proto 0.9.24）
- ✅ Service层核心组件（Job、TaskRun、Executor、Planner、StrmGenerator）
- ✅ 并发安全优化（竞态窗口消除、Cancel幂等性）

### 进行中
- 🔄 Service层完善（FileMonitor、DataServerClient等）

### 待开发
- ⏳ 前端页面和组件重构
- ⏳ 全链路集成测试
- ⏳ Docker部署方案

---

## 🏗️ 技术架构

### 后端
- **语言**: Go 1.26.0
- **框架**: Gin（HTTP）+ gRPC
- **数据库**: SQLite + GORM
- **日志**: Zap（结构化日志）
- **并发**: errgroup + context管理

### 前端
- **框架**: Vue 3（Composition API）
- **UI库**: Element Plus
- **构建**: Vite
- **状态管理**: Pinia
- **路由**: Vue Router

### CloudDrive2集成
- **协议**: gRPC (h2c)
- **Proto**: v0.9.24
- **连接**: 192.168.123.179:19798
- **功能**: 文件列表、路径遍历、健康检查

---

## 📂 项目结构

```
strm/
├── backend/                    # Go后端
│   ├── cmd/
│   │   └── server/main.go      # 应用入口
│   ├── internal/
│   │   ├── config/             # 配置管理
│   │   ├── database/           # 数据库模型（GORM）
│   │   ├── handlers/           # HTTP API处理器
│   │   ├── service/            # 业务逻辑层
│   │   │   ├── job/            # Job服务（并发控制）
│   │   │   ├── taskrun/        # TaskRun服务
│   │   │   ├── executor/       # 任务执行器
│   │   │   ├── planner/        # 同步计划器
│   │   │   └── strm/           # STRM生成器
│   │   ├── clients/
│   │   │   └── clouddrive2/    # CloudDrive2客户端
│   │   └── utils/              # 工具函数
│   ├── go.mod
│   └── Makefile
│
├── frontend/                   # Vue 3前端（待重构）
│   ├── src/
│   │   ├── views/              # 页面
│   │   ├── api/                # API封装
│   │   └── components/         # 公共组件
│   └── package.json
│
├── docs/                       # 文档
│   ├── CloudDrive2_Integration.md      # 集成文档
│   ├── CloudDrive2_gRPC_Setup.md       # gRPC设置指南
│   ├── CloudDrive2_API.md              # API参考
│   ├── clouddrive.proto                # Proto定义
│   ├── Emby_Jellyfin_API.md            # Emby/Jellyfin API
│   ├── OpenList_API.md                 # OpenList API
│   ├── IMPLEMENTATION_PLAN.md          # 实施计划
│   └── README.md                       # 文档索引
│
├── scripts/                    # 脚本
│   ├── start.sh                # 启动脚本
│   ├── stop.sh                 # 停止脚本
│   ├── test-api.sh             # API测试
│   └── gen_clouddrive2_proto.sh # Proto生成
│
├── tests/                      # 测试目录
│   ├── cmd/                    # 测试工具
│   │   ├── clouddrive2_simple/ # CloudDrive2简单测试
│   │   └── clouddrive2_full/   # CloudDrive2完整测试
│   ├── media/                  # 测试媒体文件
│   ├── output/                 # 测试输出（gitignore）
│   ├── test.env                # 测试环境变量
│   └── .env.test               # 测试配置
│
├── .claude/                    # Claude Code工作目录
│   └── summaries/              # 阶段性总结
│       ├── STAGE0_SUMMARY.md           # 阶段0总结
│       ├── DEVELOPMENT_STATUS.md       # 开发状态
│       ├── PROJECT_SUMMARY.md          # 项目总结
│       ├── START_GUIDE.md              # 启动指南
│       ├── TESTING_GUIDE.md            # 测试指南
│       └── PROJECT_CLEANUP.md          # 项目清理记录
│
├── docker-compose.yml          # Docker配置（待完善）
├── Makefile                    # 构建脚本
├── .gitignore                  # Git忽略规则
└── README.md                   # 本文件
```

---

## 🚀 快速开始

### 环境准备

**系统要求**:
- Go 1.26+
- Node.js 16+
- Make（可选）

**依赖服务**（开发测试）:
- CloudDrive2: http://192.168.123.179:19798

### 后端开发

```bash
cd backend

# 安装依赖
go mod download

# 运行服务
go run cmd/server/main.go

# 或使用Makefile
make run

# 构建
make build

# 测试CloudDrive2连接
go run cmd/test_clouddrive2_full/main.go
```

### 前端开发

```bash
cd frontend

# 安装依赖
npm install

# 开发模式
npm run dev

# 构建
npm run build
```

---

## 🧪 测试

### CloudDrive2集成测试

```bash
# 完整功能测试（11项）
cd backend
go run cmd/test_clouddrive2_full/main.go

# 快速测试
./scripts/test-api.sh
```

### 单元测试

```bash
# 后端测试
cd backend
go test ./...

# 前端测试
cd frontend
npm test
```

---

## 📚 文档索引

### 开发文档
- [CloudDrive2集成文档](docs/CloudDrive2_Integration.md) - gRPC集成详细说明
- [CloudDrive2测试报告](docs/CloudDrive2_Test_Report.md) - 功能测试报告
- [已知问题](docs/CloudDrive2_Known_Issues.md) - CloudDrive2已知问题和解决方案

### 项目总结
- [阶段0总结](.claude/summaries/STAGE0_SUMMARY.md) - 项目初始化和架构设计
- [开发状态](.claude/summaries/DEVELOPMENT_STATUS.md) - 当前开发进度
- [项目总结](.claude/summaries/PROJECT_SUMMARY.md) - 项目整体总结
- [启动指南](.claude/summaries/START_GUIDE.md) - 快速启动指南
- [测试指南](.claude/summaries/TESTING_GUIDE.md) - 测试说明

---

## 🔧 API文档

### HTTP API（端口：6754）

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/health` | GET | 健康检查 |
| `/api/data-servers` | GET/POST | 数据服务器管理 |
| `/api/media-servers` | GET/POST | 媒体服务器管理 |
| `/api/jobs` | GET/POST | 任务管理 |
| `/api/jobs/:id/run` | POST | 运行任务 |
| `/api/jobs/:id/stop` | POST | 停止任务 |
| `/api/task-runs` | GET | TaskRun记录 |

详细API文档见各Handler实现：
- [backend/internal/handlers/data_server.go](backend/internal/handlers/data_server.go)
- [backend/internal/handlers/media_server.go](backend/internal/handlers/media_server.go)
- [backend/internal/handlers/job.go](backend/internal/handlers/job.go)
- [backend/internal/handlers/task_run.go](backend/internal/handlers/task_run.go)

---

## 🎨 核心特性

### Service层架构

采用清晰的三层架构：

1. **Handler层** - HTTP请求处理
2. **Service层** - 业务逻辑（当前重点）
   - JobService: Job生命周期管理（并发安全）
   - TaskRunService: TaskRun记录管理
   - TaskExecutor: 任务执行编排
   - SyncPlanner: 同步计划生成
   - StrmGenerator: STRM文件生成
3. **Database层** - 数据持久化（GORM）

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
- **11项功能测试**: 全面覆盖核心功能

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

### Phase 1 (2026-02-18)

**Service层重构**
- 完成Job/TaskRun/Executor/Planner/StrmGenerator核心组件
- 消除并发竞态窗口（3次Codex review迭代）
- 实现Cancel幂等性和防御性检查
- 路径验证强化（防路径穿越）

**CloudDrive2集成**
- 升级proto 0.6.4-beta → 0.9.24
- 升级gRPC v1.56.3 → v1.79.1
- 完成11项功能测试（100%通过）
- 创建完整集成文档

**技术债修复**
- Handler层类型安全（自定义枚举、sentinel errors）
- 数据库并发控制（SELECT FOR UPDATE、uniqueIndex）
- 批量更新优化（单SQL + RowsAffected检查）

### Phase 0 (2024-02-16)

- 项目初始化和架构设计
- 数据库模型设计（GORM）
- Handler基础框架

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
**Current Phase**: Phase 1 - Service Layer
**Last Update**: 2026-02-18
**Go Version**: 1.26.0
**Vue Version**: 3.x
