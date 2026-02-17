# STRMSync 项目总结

## 项目概述

STRMSync 是一个自动化 STRM 媒体文件管理系统，支持本地文件系统、CloudDrive2 和 OpenList 三种数据源类型，提供文件扫描、实时监控、元数据同步和媒体库通知等功能。

## 开发阶段完成情况

### ✅ Phase 0: 环境清理和准备
- 清理旧代码和配置
- 设置项目结构

### ✅ Phase 1: 项目骨架
- Go 后端基础架构
- 数据库模型设计
- 配置管理系统

### ✅ Phase 2: 本地文件扫描
- 并发扫描服务
- 文件哈希计算
- STRM 文件生成
- 数据库索引

### ✅ Phase 3: CloudDrive2 集成
- CloudDrive2 适配器
- API 认证
- 路径映射

### ✅ Phase 4: OpenList 集成
- WebDAV 适配器
- 挂载点支持

### ✅ Phase 5: 文件监控和增量更新
- fsnotify 实时监控
- 事件去抖机制
- 增量文件处理

### ✅ Phase 6: 元数据同步
- NFO 文件同步
- 海报/背景图同步
- 字幕文件同步
- 变更检测

### ✅ Phase 7: 媒体库通知
- Emby Provider
- Jellyfin Provider
- 重试机制
- 任务记录

### ✅ Phase 8: Web 前端
- Vue 3 + Element Plus
- 仪表盘页面
- 数据源管理页面
- API 接口封装
- 暗色模式支持

### 🔄 Phase 9: 测试和优化
- 测试环境配置
- API 测试脚本
- 待进行完整测试

## 技术栈

### 后端
- **语言**: Go 1.19+
- **Web框架**: Gin
- **数据库**: SQLite + GORM
- **文件监控**: fsnotify
- **日志**: zap
- **配置**: 环境变量

### 前端
- **框架**: Vue 3 (Composition API)
- **UI**: Element Plus 2
- **构建**: Vite 5
- **路由**: Vue Router 4
- **状态**: Pinia 2
- **HTTP**: Axios

## 项目结构

```
strm/
├── backend/
│   ├── cmd/server/          # 主程序入口
│   ├── internal/
│   │   ├── adapters/        # 数据源适配器
│   │   ├── config/          # 配置管理
│   │   ├── database/        # 数据模型
│   │   ├── handlers/        # HTTP处理器
│   │   ├── services/        # 业务逻辑
│   │   │   ├── notifiers/   # 通知Provider
│   │   │   ├── scanner.go   # 扫描服务
│   │   │   ├── watcher.go   # 监控服务
│   │   │   ├── metadata.go  # 元数据服务
│   │   │   └── notifier.go  # 通知服务
│   │   └── utils/           # 工具函数
│   └── go.mod
│
├── frontend/
│   ├── src/
│   │   ├── api/             # API封装
│   │   ├── layouts/         # 布局组件
│   │   ├── views/           # 页面组件
│   │   ├── router/          # 路由配置
│   │   └── assets/          # 静态资源
│   └── package.json
│
├── scripts/
│   ├── test-start.sh        # 测试启动脚本
│   └── test-api.sh          # API测试脚本
│
├── test/
│   ├── media/               # 测试媒体文件
│   └── output/              # STRM输出目录
│
├── build/
│   └── strmsync             # 编译后的可执行文件
│
├── data/                    # 数据库文件
├── logs/                    # 日志文件
└── .env.test                # 测试环境配置
```

## 核心功能

### 1. 数据源管理
- 支持三种类型：Local、CloudDrive2、OpenList
- 路径映射配置
- 连接测试
- 启用/禁用控制

### 2. 文件扫描
- 并发扫描（可配置并发数）
- 批量入库（可配置批次大小）
- 文件哈希计算（快速哈希算法）
- STRM 文件生成
- 增量检测（size + mtime）

### 3. 文件监控
- 基于 fsnotify 的实时监控
- 递归目录监控
- 事件去抖（1秒）
- 动态目录添加
- 失败重试机制

### 4. 元数据同步
- NFO 文件
- 海报/背景图（poster.jpg, fanart.jpg 等）
- 字幕文件（.srt, .ass, .ssa, .sub, .vtt）
- 变更检测（mtime + size）
- 复制模式（跨平台兼容）

### 5. 媒体库通知
- Emby 支持
- Jellyfin 支持
- 自动通知（扫描/监控/元数据同步后）
- 手动触发
- 重试机制（指数退避）
- 事件去抖（5秒）

### 6. Web 管理界面
- 仪表盘（系统概览）
- 数据源管理（CRUD操作）
- 实时状态更新
- 暗色模式
- 响应式设计

## API 接口

### 健康检查
- `GET /api/health` - 系统健康检查

### 数据源管理
- `GET /api/sources` - 获取数据源列表
- `GET /api/sources/:id` - 获取数据源详情
- `POST /api/sources` - 创建数据源
- `PUT /api/sources/:id` - 更新数据源
- `DELETE /api/sources/:id` - 删除数据源
- `POST /api/sources/:id/test` - 测试连接
- `POST /api/sources/:id/scan` - 触发扫描

### 文件监控
- `POST /api/sources/:id/watch/start` - 启动监控
- `POST /api/sources/:id/watch/stop` - 停止监控
- `GET /api/sources/:id/watch/status` - 查询监控状态

### 元数据同步
- `POST /api/sources/:id/metadata/sync` - 同步整个数据源
- `POST /api/sources/:id/metadata/sync/path` - 同步指定路径

### 媒体库通知
- `POST /api/notify/refresh` - 全局刷新
- `POST /api/sources/:id/notify/refresh` - 按数据源刷新

## 测试指南

### 后端测试

1. **设置环境变量**

```bash
export $(cat .env.test | grep -v '^#' | xargs)
```

2. **启动服务**

```bash
./scripts/test-start.sh
```

或手动启动：

```bash
./build/strmsync
```

3. **API 测试**

在另一个终端运行：

```bash
./scripts/test-api.sh
```

### 前端测试

1. **安装依赖**

```bash
cd frontend
npm install
```

2. **启动开发服务器**

```bash
npm run dev
```

3. **访问界面**

打开浏览器访问：http://localhost:5173

### 完整测试流程

1. 启动后端服务（端口3000）
2. 启动前端服务（端口5173）
3. 在界面中创建本地数据源
   - 名称：本地测试媒体库
   - 类型：Local
   - 源路径：`/mnt/c/Users/wff19/Desktop/strm/test/media`
   - 目标路径：`/mnt/c/Users/wff19/Desktop/strm/test/output`
4. 点击"扫描"按钮触发扫描
5. 查看 `test/output` 目录中生成的 STRM 文件
6. 启动文件监控
7. 在 `test/media` 中添加/修改文件，观察自动处理
8. 测试元数据同步和媒体库通知功能

## 配置说明

### 环境变量

```bash
# 服务器配置
PORT=3000                    # HTTP端口
HOST=0.0.0.0                 # 监听地址

# 数据库配置
DB_PATH=data/strmsync.db     # 数据库文件路径

# 日志配置
LOG_LEVEL=info               # debug/info/warn/error
LOG_PATH=logs                # 日志目录

# 加密密钥
ENCRYPTION_KEY=your_key_here # 32位以上密钥

# 扫描服务
SCANNER_CONCURRENCY=20       # 并发数
SCANNER_BATCH_SIZE=500       # 批次大小

# 通知服务
NOTIFIER_ENABLED=true                    # 是否启用
NOTIFIER_PROVIDER=emby                   # emby/jellyfin
NOTIFIER_BASE_URL=http://localhost:8096  # 服务器地址
NOTIFIER_TOKEN=your_token_here           # API Token
NOTIFIER_TIMEOUT=10                      # 超时（秒）
NOTIFIER_RETRY_MAX=3                     # 最大重试次数
NOTIFIER_RETRY_BASE_MS=1000              # 重试基础延迟（毫秒）
NOTIFIER_DEBOUNCE=5                      # 去抖时间（秒）
NOTIFIER_SCOPE=global                    # global/source
```

## 已知问题和优化建议

### 已知问题

1. ❌ 前端其他页面（文件浏览器、任务管理等）未完成
2. ❌ WebSocket 实时推送未实现（当前使用轮询）
3. ❌ 用户认证和权限管理未实现
4. ❌ 配置管理界面未实现（当前通过环境变量）

### 优化建议

1. **性能优化**
   - 实现文件扫描的断点续传
   - 添加缓存层减少数据库查询
   - 优化大目录的扫描速度

2. **功能增强**
   - 支持更多元数据类型
   - 支持自定义通知模板
   - 添加任务队列管理
   - 支持定时扫描任务

3. **用户体验**
   - 完善所有前端页面
   - 添加实时WebSocket推送
   - 添加操作审计日志
   - 支持多语言

4. **运维支持**
   - Docker 容器化部署
   - 健康检查和监控指标
   - 自动备份和恢复
   - 性能监控面板

## 部署建议

### Docker 部署（推荐）

```dockerfile
# Dockerfile
FROM golang:1.19 AS backend-builder
WORKDIR /app
COPY backend/ .
RUN go build -o strmsync ./cmd/server

FROM node:18 AS frontend-builder
WORKDIR /app
COPY frontend/ .
RUN npm install && npm run build

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend-builder /app/strmsync .
COPY --from=frontend-builder /app/dist ./public
EXPOSE 3000
CMD ["./strmsync"]
```

### 二进制部署

1. 编译后端：`go build -o strmsync ./cmd/server`
2. 构建前端：`npm run build`
3. 配置环境变量
4. 使用 systemd 或 supervisor 管理服务

## 总结

STRMSync 项目已完成主要功能的开发，包括：

✅ 完整的后端服务（数据源管理、扫描、监控、元数据、通知）
✅ 基础的 Web 管理界面
✅ 三种数据源类型支持
✅ 实时文件监控
✅ 元数据自动同步
✅ 媒体库通知集成

项目采用现代化的技术栈，代码结构清晰，易于维护和扩展。后续可根据实际使用需求，继续完善前端界面和增加新功能。

## 许可证

MIT

## 作者

STRMSync Team
