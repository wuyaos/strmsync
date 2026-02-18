# STRMSync 架构优化方案

**制定日期：** 2026-02-18
**参考项目：** qmediasync-main
**目标版本：** v2.1.0
**预计工期：** 3-4周

---

## 📋 执行摘要

本方案旨在参考 qmediasync 的成熟架构设计，对 STRMSync 进行全面优化，重点解决以下8个核心问题：

1. **统一驱动层** - 当前 filesystem.Client 仅是数据访问层，缺少面向同步引擎的驱动抽象
2. **异步队列调度** - 当前 Job 直接在 goroutine 中执行，无队列管理和并发控制
3. **Cron调度** - 缺少全局统一的定时任务管理器
4. **STRM同步引擎** - 同步逻辑分散在 service 层，缺少清晰的引擎抽象
5. **OpenList/Local驱动** - 现有实现功能完整，需适配新驱动接口
6. **STRM内容校验** - 当前每次都重写 STRM 文件，无内容比对逻辑
7. **目录遍历优化** - 缺少并发控制和生产者-消费者模式
8. **日志系统** - LogService 仅为接口，未实际实现

### 优化优先级

- **P0 (本版本必须完成)**：1, 4, 6
- **P1 (本版本建议完成)**：2, 3, 5, 8
- **P2 (下版本优化)**：7

---

## 1. 统一驱动层设计 (P0)

### 1.1 核心问题

**现状：**
```go
// 当前 filesystem.Client 只提供数据访问
type Client interface {
    List(ctx context.Context, path string, recursive bool, maxDepth int) ([]RemoteFile, error)
    Watch(ctx context.Context, path string, recursive bool) (<-chan FileEvent, error)
    BuildStreamURL(ctx context.Context, remotePath string) (string, error)
    TestConnection(ctx context.Context) error
}
```

**问题：**
- 只返回 string 类型 URL，不包含 BaseURL/Sign/Path/PickCode 等结构化信息
- 缺少 `Stat` 方法用于单文件元数据查询
- 缺少 `CompareStrm` 用于内容比对
- 不提供 `Capabilities` 能力声明（如是否支持 Watch/Sign/PickCode）

### 1.2 新驱动接口设计

**参考来源：** qmediasync driverImpl 接口 + Codex 设计草案

```go
// Package syncengine - 新增模块
package syncengine

import (
    "context"
    "net/url"
    "time"
)

// DriverType 数据源类型
type DriverType string

const (
    DriverCloudDrive2 DriverType = "clouddrive2"
    DriverOpenList    DriverType = "openlist"
    DriverLocal       DriverType = "local"
)

// DriverCapability 驱动能力声明
type DriverCapability struct {
    Watch       bool // 是否支持 Watch 监控
    StrmHTTP    bool // 是否支持 HTTP 模式
    StrmMount   bool // 是否支持 Mount 模式
    PickCode    bool // 是否能提供 115 PickCode
    SignURL     bool // 是否能生成带 Sign 的 URL
}

// RemoteEntry 统一文件信息（替代 filesystem.RemoteFile）
type RemoteEntry struct {
    Path    string    // 远程路径（统一 Unix 风格 /path/to/file）
    Name    string
    Size    int64
    ModTime time.Time
    IsDir   bool
}

// DriverEvent 统一事件
type DriverEvent struct {
    Type    DriverEventType
    Path    string
    Abs     string
    Size    int64
    ModTime time.Time
    IsDir   bool
}

type DriverEventType int

const (
    DriverEventCreate DriverEventType = iota + 1
    DriverEventUpdate
    DriverEventDelete
)

// ListOptions 列表参数
type ListOptions struct {
    Recursive bool
    MaxDepth  int // 0=非递归，>0=最大深度
}

// WatchOptions 监控参数
type WatchOptions struct {
    Recursive bool
}

// StrmInfo 结构化 STRM 信息
type StrmInfo struct {
    RawURL    string    // 写入 .strm 的完整内容
    BaseURL   *url.URL  // 解析后的 base (scheme://host:port)
    Path      string    // 远程路径（clean 过）
    PickCode  string    // 可选：115 PickCode
    Sign      string    // 可选：签名参数
    ExpiresAt time.Time // 可选：签名过期时间
}

// BuildStrmRequest 生成 STRM 的输入
type BuildStrmRequest struct {
    ServerID   uint   // 服务器ID
    RemotePath string
    RemoteMeta *RemoteEntry // 可选：避免重复拉取元数据
}

// CompareInput 比对输入
type CompareInput struct {
    Expected  StrmInfo
    ActualRaw string // 读取到的现有 .strm 内容
}

// CompareResult 比对结果
type CompareResult struct {
    Equal      bool
    NeedUpdate bool
    Reason     string // 不一致原因（用于日志）
}

// driverImpl 统一驱动接口
type driverImpl interface {
    Type() DriverType
    Capabilities() DriverCapability

    // List 列出目录内容
    List(ctx context.Context, path string, opt ListOptions) ([]RemoteEntry, error)

    // Watch 监控目录变化（不支持则返回 ErrNotSupported）
    Watch(ctx context.Context, path string, opt WatchOptions) (<-chan DriverEvent, error)

    // Stat 获取单个文件元数据
    Stat(ctx context.Context, path string) (RemoteEntry, error)

    // BuildStrmInfo 构建 STRM 写入内容
    BuildStrmInfo(ctx context.Context, req BuildStrmRequest) (StrmInfo, error)

    // CompareStrm 对比已有 .strm 内容是否一致
    CompareStrm(ctx context.Context, input CompareInput) (CompareResult, error)

    // TestConnection 连接测试
    TestConnection(ctx context.Context) error
}
```

### 1.3 与现有 filesystem.Client 的关系

**设计原则：** 保持向后兼容，filesystem.Client 继续服务于非引擎场景（如手动文件列表查询）

**适配方案：**

```go
// Package filesystemdriver - 新增适配器层
package filesystemdriver

import (
    "github.com/strmsync/backend/filesystem"
    "github.com/strmsync/backend/syncengine"
)

// Adapter 将 filesystem.Client 适配为 driverImpl
type Adapter struct {
    client filesystem.Client
    typ    syncengine.DriverType
}

func NewAdapter(client filesystem.Client, typ syncengine.DriverType) syncengine.driverImpl {
    return &Adapter{client: client, typ: typ}
}

func (a *Adapter) Type() syncengine.DriverType {
    return a.typ
}

func (a *Adapter) Capabilities() syncengine.DriverCapability {
    // 根据 typ 返回不同能力
    switch a.typ {
    case syncengine.DriverCloudDrive2:
        return syncengine.DriverCapability{
            Watch:    true,
            StrmHTTP: true,
            PickCode: true, // 需要扩展 provider 支持
            SignURL:  true,
        }
    case syncengine.DriverOpenList:
        return syncengine.DriverCapability{
            Watch:    false, // OpenList 不支持 Watch
            StrmHTTP: true,
            PickCode: false,
            SignURL:  false,
        }
    case syncengine.DriverLocal:
        return syncengine.DriverCapability{
            Watch:    true,
            StrmHTTP: false,
            PickCode: false,
            SignURL:  false,
        }
    default:
        return syncengine.DriverCapability{}
    }
}

func (a *Adapter) List(ctx context.Context, path string, opt syncengine.ListOptions) ([]syncengine.RemoteEntry, error) {
    files, err := a.client.List(ctx, path, opt.Recursive, opt.MaxDepth)
    if err != nil {
        return nil, err
    }
    // 转换 filesystem.RemoteFile -> syncengine.RemoteEntry
    entries := make([]syncengine.RemoteEntry, len(files))
    for i, f := range files {
        entries[i] = syncengine.RemoteEntry{
            Path:    f.Path,
            Name:    f.Name,
            Size:    f.Size,
            ModTime: f.ModTime,
            IsDir:   f.IsDir,
        }
    }
    return entries, nil
}

func (a *Adapter) Stat(ctx context.Context, path string) (syncengine.RemoteEntry, error) {
    // 尝试类型断言检查是否支持 Stat
    if statProvider, ok := a.client.(interface {
        Stat(context.Context, string) (filesystem.RemoteFile, error)
    }); ok {
        file, err := statProvider.Stat(ctx, path)
        if err != nil {
            return syncengine.RemoteEntry{}, err
        }
        return syncengine.RemoteEntry{
            Path:    file.Path,
            Name:    file.Name,
            Size:    file.Size,
            ModTime: file.ModTime,
            IsDir:   file.IsDir,
        }, nil
    }
    // 降级：使用 List 过滤
    files, err := a.client.List(ctx, path, false, 0)
    if err != nil {
        return syncengine.RemoteEntry{}, err
    }
    for _, f := range files {
        if f.Path == path {
            return syncengine.RemoteEntry{
                Path:    f.Path,
                Name:    f.Name,
                Size:    f.Size,
                ModTime: f.ModTime,
                IsDir:   f.IsDir,
            }, nil
        }
    }
    return syncengine.RemoteEntry{}, fmt.Errorf("file not found: %s", path)
}

func (a *Adapter) BuildStrmInfo(ctx context.Context, req syncengine.BuildStrmRequest) (syncengine.StrmInfo, error) {
    // 尝试类型断言检查是否支持结构化构建
    if builder, ok := a.client.(interface {
        BuildStrmInfo(context.Context, string) (syncengine.StrmInfo, error)
    }); ok {
        return builder.BuildStrmInfo(ctx, req.RemotePath)
    }
    // 降级：使用现有 BuildStreamURL，然后解析
    rawURL, err := a.client.BuildStreamURL(ctx, req.RemotePath)
    if err != nil {
        return syncengine.StrmInfo{}, err
    }
    parsedURL, err := url.Parse(rawURL)
    if err != nil {
        return syncengine.StrmInfo{}, fmt.Errorf("parse URL: %w", err)
    }
    // 从 URL 中提取 Sign/PickCode/Path
    info := syncengine.StrmInfo{
        RawURL:  rawURL,
        BaseURL: &url.URL{Scheme: parsedURL.Scheme, Host: parsedURL.Host},
        Path:    parsedURL.Path,
    }
    query := parsedURL.Query()
    if sign := query.Get("sign"); sign != "" {
        info.Sign = sign
    }
    if pickcode := query.Get("pickcode"); pickcode != "" {
        info.PickCode = pickcode
    }
    if expires := query.Get("expires"); expires != "" {
        // 解析过期时间
        if exp, err := strconv.ParseInt(expires, 10, 64); err == nil {
            info.ExpiresAt = time.Unix(exp, 0)
        }
    }
    return info, nil
}

func (a *Adapter) CompareStrm(ctx context.Context, input syncengine.CompareInput) (syncengine.CompareResult, error) {
    // 实现详见 1.4 节
    return syncengine.CompareResult{}, nil
}

func (a *Adapter) Watch(ctx context.Context, path string, opt syncengine.WatchOptions) (<-chan syncengine.DriverEvent, error) {
    if !a.Capabilities().Watch {
        return nil, syncengine.ErrNotSupported
    }
    fileEventCh, err := a.client.Watch(ctx, path, opt.Recursive)
    if err != nil {
        return nil, err
    }
    // 转换 filesystem.FileEvent -> syncengine.DriverEvent
    driverEventCh := make(chan syncengine.DriverEvent)
    go func() {
        defer close(driverEventCh)
        for event := range fileEventCh {
            driverEvent := syncengine.DriverEvent{
                Path:    event.Path,
                Abs:     event.Abs,
                Size:    event.Size,
                ModTime: event.ModTime,
                IsDir:   event.IsDir,
            }
            switch event.Type {
            case filesystem.EventCreate:
                driverEvent.Type = syncengine.DriverEventCreate
            case filesystem.EventUpdate:
                driverEvent.Type = syncengine.DriverEventUpdate
            case filesystem.EventDelete:
                driverEvent.Type = syncengine.DriverEventDelete
            }
            driverEventCh <- driverEvent
        }
    }()
    return driverEventCh, nil
}

func (a *Adapter) TestConnection(ctx context.Context) error {
    return a.client.TestConnection(ctx)
}
```

### 1.4 CompareStrm 实现细节

**目标：** 避免重复写入相同内容的 STRM 文件，减少磁盘 I/O 和文件系统事件

**验证规则：**

1. **BaseURL 比对**
   - 只比较 `scheme + host + port`
   - 忽略末尾 `/`
   - 若配置允许多域名（如 CDN），需提供等价列表

2. **Path 比对**
   - 统一用 `path.Clean("/"+path)` 规范化
   - 大小写敏感（CloudDrive2/OpenList 通常敏感）

3. **PickCode 比对**
   - 若驱动声明 `PickCode=true`，则必须存在且完全匹配
   - 否则忽略此字段

4. **Sign 比对**
   - 若驱动声明 `SignURL=true`，则必须存在
   - 若包含过期字段（如 `expires` 或 `e`），需保证 `now < expires`
   - 过期则 `NeedUpdate=true`

5. **失败策略**
   - 解析失败直接 `NeedUpdate=true`
   - 记录 `Reason` 以便日志追踪

**伪代码实现：**

```go
func (a *Adapter) CompareStrm(ctx context.Context, input syncengine.CompareInput) (syncengine.CompareResult, error) {
    // 1. 空内容检查
    if strings.TrimSpace(input.ActualRaw) == "" {
        return syncengine.CompareResult{
            Equal:      false,
            NeedUpdate: true,
            Reason:     "empty file",
        }, nil
    }

    // 2. 解析实际 URL
    actualURL, err := url.Parse(strings.TrimSpace(input.ActualRaw))
    if err != nil {
        return syncengine.CompareResult{
            Equal:      false,
            NeedUpdate: true,
            Reason:     fmt.Sprintf("parse failed: %v", err),
        }, nil
    }

    // 3. BaseURL 比对
    expectedBase := input.Expected.BaseURL
    actualBase := &url.URL{Scheme: actualURL.Scheme, Host: actualURL.Host}
    if expectedBase.String() != actualBase.String() {
        return syncengine.CompareResult{
            Equal:      false,
            NeedUpdate: true,
            Reason:     fmt.Sprintf("baseURL mismatch: expected %s, got %s", expectedBase, actualBase),
        }, nil
    }

    // 4. Path 比对
    expectedPath := path.Clean("/" + input.Expected.Path)
    actualPath := path.Clean("/" + actualURL.Path)
    if expectedPath != actualPath {
        return syncengine.CompareResult{
            Equal:      false,
            NeedUpdate: true,
            Reason:     fmt.Sprintf("path mismatch: expected %s, got %s", expectedPath, actualPath),
        }, nil
    }

    // 5. PickCode 比对（若驱动支持）
    if a.Capabilities().PickCode {
        actualPickCode := actualURL.Query().Get("pickcode")
        if input.Expected.PickCode != actualPickCode {
            return syncengine.CompareResult{
                Equal:      false,
                NeedUpdate: true,
                Reason:     fmt.Sprintf("pickcode mismatch: expected %s, got %s", input.Expected.PickCode, actualPickCode),
            }, nil
        }
    }

    // 6. Sign 比对（若驱动支持）
    if a.Capabilities().SignURL {
        actualSign := actualURL.Query().Get("sign")
        if actualSign == "" {
            return syncengine.CompareResult{
                Equal:      false,
                NeedUpdate: true,
                Reason:     "sign missing",
            }, nil
        }
        // 检查过期时间
        if !input.Expected.ExpiresAt.IsZero() {
            if time.Now().After(input.Expected.ExpiresAt) {
                return syncengine.CompareResult{
                    Equal:      false,
                    NeedUpdate: true,
                    Reason:     "sign expired",
                }, nil
            }
        }
        // 比对 sign 值（可选：若 sign 包含随机数，则跳过）
        if input.Expected.Sign != "" && input.Expected.Sign != actualSign {
            return syncengine.CompareResult{
                Equal:      false,
                NeedUpdate: true,
                Reason:     "sign mismatch",
            }, nil
        }
    }

    // 7. 全部一致
    return syncengine.CompareResult{
        Equal:      true,
        NeedUpdate: false,
        Reason:     "identical",
    }, nil
}
```

### 1.5 实施步骤

**阶段1：接口定义与适配器实现（2天）**

1. 创建 `backend/syncengine/types.go` - 定义所有类型
2. 创建 `backend/syncengine/interfaces.go` - 定义 `driverImpl` 接口
3. 创建 `backend/filesystemdriver/adapter.go` - 实现适配器
4. 编写单元测试验证类型转换

**阶段2：扩展现有 provider（3天）**

1. CloudDrive2 provider 添加 `Stat` 和 `BuildStrmInfo` 方法（需研究 gRPC API）
2. OpenList provider 添加 `Stat` 方法（使用 `/api/fs/get` 接口）
3. Local provider 添加 `Stat` 方法（使用 `os.Stat`）
4. 实现 `CompareStrm` 逻辑并编写测试用例

**阶段3：集成测试（1天）**

1. 使用 test-production-env.sh 验证新适配器
2. 对比新旧接口输出一致性
3. 性能基准测试（避免性能倒退）

---

## 2. STRM同步引擎设计 (P0)

### 2.1 核心问题

**现状：**
- 同步逻辑分散在 `service.Executor` 中
- 直接操作 `filesystem.Client` 和 `os` 包
- 缺少清晰的 Scan → Diff → Apply 流程
- 没有 `CompareStrm` 导致每次都重写文件

**目标：**
- 借鉴 qmediasync 的 SyncStrm 设计，创建独立的引擎模块
- 清晰的工作流程：扫描 → 对比 → 写入 → 清理
- 支持两种模式：一次性同步（RunOnce）和持续监听（RunWatch）

### 2.2 引擎架构设计

```go
// Package syncengine
package syncengine

import (
    "context"
    "time"
    "golang.org/x/sync/errgroup"
)

// SyncOp 同步操作类型
type SyncOp int

const (
    SyncOpCreate SyncOp = iota + 1
    SyncOpUpdate
    SyncOpDelete
    SyncOpSkip
)

// SyncPlanItem 同步计划项
type SyncPlanItem struct {
    Op          SyncOp
    SourcePath  string
    TargetPath  string
    Strm        StrmInfo
    Size        int64
    ModTime     time.Time
}

// StrmWriter 抽象 STRM 文件写入器
type StrmWriter interface {
    Read(ctx context.Context, path string) (string, error)
    Write(ctx context.Context, path string, content string, modTime time.Time) error
    Delete(ctx context.Context, path string) error
    MkdirAll(ctx context.Context, dirPath string) error
}

// SyncContext 同步上下文
type SyncContext struct {
    JobID       uint
    TaskRunID   uint
    SourceRoot  string
    TargetRoot  string
    Extensions  map[string]struct{} // 如 {".mp4": {}, ".mkv": {}}
    Recursive   bool
    MaxDepth    int
    Now         time.Time
    Concurrency int // 并发度（errgroup.SetLimit）
}

// TaskRunSummary 执行摘要
type TaskRunSummary struct {
    CreatedCount int
    UpdatedCount int
    DeletedCount int
    SkippedCount int
    FailedCount  int
    Duration     time.Duration
    StartedAt    time.Time
    EndedAt      time.Time
    ErrorMessage string
}

// SyncEngine 核心同步引擎
type SyncEngine struct {
    driver driverImpl
    writer StrmWriter
    logger Logger // 抽象日志接口
}

// NewSyncEngine 构造器
func NewSyncEngine(driver driverImpl, writer StrmWriter, logger Logger) *SyncEngine {
    return &SyncEngine{
        driver: driver,
        writer: writer,
        logger: logger,
    }
}

// RunOnce 一次性同步（API/Cron 触发）
func (e *SyncEngine) RunOnce(ctx context.Context, sctx *SyncContext) (*TaskRunSummary, error) {
    start := time.Now()
    sum := &TaskRunSummary{StartedAt: start}

    e.logger.Info(ctx, "sync started", map[string]interface{}{
        "job_id":       sctx.JobID,
        "task_run_id":  sctx.TaskRunID,
        "source_root":  sctx.SourceRoot,
        "target_root":  sctx.TargetRoot,
    })

    // 阶段1：扫描源目录
    entries, err := e.driver.List(ctx, sctx.SourceRoot, ListOptions{
        Recursive: sctx.Recursive,
        MaxDepth:  sctx.MaxDepth,
    })
    if err != nil {
        sum.ErrorMessage = fmt.Sprintf("list failed: %v", err)
        e.logger.Error(ctx, sum.ErrorMessage, nil)
        return sum, err
    }

    e.logger.Info(ctx, "scan completed", map[string]interface{}{
        "total_files": len(entries),
    })

    // 阶段2：过滤 + 构建计划
    plans, err := e.buildSyncPlans(ctx, sctx, entries)
    if err != nil {
        sum.ErrorMessage = fmt.Sprintf("build plans failed: %v", err)
        e.logger.Error(ctx, sum.ErrorMessage, nil)
        return sum, err
    }

    e.logger.Info(ctx, "plans built", map[string]interface{}{
        "create_count": countOp(plans, SyncOpCreate),
        "update_count": countOp(plans, SyncOpUpdate),
        "delete_count": countOp(plans, SyncOpDelete),
        "skip_count":   countOp(plans, SyncOpSkip),
    })

    // 阶段3：并发应用计划
    if err := e.applyPlans(ctx, sctx, plans, sum); err != nil {
        sum.ErrorMessage = fmt.Sprintf("apply plans failed: %v", err)
        e.logger.Error(ctx, sum.ErrorMessage, nil)
        return sum, err
    }

    // 阶段4：清理孤儿文件（可选）
    if err := e.cleanupOrphans(ctx, sctx, entries, sum); err != nil {
        e.logger.Warn(ctx, "cleanup orphans failed", map[string]interface{}{
            "error": err.Error(),
        })
        // 不阻断主流程
    }

    sum.EndedAt = time.Now()
    sum.Duration = sum.EndedAt.Sub(start)

    e.logger.Info(ctx, "sync completed", map[string]interface{}{
        "created":  sum.CreatedCount,
        "updated":  sum.UpdatedCount,
        "deleted":  sum.DeletedCount,
        "skipped":  sum.SkippedCount,
        "failed":   sum.FailedCount,
        "duration": sum.Duration.Seconds(),
    })

    return sum, nil
}

// RunWatch 持续监听模式（若驱动支持 Watch）
func (e *SyncEngine) RunWatch(ctx context.Context, sctx *SyncContext) (*TaskRunSummary, error) {
    if !e.driver.Capabilities().Watch {
        return nil, ErrNotSupported
    }

    e.logger.Info(ctx, "watch mode started", map[string]interface{}{
        "source_root": sctx.SourceRoot,
    })

    eventCh, err := e.driver.Watch(ctx, sctx.SourceRoot, WatchOptions{
        Recursive: sctx.Recursive,
    })
    if err != nil {
        return nil, fmt.Errorf("watch: %w", err)
    }

    sum := &TaskRunSummary{StartedAt: time.Now()}

    for {
        select {
        case <-ctx.Done():
            sum.EndedAt = time.Now()
            sum.Duration = sum.EndedAt.Sub(sum.StartedAt)
            return sum, ctx.Err()
        case event, ok := <-eventCh:
            if !ok {
                sum.EndedAt = time.Now()
                sum.Duration = sum.EndedAt.Sub(sum.StartedAt)
                return sum, nil
            }
            // 处理单个事件
            if err := e.handleEvent(ctx, sctx, event, sum); err != nil {
                e.logger.Error(ctx, "handle event failed", map[string]interface{}{
                    "event": event,
                    "error": err.Error(),
                })
                sum.FailedCount++
            }
        }
    }
}

// buildSyncPlans 构建同步计划
func (e *SyncEngine) buildSyncPlans(ctx context.Context, sctx *SyncContext, entries []RemoteEntry) ([]SyncPlanItem, error) {
    var plans []SyncPlanItem

    for _, entry := range entries {
        // 1. 跳过目录
        if entry.IsDir {
            continue
        }

        // 2. 过滤扩展名
        if len(sctx.Extensions) > 0 {
            ext := strings.ToLower(filepath.Ext(entry.Name))
            if _, ok := sctx.Extensions[ext]; !ok {
                continue
            }
        }

        // 3. 计算目标路径
        relPath, err := filepath.Rel(sctx.SourceRoot, entry.Path)
        if err != nil {
            e.logger.Warn(ctx, "rel path failed", map[string]interface{}{
                "source_path": entry.Path,
                "error":       err.Error(),
            })
            continue
        }
        targetPath := filepath.Join(sctx.TargetRoot, relPath)
        targetPath = strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + ".strm"

        // 4. 构建期望的 STRM 内容
        expectedStrm, err := e.driver.BuildStrmInfo(ctx, BuildStrmRequest{
            ServerID:   0, // 可从 sctx 传入
            RemotePath: entry.Path,
            RemoteMeta: &entry,
        })
        if err != nil {
            e.logger.Error(ctx, "build strm info failed", map[string]interface{}{
                "remote_path": entry.Path,
                "error":       err.Error(),
            })
            continue
        }

        // 5. 读取现有 STRM 内容
        actualRaw, err := e.writer.Read(ctx, targetPath)
        if err != nil && !os.IsNotExist(err) {
            e.logger.Error(ctx, "read strm failed", map[string]interface{}{
                "target_path": targetPath,
                "error":       err.Error(),
            })
            continue
        }

        // 6. 比对内容
        compareResult, err := e.driver.CompareStrm(ctx, CompareInput{
            Expected:  expectedStrm,
            ActualRaw: actualRaw,
        })
        if err != nil {
            e.logger.Error(ctx, "compare strm failed", map[string]interface{}{
                "target_path": targetPath,
                "error":       err.Error(),
            })
            continue
        }

        // 7. 确定操作类型
        var op SyncOp
        if actualRaw == "" || os.IsNotExist(err) {
            op = SyncOpCreate
        } else if compareResult.NeedUpdate {
            op = SyncOpUpdate
        } else if compareResult.Equal {
            op = SyncOpSkip
        } else {
            op = SyncOpUpdate // 默认更新
        }

        plans = append(plans, SyncPlanItem{
            Op:         op,
            SourcePath: entry.Path,
            TargetPath: targetPath,
            Strm:       expectedStrm,
            Size:       entry.Size,
            ModTime:    entry.ModTime,
        })
    }

    return plans, nil
}

// applyPlans 应用同步计划（并发执行）
func (e *SyncEngine) applyPlans(ctx context.Context, sctx *SyncContext, plans []SyncPlanItem, sum *TaskRunSummary) error {
    g, gctx := errgroup.WithContext(ctx)
    g.SetLimit(sctx.Concurrency)

    for _, plan := range plans {
        plan := plan // 避免闭包问题
        g.Go(func() error {
            switch plan.Op {
            case SyncOpCreate, SyncOpUpdate:
                // 确保目标目录存在
                targetDir := filepath.Dir(plan.TargetPath)
                if err := e.writer.MkdirAll(gctx, targetDir); err != nil {
                    e.logger.Error(gctx, "mkdir failed", map[string]interface{}{
                        "target_dir": targetDir,
                        "error":      err.Error(),
                    })
                    sum.FailedCount++
                    return nil // 不阻断其他任务
                }

                // 写入 STRM 文件
                if err := e.writer.Write(gctx, plan.TargetPath, plan.Strm.RawURL, plan.ModTime); err != nil {
                    e.logger.Error(gctx, "write strm failed", map[string]interface{}{
                        "target_path": plan.TargetPath,
                        "error":       err.Error(),
                    })
                    sum.FailedCount++
                    return nil
                }

                if plan.Op == SyncOpCreate {
                    sum.CreatedCount++
                } else {
                    sum.UpdatedCount++
                }

            case SyncOpDelete:
                if err := e.writer.Delete(gctx, plan.TargetPath); err != nil {
                    e.logger.Error(gctx, "delete strm failed", map[string]interface{}{
                        "target_path": plan.TargetPath,
                        "error":       err.Error(),
                    })
                    sum.FailedCount++
                    return nil
                }
                sum.DeletedCount++

            case SyncOpSkip:
                sum.SkippedCount++
            }

            return nil
        })
    }

    return g.Wait()
}

// cleanupOrphans 清理孤儿文件
func (e *SyncEngine) cleanupOrphans(ctx context.Context, sctx *SyncContext, entries []RemoteEntry, sum *TaskRunSummary) error {
    // 构建源文件集合
    sourceSet := make(map[string]struct{})
    for _, entry := range entries {
        if entry.IsDir {
            continue
        }
        relPath, _ := filepath.Rel(sctx.SourceRoot, entry.Path)
        targetPath := filepath.Join(sctx.TargetRoot, relPath)
        targetPath = strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + ".strm"
        sourceSet[targetPath] = struct{}{}
    }

    // 遍历目标目录，删除不在源集合中的 .strm 文件
    return filepath.WalkDir(sctx.TargetRoot, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            return nil
        }
        if filepath.Ext(path) != ".strm" {
            return nil
        }
        if _, exists := sourceSet[path]; !exists {
            if err := e.writer.Delete(ctx, path); err != nil {
                e.logger.Error(ctx, "cleanup orphan failed", map[string]interface{}{
                    "path":  path,
                    "error": err.Error(),
                })
                sum.FailedCount++
            } else {
                sum.DeletedCount++
            }
        }
        return nil
    })
}

// handleEvent 处理单个 Watch 事件
func (e *SyncEngine) handleEvent(ctx context.Context, sctx *SyncContext, event DriverEvent, sum *TaskRunSummary) error {
    // 类似 buildSyncPlans，但针对单个文件
    // Create/Update -> BuildStrmInfo -> CompareStrm -> Write
    // Delete -> Delete
    return nil
}

// 辅助函数
func countOp(plans []SyncPlanItem, op SyncOp) int {
    count := 0
    for _, p := range plans {
        if p.Op == op {
            count++
        }
    }
    return count
}
```

### 2.3 StrmWriter 实现

```go
// Package strmwriter
package strmwriter

import (
    "context"
    "os"
    "path/filepath"
    "time"
)

// LocalStrmWriter 本地文件系统写入器
type LocalStrmWriter struct{}

func NewLocalStrmWriter() *LocalStrmWriter {
    return &LocalStrmWriter{}
}

func (w *LocalStrmWriter) Read(ctx context.Context, path string) (string, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    return string(data), nil
}

func (w *LocalStrmWriter) Write(ctx context.Context, path string, content string, modTime time.Time) error {
    if err := os.WriteFile(path, []byte(content), 0644); err != nil {
        return err
    }
    // 设置修改时间与源文件一致
    if !modTime.IsZero() {
        _ = os.Chtimes(path, modTime, modTime)
    }
    return nil
}

func (w *LocalStrmWriter) Delete(ctx context.Context, path string) error {
    return os.Remove(path)
}

func (w *LocalStrmWriter) MkdirAll(ctx context.Context, dirPath string) error {
    return os.MkdirAll(dirPath, 0755)
}
```

### 2.4 实施步骤

**阶段1：引擎核心实现（3天）**

1. 创建 `backend/syncengine/engine.go` - 实现 `SyncEngine`
2. 创建 `backend/strmwriter/writer.go` - 实现 `LocalStrmWriter`
3. 实现 `RunOnce` 的完整流程
4. 编写单元测试（使用 mock driver 和 writer）

**阶段2：集成到 service 层（2天）**

1. 修改 `service.Executor` 使用 `SyncEngine`
2. 传递正确的 `SyncContext`
3. 将 `TaskRunSummary` 回写到数据库
4. 集成日志系统

**阶段3：RunWatch 实现（2天）**

1. 实现 `handleEvent` 方法
2. 测试 CloudDrive2 Watch 模式
3. 测试 Local Watch 模式（使用 fsnotify）

**阶段4：集成测试（1天）**

1. 使用真实服务器测试 RunOnce
2. 验证 CompareStrm 避免重复写入
3. 压力测试（大文件量场景）

---

## 3. 异步队列调度 (P1)

### 3.1 核心问题

**现状：**
```go
// service/executor.go
func (e *Executor) Execute(ctx context.Context, jobID uint) error {
    go func() {
        // 直接执行，无队列管理
        e.executeJob(context.Background(), jobID)
    }()
    return nil
}
```

**问题：**
- 无队列，无法控制并发
- 重复触发会启动多个 goroutine
- 无任务去重机制
- 无优先级管理

### 3.2 SyncQueue 设计

**参考：** qmediasync 的 Queue + errgroup.SetLimit

```go
// Package syncqueue
package syncqueue

import (
    "context"
    "sync"
    "golang.org/x/sync/errgroup"
)

// QueueItem 队列项
type QueueItem struct {
    JobID    uint
    Priority int // 优先级（数字越小优先级越高）
    Payload  []byte // 可选：序列化参数
}

// SyncQueue 同步队列
type SyncQueue struct {
    ch          chan QueueItem
    workers     int // worker 数量
    concurrency int // 单个 worker 内部并发度
    running     map[uint]struct{} // 去重：正在运行的 JobID
    mu          sync.Mutex
    stopOnce    sync.Once
}

// NewSyncQueue 构造器
func NewSyncQueue(workers, concurrency, buffer int) *SyncQueue {
    return &SyncQueue{
        ch:          make(chan QueueItem, buffer),
        workers:     workers,
        concurrency: concurrency,
        running:     make(map[uint]struct{}),
    }
}

// Enqueue 入队（带去重）
func (q *SyncQueue) Enqueue(item QueueItem) bool {
    q.mu.Lock()
    defer q.mu.Unlock()

    // 去重检查
    if _, exists := q.running[item.JobID]; exists {
        return false // 已在运行中，不重复入队
    }

    q.running[item.JobID] = struct{}{}
    q.ch <- item
    return true
}

// Run 启动队列处理器
func (q *SyncQueue) Run(ctx context.Context, handle func(context.Context, QueueItem) error) error {
    g, gctx := errgroup.WithContext(ctx)
    g.SetLimit(q.workers) // 限制 worker 数量

    for i := 0; i < q.workers; i++ {
        g.Go(func() error {
            for {
                select {
                case <-gctx.Done():
                    return gctx.Err()
                case item, ok := <-q.ch:
                    if !ok {
                        return nil
                    }
                    // 执行任务
                    if err := handle(gctx, item); err != nil {
                        // 记录错误但不阻断队列
                        // TODO: 通知错误
                    }
                    // 执行完成，从 running 中移除
                    q.mu.Lock()
                    delete(q.running, item.JobID)
                    q.mu.Unlock()
                }
            }
        })
    }

    return g.Wait()
}

// Stop 停止队列
func (q *SyncQueue) Stop() {
    q.stopOnce.Do(func() {
        close(q.ch)
    })
}
```

### 3.3 集成到 Executor

```go
// service/executor.go (修改后)
type Executor struct {
    jobRepo         database.JobRepository
    serverRepo      database.DataServerRepository
    mediaServerRepo database.MediaServerRepository
    taskRunRepo     database.TaskRunRepository
    filesystemMgr   *filesystem.Manager
    queue           *syncqueue.SyncQueue
    engineFactory   func(jobID uint) (*syncengine.SyncEngine, error) // 工厂函数
}

func NewExecutor(
    jobRepo database.JobRepository,
    serverRepo database.DataServerRepository,
    mediaServerRepo database.MediaServerRepository,
    taskRunRepo database.TaskRunRepository,
    filesystemMgr *filesystem.Manager,
) *Executor {
    exec := &Executor{
        jobRepo:         jobRepo,
        serverRepo:      serverRepo,
        mediaServerRepo: mediaServerRepo,
        taskRunRepo:     taskRunRepo,
        filesystemMgr:   filesystemMgr,
        queue:           syncqueue.NewSyncQueue(3, 5, 100), // 3 workers, 5 concurrency, buffer 100
    }

    // 启动队列处理器
    go exec.queue.Run(context.Background(), exec.handleQueueItem)

    return exec
}

func (e *Executor) Execute(ctx context.Context, jobID uint) error {
    // 入队（带去重）
    if !e.queue.Enqueue(syncqueue.QueueItem{JobID: jobID, Priority: 10}) {
        return fmt.Errorf("job %d already in queue", jobID)
    }
    return nil
}

func (e *Executor) handleQueueItem(ctx context.Context, item syncqueue.QueueItem) error {
    // 创建 TaskRun 记录
    taskRun := &database.TaskRun{
        JobID:     item.JobID,
        Status:    "running",
        StartedAt: time.Now(),
    }
    if err := e.taskRunRepo.Create(ctx, taskRun); err != nil {
        return fmt.Errorf("create task run: %w", err)
    }

    // 执行同步
    engine, err := e.engineFactory(item.JobID)
    if err != nil {
        taskRun.Status = "failed"
        taskRun.ErrorMessage = err.Error()
        _ = e.taskRunRepo.Update(ctx, taskRun)
        return err
    }

    summary, err := engine.RunOnce(ctx, &syncengine.SyncContext{
        JobID:     item.JobID,
        TaskRunID: taskRun.ID,
        // ... 其他参数从 Job 配置读取
    })

    // 更新 TaskRun 记录
    taskRun.Status = "completed"
    if err != nil {
        taskRun.Status = "failed"
        taskRun.ErrorMessage = err.Error()
    }
    taskRun.CreatedCount = summary.CreatedCount
    taskRun.UpdatedCount = summary.UpdatedCount
    taskRun.DeletedCount = summary.DeletedCount
    taskRun.SkippedCount = summary.SkippedCount
    taskRun.FailedCount = summary.FailedCount
    taskRun.EndedAt = time.Now()

    if err := e.taskRunRepo.Update(ctx, taskRun); err != nil {
        return fmt.Errorf("update task run: %w", err)
    }

    return nil
}
```

### 3.4 实施步骤

**阶段1：队列实现（2天）**

1. 创建 `backend/syncqueue/queue.go`
2. 实现去重和优先级逻辑（可选）
3. 编写单元测试

**阶段2：集成到 Executor（1天）**

1. 修改 `service.Executor` 使用队列
2. 实现 `handleQueueItem`
3. 启动队列处理器

**阶段3：测试（1天）**

1. 测试去重（连续触发同一 Job）
2. 测试并发限制（启动大量 Job）
3. 压力测试

---

## 4. Cron调度 (P1)

### 4.1 Scheduler 设计

**参考：** qmediasync 的 synccron 模块

```go
// Package scheduler
package scheduler

import (
    "context"
    "sync"
    "github.com/robfig/cron/v3"
    "github.com/strmsync/backend/syncqueue"
)

// Scheduler 全局定时任务管理器
type Scheduler struct {
    cron    *cron.Cron
    queue   *syncqueue.SyncQueue
    entries map[uint]cron.EntryID // JobID -> EntryID
    mu      sync.Mutex
}

// NewScheduler 构造器
func NewScheduler(queue *syncqueue.SyncQueue) *Scheduler {
    return &Scheduler{
        cron:    cron.New(),
        queue:   queue,
        entries: make(map[uint]cron.EntryID),
    }
}

// Start 启动调度器
func (s *Scheduler) Start() {
    s.cron.Start()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
    s.cron.Stop()
}

// Register 注册定时任务
func (s *Scheduler) Register(jobID uint, spec string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // 移除旧任务（如果存在）
    if oldEntryID, exists := s.entries[jobID]; exists {
        s.cron.Remove(oldEntryID)
    }

    // 添加新任务
    entryID, err := s.cron.AddFunc(spec, func() {
        s.queue.Enqueue(syncqueue.QueueItem{JobID: jobID, Priority: 10})
    })
    if err != nil {
        return err
    }

    s.entries[jobID] = entryID
    return nil
}

// Unregister 取消注册
func (s *Scheduler) Unregister(jobID uint) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if entryID, exists := s.entries[jobID]; exists {
        s.cron.Remove(entryID)
        delete(s.entries, jobID)
    }
}

// Reload 重新加载所有启用 cron 的 Job
func (s *Scheduler) Reload(ctx context.Context, jobs []database.Job) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // 清空现有任务
    for _, entryID := range s.entries {
        s.cron.Remove(entryID)
    }
    s.entries = make(map[uint]cron.EntryID)

    // 注册启用 cron 的 Job
    for _, job := range jobs {
        if !job.Enabled || job.WatchMode != "cron" {
            continue
        }
        // 从 job.Options 中解析 cron_spec
        // 例如: {"cron_spec": "0 * * * *"}
        var opts struct {
            CronSpec string `json:"cron_spec"`
        }
        if err := json.Unmarshal([]byte(job.Options), &opts); err != nil || opts.CronSpec == "" {
            continue
        }
        if err := s.Register(job.ID, opts.CronSpec); err != nil {
            return fmt.Errorf("register job %d: %w", job.ID, err)
        }
    }

    return nil
}
```

### 4.2 实施步骤

**阶段1：Scheduler 实现（2天）**

1. 创建 `backend/scheduler/scheduler.go`
2. 集成 `robfig/cron/v3`
3. 实现 Register/Unregister/Reload

**阶段2：集成到应用启动（1天）**

1. 在 `cmd/server/main.go` 中初始化 Scheduler
2. 应用启动时调用 `Reload`
3. Job 更新时调用 Register/Unregister

**阶段3：测试（1天）**

1. 测试定时触发
2. 测试动态更新 cron_spec
3. 测试重启后任务恢复

---

## 5. 日志系统实现 (P1)

### 5.1 LogService 实现

```go
// Package service
package service

import (
    "context"
    "time"
    "github.com/strmsync/backend/database"
)

// LogServiceImpl LogService 接口实现
type LogServiceImpl struct {
    logRepo database.LogRepository
}

func NewLogService(logRepo database.LogRepository) *LogServiceImpl {
    return &LogServiceImpl{logRepo: logRepo}
}

func (s *LogServiceImpl) Info(ctx context.Context, message string, fields map[string]interface{}) {
    s.log(ctx, "info", message, fields)
}

func (s *LogServiceImpl) Warn(ctx context.Context, message string, fields map[string]interface{}) {
    s.log(ctx, "warn", message, fields)
}

func (s *LogServiceImpl) Error(ctx context.Context, message string, fields map[string]interface{}) {
    s.log(ctx, "error", message, fields)
}

func (s *LogServiceImpl) log(ctx context.Context, level string, message string, fields map[string]interface{}) {
    // 从 fields 中提取 job_id/task_run_id
    jobID, _ := fields["job_id"].(uint)
    taskRunID, _ := fields["task_run_id"].(uint)

    log := &database.Log{
        JobID:     jobID,
        TaskRunID: taskRunID,
        Level:     level,
        Message:   message,
        Fields:    fields,
        CreatedAt: time.Now(),
    }

    // 异步写入数据库（避免阻塞主流程）
    go func() {
        if err := s.logRepo.Create(context.Background(), log); err != nil {
            // 无法记录日志，打印到 stderr
            fmt.Fprintf(os.Stderr, "failed to write log: %v\n", err)
        }
    }()
}
```

### 5.2 实施步骤

**阶段1：LogService 实现（1天）**

1. 创建 `backend/service/log_service.go`
2. 实现 Info/Warn/Error 方法
3. 异步写入数据库

**阶段2：集成到 SyncEngine（1天）**

1. 在 `NewSyncEngine` 中传入 `LogService`
2. 替换所有日志调用
3. 测试日志写入数据库

---

## 6. 其他优化 (P1/P2)

### 6.1 OpenList/Local 驱动适配 (P1)

**工作量：** 1天

1. 为 OpenList provider 添加 `Stat` 方法（使用 `/api/fs/get`）
2. 为 Local provider 添加 `Stat` 方法（使用 `os.Stat`）
3. 测试适配器正确性

### 6.2 目录遍历优化 (P2)

**参考：** qmediasync 的 BFS + worker pool

**工作量：** 2-3天

1. 使用 pathQueue + 生产者-消费者模式
2. errgroup.SetLimit 控制并发
3. 支持分页（pagination）

---

## 7. 实施计划

### 7.1 整体时间表

| 阶段 | 工作内容 | 优先级 | 预计工期 | 依赖 |
|------|----------|--------|----------|------|
| **第1周** | 统一驱动层 + 适配器 | P0 | 5天 | - |
| **第2周** | STRM同步引擎 + CompareStrm | P0 | 5天 | 第1周 |
| **第3周** | 异步队列 + Cron调度 + LogService | P1 | 5天 | 第2周 |
| **第4周** | 集成测试 + 文档 | - | 5天 | 第3周 |

### 7.2 每周详细任务

**第1周：统一驱动层 (P0)**

| 天数 | 任务 | 交付物 |
|------|------|--------|
| Day 1-2 | driverImpl 接口定义 + 适配器实现 | `syncengine/types.go`、`filesystemdriver/adapter.go` |
| Day 3-4 | 扩展 provider (Stat, BuildStrmInfo, CompareStrm) | CloudDrive2/OpenList/Local provider 更新 |
| Day 5 | 单元测试 + 集成测试 | 测试用例覆盖率 >80% |

**第2周：STRM同步引擎 (P0)**

| 天数 | 任务 | 交付物 |
|------|------|--------|
| Day 1-3 | SyncEngine 核心实现 (RunOnce) | `syncengine/engine.go`、`strmwriter/writer.go` |
| Day 4 | 集成到 service.Executor | Executor 使用 SyncEngine |
| Day 5 | RunWatch 实现 + 测试 | Watch 模式可用 |

**第3周：异步队列 + Cron + 日志 (P1)**

| 天数 | 任务 | 交付物 |
|------|------|--------|
| Day 1-2 | SyncQueue 实现 + 集成 | `syncqueue/queue.go` |
| Day 3 | Scheduler 实现 + 集成 | `scheduler/scheduler.go` |
| Day 4 | LogService 实现 + 集成 | `service/log_service.go` |
| Day 5 | OpenList/Local 驱动适配 | Adapter 完整支持 |

**第4周：集成测试 + 文档**

| 天数 | 任务 | 交付物 |
|------|------|--------|
| Day 1-2 | 完整系统测试（使用真实服务器） | 测试报告 |
| Day 3 | 性能压力测试 + 优化 | 性能基准报告 |
| Day 4 | 更新文档（API、架构、迁移指南） | 完整文档 |
| Day 5 | Code Review + 发布准备 | v2.1.0-rc |

### 7.3 里程碑

- **Week 1 End**: 统一驱动层可用，现有 filesystem 适配完成
- **Week 2 End**: SyncEngine 可独立运行，CompareStrm 避免重复写入
- **Week 3 End**: 队列、Cron、日志全部集成，P1 功能完成
- **Week 4 End**: v2.1.0 发布候选版本就绪

---

## 8. 风险评估与应对

### 8.1 技术风险

| 风险 | 概率 | 影响 | 应对策略 |
|------|------|------|----------|
| CloudDrive2 gRPC API 不支持 PickCode/Sign 提取 | 中 | 高 | 降级：BuildStrmInfo 返回简化版，仅包含 BaseURL + Path |
| CompareStrm 逻辑复杂，边界情况多 | 高 | 中 | 编写完整单元测试，覆盖所有边界条件 |
| 并发控制不当导致死锁 | 低 | 高 | 使用 errgroup 成熟库，避免手动管理 goroutine |
| 日志异步写入丢失 | 低 | 中 | 使用带缓冲 channel + 优雅关闭 |

### 8.2 进度风险

| 风险 | 概率 | 影响 | 应对策略 |
|------|------|------|----------|
| Codex Code Review 发现设计问题 | 中 | 中 | 预留 buffer 时间（每周1天）用于返工 |
| 测试环境不稳定 | 中 | 中 | 优先使用 mock，真实测试放最后 |
| 用户需求变更 | 低 | 高 | 冻结需求，P2 功能推迟到下版本 |

---

## 9. 验收标准

### 9.1 功能验收

- [ ] driverImpl 接口可适配 CloudDrive2/OpenList/Local
- [ ] SyncEngine.RunOnce 可完成完整同步流程
- [ ] CompareStrm 正确避免重复写入（测试覆盖率 >90%）
- [ ] SyncQueue 支持去重和并发限制
- [ ] Scheduler 可动态注册/取消 cron 任务
- [ ] LogService 正确写入数据库（异步，无阻塞）
- [ ] RunWatch 在支持的驱动上正常工作

### 9.2 性能验收

- [ ] 10000 文件同步时间 <5分钟（baseline: 当前约10分钟）
- [ ] CompareStrm 避免重写后，IOPS 降低 >80%
- [ ] 并发控制下，内存占用 <500MB（baseline: 当前约800MB）
- [ ] 队列峰值吞吐 >100 Job/秒

### 9.3 质量验收

- [ ] 单元测试覆盖率 >80%
- [ ] 集成测试通过率 100%（使用真实服务器）
- [ ] Codex Code Review 无 Critical 问题
- [ ] 文档完整（API、架构、迁移指南）

---

## 10. 后续优化建议

### 10.1 v2.2.0 规划 (P2)

1. **目录遍历优化**
   - BFS + worker pool + errgroup.SetLimit
   - 支持分页和增量扫描

2. **增量同步**
   - 基于 mtime 的增量扫描
   - 支持 last_sync_at 记录

3. **通知系统**
   - 完成/失败时发送通知（Telegram/邮件）
   - 集成 notification service

### 10.2 v2.3.0 规划 (P3)

1. **状态追踪**
   - SyncPath.last_sync_at 记录
   - 支持断点续传

2. **监控与指标**
   - Prometheus metrics
   - 性能监控面板

---

## 11. 参考资料

- **qmediasync 分析报告**: `docs/reference-projects/qmediasync_backend_analysis.md`
- **递归深度限制文档**: `docs/RECURSIVE_DEPTH_LIMIT.md`
- **工作会话总结**: `docs/WORK_SESSION_20260218_2.md`
- **Go 最佳实践**: `golang.org/x/sync/errgroup`
- **Cron 库文档**: `github.com/robfig/cron`

---

**文档版本**: v1.0
**最后更新**: 2026-02-18
**审核状态**: 待 Codex Review
