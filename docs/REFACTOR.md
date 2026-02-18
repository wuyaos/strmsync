# 代码重构建议

**创建时间**: 2026-02-19
**最后更新**: 2026-02-19 20:45 (Week 4 Day 1)
**维护说明**: 本文档长期维护，每次扫描发现新的重复代码时追加到末尾

---

## 实施状态总览

| 编号 | 项目 | 优先级 | 预估行数 | 状态 | 完成时间 |
|------|------|--------|----------|------|----------|
| #4 | cleanRemotePath 重复 | Low | 10 | ✅ 已完成 | 2026-02-19 |
| #3 | GormRepository 重复 | Medium | 60-80 | ✅ 已完成 | 2026-02-19 |
| #2 | 参数验证逻辑重复 | High | 120 | ✅ 已完成 | 2026-02-19 |
| #1 | Handler CRUD 模式重复 | High | 1000+ | ⏸️ 延后 | - |
| #5 | 客户端初始化模式 | Low | 50 | ⏸️ 延后 | - |
| #6 | Job/Server CRUD 流程相似 | Medium | 200+ | ⏸️ 延后 | - |

**已消除重复代码：~200 行**（#4: 10行 + #3: 70行 + #2: 120行）

---

## 完成的重构详情

### ✅ #4 cleanRemotePath 函数重复（2026-02-19）

**问题：** 相同的路径清理函数在两处重复实现

**解决方案：** 合并 `filesystemdriver` 包到 `filesystem` 包时自动消除重复，保留 `client.go` 中的实现

**修改文件：**
- 删除：`filesystemdriver/` 目录（整个包合并到 filesystem）
- 删除：`filesystem/driver_adapter.go` 中的重复 `cleanRemotePath` 函数

**收益：** ~10 行

---

### ✅ #3 GormRepository 重复（2026-02-19）

**问题：** `GormJobRepository` 和 `GormDataServerRepository` 在多个包中重复实现

**解决方案：** 统一到 `core` 包，scheduler 和 worker 共享实例

**修改文件：**
- 新增：`core/job_repository.go`
- 新增：`core/data_server_repository.go`
- 删除：`scheduler/gorm_repo.go`
- 修改：`worker/executor.go`（删除 GormJobRepository 和 GormDataServerRepository）
- 修改：`main.go`（使用 core.NewGormJobRepository）

**收益：** ~70 行

**设计决策：** `GormTaskRunRepository` 保留在 worker 包（依赖内部类型 TaskRunProgress）

---

### ✅ #2 参数验证逻辑重复（2026-02-19）

**问题：** 参数验证块在 4 处重复（filesystem_server 和 media_server 的 Create/Update）

**解决方案：** 添加组合验证器 `validateServerRequest` 到 `handler/helpers.go`

**修改文件：**
- 修改：`handler/helpers.go`（新增 validateServerRequest）
- 修改：`handler/filesystem_server.go`（Create/Update 各减少 30 行）
- 修改：`handler/media_server.go`（Create/Update 各减少 30 行）

**收益：** ~120 行

---

## 扫描记录 #1 - 2026-02-19

### 1. Handler CRUD 模式重复 ⚠️ **High Priority**

#### 问题描述
`DataServerHandler` 和 `MediaServerHandler` 的 CRUD 操作（Create/List/Get/Update/Delete）存在高度重复：

**涉及文件:**
- `handler/filesystem_server.go` (591 行)
- `handler/media_server.go` (611 行)

**重复模式:**
```go
// Create 流程（两个 Handler 完全相同）:
1. ShouldBindJSON() - 绑定请求
2. 验证必填字段（name, type, host, port）
3. 验证枚举值（type）
4. 验证 JSON 字符串（options）
5. SSRF 防护（validateHostForSSRF）
6. 唯一性检查（WHERE name = ?）
7. 设置默认值（enabled）
8. Create() + 唯一约束错误处理
9. 成功/错误响应

// 其他操作类似重复
```

**差异点仅有:**
- 模型类型：`core.DataServer` vs `core.MediaServer`
- 枚举值：`["clouddrive2", "openlist"]` vs `["emby", "jellyfin", "plex"]`

#### 重构建议

**方案 A: 抽取通用 CRUD 函数（推荐）**
```go
// 在 handler/helpers.go 中添加
func handleServerCreate[T any](
    c *gin.Context,
    db *gorm.DB,
    logger *zap.Logger,
    allowedTypes []string,
    serverType string, // "data" or "media"
) {
    // 统一处理 Create 流程
}

func handleServerList[T any](...)
func handleServerUpdate[T any](...)
// ...
```

**方案 B: 提取 Service 层（更彻底）**
```go
// 新建 service/server_service.go
type ServerService[T any] struct {
    db          *gorm.DB
    logger      *zap.Logger
    allowedTypes []string
}

func (s *ServerService[T]) Create(req ServerRequest) (*T, error)
func (s *ServerService[T]) List(filters ListFilters) ([]T, int64, error)
// ...
```

**预期收益:**
- 减少约 **1000+ 行**重复代码
- 统一错误处理和验证逻辑
- 新增 Server 类型只需实现模型，无需重写 Handler

---

### 2. 参数验证逻辑重复 ⚠️ **High Priority**

#### 问题描述
Create 和 Update 操作中的参数验证块高度重复：

**涉及位置:**
- `handler/filesystem_server.go:48-76` (Create 验证)
- `handler/filesystem_server.go:227-255` (Update 验证)
- `handler/media_server.go:44-72` (Create 验证)
- `handler/media_server.go:223-251` (Update 验证)

**重复代码示例:**
```go
// 在 4 个地方重复出现
var fieldErrors []FieldError
validateRequiredString("name", req.Name, &fieldErrors)
validateRequiredString("type", req.Type, &fieldErrors)
validateRequiredString("host", req.Host, &fieldErrors)
if req.Port == 0 {
    fieldErrors = append(fieldErrors, FieldError{Field: "port", Message: "必填字段不能为空"})
} else {
    validatePort("port", req.Port, &fieldErrors)
}
validateEnum("type", req.Type, allowedTypes, &fieldErrors)
validateJSONString("options", req.Options, &fieldErrors)
if allowed, _, msg := validateHostForSSRF(req.Host); !allowed {
    fieldErrors = append(fieldErrors, FieldError{Field: "host", Message: msg})
}
```

#### 重构建议

在 `handler/helpers.go` 中添加组合验证器：

```go
// validateServerRequest 验证服务器配置请求参数
func validateServerRequest(
    name, stype, host string,
    port int,
    options string,
    allowedTypes []string,
) []FieldError {
    var fieldErrors []FieldError

    validateRequiredString("name", name, &fieldErrors)
    validateRequiredString("type", stype, &fieldErrors)
    validateRequiredString("host", host, &fieldErrors)

    if port == 0 {
        fieldErrors = append(fieldErrors, FieldError{Field: "port", Message: "必填字段不能为空"})
    } else {
        validatePort("port", port, &fieldErrors)
    }

    validateEnum("type", stype, allowedTypes, &fieldErrors)
    validateJSONString("options", options, &fieldErrors)

    if allowed, _, msg := validateHostForSSRF(host); !allowed {
        fieldErrors = append(fieldErrors, FieldError{Field: "host", Message: msg})
    }

    return fieldErrors
}
```

**预期收益:**
- 减少约 **120 行**重复代码
- 统一验证逻辑，修改一处即可影响所有地方

---

### 3. GormRepository 重复实现 ⚠️ **Medium Priority**

#### 问题描述
在多个包中重复实现了相同名称和结构的 GORM Repository：

**GormJobRepository 重复:**
- `scheduler/gorm_repo.go:13-23` - Scheduler 包
- `worker/executor.go:465-475` - Worker 包

两处的结构体定义和构造函数**完全相同**：
```go
type GormJobRepository struct {
    db *gorm.DB
}

func NewGormJobRepository(db *gorm.DB) (*GormJobRepository, error) {
    if db == nil {
        return nil, fmt.Errorf("xxx: gorm db is nil")
    }
    return &GormJobRepository{db: db}, nil
}
```

**worker/executor.go 中还有类似模式的重复:**
- `GormJobRepository` (465-475)
- `GormDataServerRepository` (501-507)
- `GormTaskRunRepository` (526-532)

#### 重构建议

**方案 A: 统一到 core/repository 包（推荐）**
```bash
# 新建 core/repository.go
```

```go
package core

import (
    "context"
    "fmt"
    "gorm.io/gorm"
)

// JobRepository Job 数据访问接口（被 scheduler 和 worker 共享）
type JobRepository interface {
    GetByID(ctx context.Context, id uint) (Job, error)
    ListEnabledJobs(ctx context.Context) ([]Job, error)
    UpdateStatus(ctx context.Context, id uint, status string) error
}

// GormJobRepository GORM 实现
type GormJobRepository struct {
    db *gorm.DB
}

func NewGormJobRepository(db *gorm.DB) (*GormJobRepository, error) {
    if db == nil {
        return nil, fmt.Errorf("gorm db is nil")
    }
    return &GormJobRepository{db: db}, nil
}

func (r *GormJobRepository) GetByID(ctx context.Context, id uint) (Job, error) {
    // ... 实现
}

func (r *GormJobRepository) ListEnabledJobs(ctx context.Context) ([]Job, error) {
    // ... 实现
}

func (r *GormJobRepository) UpdateStatus(ctx context.Context, id uint, status string) error {
    // ... 实现
}

// 同理添加 DataServerRepository, TaskRunRepository
```

**方案 B: 使用泛型简化 Repository 基类（Go 1.18+）**
```go
// core/base_repository.go
type BaseRepository[T any] struct {
    db *gorm.DB
}

func NewBaseRepository[T any](db *gorm.DB) (*BaseRepository[T], error) {
    if db == nil {
        return nil, fmt.Errorf("gorm db is nil")
    }
    return &BaseRepository[T]{db: db}, nil
}

func (r *BaseRepository[T]) GetByID(ctx context.Context, id uint) (*T, error) {
    var entity T
    if err := r.db.WithContext(ctx).First(&entity, id).Error; err != nil {
        return nil, err
    }
    return &entity, nil
}
```

**预期收益:**
- 消除跨包重复定义
- 统一接口规范
- 减少约 **50-100 行**重复代码

---

### 4. cleanRemotePath 函数重复 ⚠️ **Low Priority**

#### 问题描述
相同的路径清理函数在两处重复实现：

**涉及位置:**
- `filesystem/client.go:266-273`
- `filesystemdriver/adapter.go:445-450`

**重复代码:**
```go
func cleanRemotePath(p string) string {
    if strings.TrimSpace(p) == "" {
        return "/"
    }
    return path.Clean("/" + p)
}
```

#### 重构建议

移动到 `utils/path.go`：

```go
// CleanRemotePath 清理远程路径（统一使用 Unix 路径格式）
//
// 确保路径以 "/" 开头，并移除多余的斜杠和 ".." 等。
func CleanRemotePath(p string) string {
    if strings.TrimSpace(p) == "" {
        return "/"
    }
    return path.Clean("/" + p)
}
```

然后在 `filesystem/client.go` 和 `filesystemdriver/adapter.go` 中导入使用：
```go
import "github.com/strmsync/strmsync/utils"

// ...
cleanPath := utils.CleanRemotePath(remotePath)
```

**预期收益:**
- 减少 **10 行**重复代码
- 统一路径处理逻辑

---

### 5. 客户端初始化模式重复 📝 **Low Priority**

#### 问题描述
`filesystem` 包下的三个客户端（CloudDrive2/OpenList/Local）在初始化时有相似的模式：

**涉及文件:**
- `filesystem/clouddrive2.go`
- `filesystem/openlist.go`
- `filesystem/local.go`

**重复模式:**
- 配置结构（host/port/apiKey/timeout）
- Logger 注入
- HTTP Client 注入
- 选项模式（WithLogger, WithHTTPClient）

#### 重构建议

抽取公共基础结构：

```go
// filesystem/base_client.go
type BaseClient struct {
    config     Config
    logger     *zap.Logger
    httpClient *http.Client
}

func newBaseClient(cfg Config, opts ...ClientOption) (*BaseClient, error) {
    bc := &BaseClient{
        config:     cfg,
        logger:     zap.NewNop(),
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }

    for _, opt := range opts {
        opt(bc)
    }

    return bc, nil
}
```

**预期收益:**
- 减少约 **50 行**重复初始化代码
- 统一选项模式

---

### 6. Job CRUD 与 Server CRUD 流程相似 📝 **Medium Priority**

#### 问题描述
`handler/job.go` 中的 CRUD 操作与 Server Handler 有相同的控制流结构：

**共同模式:**
1. 绑定请求 → 2. 验证参数 → 3. 唯一性检查 → 4. 保存/更新 → 5. 响应

**差异点:**
- Job 有额外的 `validateRelatedServers` 验证
- Job 有 `RunJob/StopJob` 特殊操作

#### 重构建议

考虑引入通用的 CRUD 模板或中间件：

```go
// handler/crud_template.go
type CRUDHandler[ReqT, ModelT any] struct {
    db       *gorm.DB
    logger   *zap.Logger
    validate func(*ReqT) []FieldError
    toModel  func(*ReqT) *ModelT
}

func (h *CRUDHandler[ReqT, ModelT]) HandleCreate(c *gin.Context) {
    var req ReqT
    if err := c.ShouldBindJSON(&req); err != nil {
        respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
        return
    }

    if fieldErrors := h.validate(&req); len(fieldErrors) > 0 {
        respondValidationError(c, fieldErrors)
        return
    }

    model := h.toModel(&req)
    if err := h.db.Create(model).Error; err != nil {
        // ... 统一错误处理
    }

    c.JSON(http.StatusCreated, gin.H{"data": model})
}
```

**预期收益:**
- 进一步减少 Handler 层重复代码
- 统一 HTTP 响应格式

---

## 重构优先级总结

| 优先级 | 项目 | 预估减少代码行数 | 难度 |
|--------|------|------------------|------|
| **High** | Handler CRUD 模式重复 | 1000+ | 中等 |
| **High** | 参数验证逻辑重复 | 120 | 简单 |
| **Medium** | GormRepository 重复 | 50-100 | 简单 |
| **Medium** | Job/Server CRUD 流程相似 | 200+ | 中等 |
| **Low** | cleanRemotePath 重复 | 10 | 简单 |
| **Low** | 客户端初始化模式 | 50 | 简单 |

**建议实施顺序:**
1. 先实施简单的低悬果实（#3 GormRepository, #4 cleanRemotePath, #2 参数验证）
2. 再重构 Handler CRUD (#1)
3. 最后考虑更大范围的模式统一 (#5, #6)

---

## 备注

- 本次扫描基于 2026-02-19 的代码快照
- 所有建议均为**只读分析**，未实际修改代码
- 实施重构前请确保有完整的测试覆盖
- 建议使用渐进式重构，避免一次性大规模改动

---

## 下次扫描

**待扫描领域:**
- [ ] service 包接口与实际实现的对齐情况
- [ ] filesystem 包下三个客户端（CloudDrive2/OpenList/Local）的方法重复
- [ ] worker/executor.go 和 filesystemdriver/adapter.go 的 driver/writer 构建逻辑
- [ ] 测试代码中的 mock 实现重复
