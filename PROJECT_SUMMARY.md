# STRMSync 项目文档总结

## 📁 项目结构

```
strm/
├── README.md                          # 项目总览和快速开始指南
├── API_CONTEXT.md                     # API 对接总览
├── WEB_UI_DESIGN.md                   # Web 界面设计规范（40KB）
│
├── config/
│   └── config.example.yaml            # 启动配置示例（最小化）
│
└── docs/
    ├── CloudDrive2_API.md             # CloudDrive2 完整 API 文档（20KB+）
    ├── OpenList_API.md                # OpenList 完整 API 文档（18KB+）
    ├── IMPLEMENTATION_PLAN.md         # 详细实施方案（40KB+）
    └── CONFIG_MANAGEMENT.md           # 配置管理方案（20KB+）
```

---

## 📚 文档说明

### 1. README.md（主文档）
**内容**:
- 项目简介和核心特性
- 快速开始指南（Docker 部署）
- 使用指南（数据源管理、文件浏览、任务管理）
- 技术架构和性能指标
- FAQ

**关键信息**:
- ✅ 配置通过 Web 界面管理
- ✅ CloudDrive2 仅支持挂载模式
- ✅ OpenList 支持 HTTP 和本地两种模式
- ✅ 首次启动只需配置 `config.yaml`（最小化）

---

### 2. API_CONTEXT.md（API 总览）
**内容**:
- CloudDrive2/OpenList/Emby/Jellyfin API 快速参考
- 路径映射规则
- 数据规模和性能要求
- 排除规则配置

---

### 3. docs/CloudDrive2_API.md（20KB+）
**详细文档**:
- gRPC API 完整接口定义
- 认证方式（密码/Token 两种）
- 文件操作、挂载管理、云盘集成、传输任务
- Go 客户端示例代码
- 错误处理和限流策略
- STRMSync 集成方案

**关键规则**:
```
CloudDrive2 STRM 生成规则（仅本地挂载）:
源文件: /mnt/clouddrive/115/Movies/Action/Movie.mkv
STRM 文件: /media/library/Movies/Action/Movie.strm
STRM 内容: /mnt/clouddrive/115/Movies/Action/Movie.mkv
```

---

### 4. docs/OpenList_API.md（18KB+）
**详细文档**:
- REST API 完整接口定义
- 认证方式（密码/Token 两种）
- 文件操作（列表、搜索、上传、下载）
- Go 客户端示例代码
- STRMSync 集成方案

**关键规则**:
```
OpenList HTTP 模式:
源文件: /Movies/Action/Movie.mkv
STRM 文件: /media/library/Movies/Action/Movie.strm
STRM 内容: http://localhost:5244/d/Movies/Action/Movie.mkv

OpenList 本地模式:
源文件: /Movies/Action/Movie.mkv（OpenList 内部路径）
挂载路径: /mnt/openlist
STRM 文件: /media/library/Movies/Action/Movie.strm
STRM 内容: /mnt/openlist/Movies/Action/Movie.mkv
```

---

### 5. docs/IMPLEMENTATION_PLAN.md（40KB+）
**详细实施方案**:

#### 1. STRM 生成规则
- CloudDrive2（仅本地挂载）
- OpenList（HTTP/本地两种模式）
- 本地文件系统
- 完整流程图

#### 2. 数据库设计
- 5 张表：sources、files、metadata_files、tasks、settings
- 完整的 SQL 和 GORM 模型定义

#### 3. 核心服务架构
- Orchestrator（编排器）
- Scanner（扫描器）
- Watcher（监控器）
- Metadata Sync（元数据同步）
- Notifier（通知器）
- 服务职责和接口定义

#### 4. 数据源适配器设计
- 统一的 Adapter 接口
- LocalAdapter 实现
- CloudDrive2Adapter 实现（gRPC）
- OpenListAdapter 实现（REST + 两种 STRM 模式）

#### 5. 实施步骤（10 周计划）
- 阶段 1: 项目骨架（1 周）
- 阶段 2: 本地文件扫描（1 周）
- 阶段 3: CloudDrive2 集成（1 周）
- 阶段 4: OpenList 集成（1 周）
- 阶段 5: 文件监控（1 周）
- 阶段 6: 元数据同步（1 周）
- 阶段 7: 媒体库通知（1 周）
- 阶段 8: Web 前端（2 周）
- 阶段 9: 测试优化（1 周）

#### 6. API 设计
- 数据源 API
- 任务 API
- 文件 API

#### 7. Docker 部署方案
- 多阶段构建 Dockerfile
- docker-compose.yml

---

### 6. docs/CONFIG_MANAGEMENT.md（20KB+）
**配置管理方案**:

#### 配置分层
```
启动配置（config.yaml）
    ↓
数据库配置（settings 表 + sources 表 + notifiers 表）
    ↓
Web 界面管理
```

#### 数据库配置结构
- **settings 表**: 系统设置（通用、扫描、STRM、定时任务）
- **sources 表**: 数据源配置（存储 JSON 配置）
- **notifiers 表**: 通知器配置（Emby/Jellyfin）

#### Web 界面设计
- 系统设置页面（5 个选项卡）
- 数据源管理页面（添加/编辑/测试）
- 通知器管理页面

#### API 接口
- GET/PUT `/api/settings`
- GET/POST/PUT/DELETE `/api/sources`
- GET/PUT/POST `/api/notifiers`

#### 配置加密
- AES-256-GCM 加密
- 密码和 API Key 加密存储
- 格式: `encrypted:base64string`

---

### 7. WEB_UI_DESIGN.md（40KB）
**完整的 UI 设计**:
- 5 个核心页面设计（仪表盘、数据源、任务、文件、设置）
- 布局和交互设计
- 技术栈选型（Vue 3 + Element Plus）
- 颜色方案
- API 接口定义
- 响应式设计

---

## 🎯 核心特性总结

### STRM 生成规则

| 数据源 | 模式 | STRM 内容 |
|--------|------|-----------|
| 本地文件系统 | - | 本地路径 |
| CloudDrive2 | 挂载模式（仅支持） | 本地挂载路径 |
| OpenList | HTTP 模式 | HTTP 下载链接 |
| OpenList | 本地模式 | 本地挂载路径 |

### 认证方式支持

| 服务 | 认证方式 |
|------|----------|
| CloudDrive2 | 密码 / Token |
| OpenList | 密码 / Token |
| Emby | API Key / 密码 |
| Jellyfin | API Key / 密码 |

### 配置管理

- ✅ **启动配置**: `config.yaml`（最小化，仅端口/数据库/日志）
- ✅ **业务配置**: 数据库（通过 Web 界面管理）
- ✅ **敏感信息**: AES-256 加密存储
- ✅ **配置测试**: Web 界面支持测试连接

---

## 🚀 开发优先级

### P0（Critical）
1. 项目骨架 + Docker 环境
2. 本地文件扫描
3. STRM 生成和数据库索引
4. Web 界面（数据源管理）

### P1（High）
1. CloudDrive2 集成（挂载模式）
2. OpenList 集成（HTTP + 本地两种模式）
3. 文件监控（fsnotify）
4. Web 界面（任务管理）

### P2（Medium）
1. 元数据同步
2. 媒体库通知（Emby/Jellyfin）
3. Web 界面（文件浏览、系统设置）

### P3（Low）
1. 性能优化
2. 单元测试
3. 文档完善

---

## 📊 预计开发周期

**总计**: 10 周

- **第 1-3 周**: 核心功能（框架 + 本地扫描 + CloudDrive2）
- **第 4-5 周**: OpenList 集成 + 文件监控
- **第 6-7 周**: 元数据 + 通知器
- **第 8-9 周**: Web 前端
- **第 10 周**: 测试优化

---

## ✅ 下一步行动

1. **初始化 Go 项目**:
   ```bash
   cd backend
   go mod init github.com/strmsync/strmsync
   ```

2. **创建项目目录结构**:
   ```bash
   mkdir -p backend/{cmd/server,internal/{config,database,adapters,services,api/handlers,utils}}
   ```

3. **开始阶段 1**: 实施项目骨架（见 IMPLEMENTATION_PLAN.md）

---

**文档版本**: 1.0.0
**最后更新**: 2024-02-16
**作者**: STRMSync Team
**文档总计**: 8 个文件，约 150KB
