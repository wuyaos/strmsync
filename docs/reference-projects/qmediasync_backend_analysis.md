# QMediaSync 项目完整架构分析报告

> 基于 qmediasync-main 项目的深度代码分析

**分析日期：** 2026-02-18
**项目类型：** Go 多源网盘 STRM 同步系统

---

## 📋 执行摘要

QMediaSync 是一个专业的多源网盘 STRM 同步系统，支持 **115网盘、OpenList、本地目录、百度网盘**四大数据源。核心设计特点包括：

- **统一驱动层**：通过 `driverImpl` 接口适配不同网盘，实现快速接入新数据源
- **异步队列调度**：使用 Cron + 优先级队列实现灵活的任务调度与执行
- **状态机管理**：Sync/SyncPath 两层模型清晰分离任务与配置
- **并发限流**：通过 `errgroup.SetLimit` 和无界队列防止 API 限流与内存溢出
- **增量同步**：支持基于 mtime 的增量扫描，提升大目录处理效率
- **立即通知**：完成/失败时刻发送通知，支持 Telegram、邮件等多渠道

---

## 1. 项目架构概览

### 1.1 分层结构

```
┌─────────────────────────────────────┐
│    表现层 (Presentation)            │
│  Gin Routes + API Controllers       │
│  /sync/start, /sync/records ...     │
└────────────────┬────────────────────┘
                 │
┌─────────────────▼────────────────────┐
│   应用层 (Application)               │
│  synccron (调度) + Queue (执行队列)  │
│  - GlobalCron 定时触发               │
│  - AddNewSyncTask 加入队列           │
└────────────────┬────────────────────┘
                 │
┌─────────────────▼────────────────────┐
│   业务层 (Domain)                    │
│  SyncStrm (同步引擎)                 │
│  - 驱动选择与管理                    │
│  - STRM/Meta 生成与对比              │
│  - 本地差异处理                      │
└────────────────┬────────────────────┘
                 │
┌─────────────────▼────────────────────┐
│   数据层 (Infrastructure)            │
│  models (Sync/SyncPath 实体)        │
│  db (GORM ORM + SQLite/Postgres)    │
└─────────────────────────────────────┘
```

**关键文件位置：**
- 入口：`main.go` (App.Start)
- 表现：`internal/controllers/sync.go` (StartSync)
- 调度：`internal/synccron/synccron.go` (InitCron)
- 业务：`internal/syncstrm/sync.go` (SyncStrm.Start)
- 数据：`internal/models/sync.go` (Sync struct)

### 1.2 核心同步流程

```
API /sync/start
    │
    ├─> StartSyncCron()
    │   ├─> GetSyncPathList(启用cron的路径)
    │   └─> AddNewSyncTask(SyncPath.ID)
    │
    ├─> Queue Manager
    │   └─> ProcessSyncTask
    │
    ├─> NewSyncStrm(Account, SyncPath, Config)
    │   ├─> SelectDriver (115/OpenList/Local/BaiduPan)
    │   └─> Initialize memSyncCache
    │
    └─> SyncStrm.Start()
        ├─> GetNetFileFiles (扫描网盘)
        ├─> Start115PathDispatcher (路径补全-115特有)
        ├─> ProcessStrmFile (生成STRM)
        ├─> ProcessMetaFile (元数据同步)
        ├─> ProcessLocalFile (本地差异)
        ├─> DeleteLocalFile (清理删除)
        ├─> UpdateDatabase (差异回写)
        ├─> Sync.Complete() (标记完成)
        └─> TriggerEmbyRefresh() (延迟触发Emby刷新)
```

---

## 2. 核心模块详解

### 2.1 synccron 调度模块

**职责：** 全局定时任务的统一入口，包括同步、刮削、Token刷新、日志清理等。

**关键组件：**

| 函数 | 职责 |
|------|------|
| `InitCron()` | 初始化 GlobalCron，注册所有定时任务 |
| `StartSyncCron()` | 触发同步任务，将SyncPath加入队列 |
| `startScrapeCron()` | 触发刮削任务 |
| `RefreshOAuthAccessToken()` | 刷新115/百度网盘Token |

**定时任务配置：**
```go
GlobalCron.AddFunc("0 1 * * *", ...)          // 每天凌晨1点清理过期任务
GlobalCron.AddFunc(SettingsGlobal.Cron, ...)  // 用户配置的同步时间
GlobalCron.AddFunc("*/5 * * * *", ...)        // 每5分钟刷新Token
GlobalCron.AddFunc("*/13 * * * *", ...)       // 每13分钟刷新刮削任务
```

### 2.2 syncstrm 同步引擎

**职责：** 同步的核心业务逻辑，统一处理不同网盘的文件扫描、STRM生成、差异比对。

**SyncStrm 结构核心字段：**
```go
type SyncStrm struct {
    SyncDriver   driverImpl            // 驱动实现(115/OpenList/Local/Baidu)
    Account      *models.Account       // 网盘账号
    Sync         *models.Sync          // 同步记录
    SourcePath   string                // 源路径
    TargetPath   string                // 目标路径
    Config       SyncStrmConfig        // 同步配置
    memSyncCache *MemorySyncCache      // 内存缓存(临时表)
    PathWorkerMax int64                // 并发限制
}
```

**关键流程（按执行顺序）：**
1. GetNetFileFiles() - 扫描网盘文件列表
2. PathDispatcher() - 补全路径(115特有)
3. ProcessStrmFile() - 生成STRM文件
4. ProcessMetaFile() - 处理元数据
5. ProcessLocalFile() - 处理本地文件
6. DeleteLocalFile() - 清理删除的文件
7. SyncToDatabase() - 差异更新到数据库

### 2.3 驱动层（driverImpl）

**设计模式：** 策略模式 + 工厂模式

**统一接口：**
```go
type driverImpl interface {
    GetNetFileFiles(ctx context.Context, parentPath, parentPathId string) ([]*SyncFileCache, error)
    GetPathIdByPath(ctx context.Context, path string) (string, error)
    MakeStrmContent(sf *SyncFileCache) string
    CreateDirRecursively(ctx context.Context, parentDir string) (pathId, remotePath string, err error)
    GetDirsByPathId(ctx context.Context, pathId string) ([]pathQueueItem, error)
    DeleteFile(ctx context.Context, parentId string, fileIds []string) error
}
```

**驱动选择：**
```go
switch account.SourceType {
case models.SourceType115:
    syncDriver = NewOpen115Driver(account.Get115Client())
case models.SourceTypeOpenList:
    syncDriver = NewOpenListDriver(account.GetOpenListClient())
case models.SourceTypeLocal:
    syncDriver = NewLocalDriver()
case models.SourceTypeBaiduPan:
    syncDriver = NewBaiduPanDriver(account.GetBaiDuPanClient())
}
```

---

## 3. 文件系统集成分析

### 3.1 驱动功能对比表

| 特性 | 115网盘 | OpenList | 本地目录 | 百度网盘 |
|------|---------|----------|--------|---------|
| **列目录方式** | OpenAPI分页 | HTTP分页 | os.ReadDir | 云同步API分页 |
| **路径ID标识** | FileId + PickCode | 路径字符串 | 本地路径 | FsId |
| **STRM生成方式** | `/115/url/video{ext}` | 签名URL | 文件自身路径 | `/baidupan/url/video{ext}` |
| **目录创建** | MkDir (递归+缓存) | Mkdir逐级 | os.MkdirAll | Mkdir |
| **文件删除** | Del(fileIds) | Del(names) | os.Remove | Del |
| **增量同步支持** | ❌ | ❌ | ❌ | ✅ (按mtime) |
| **路径补全** | ✅ (特殊逻辑) | ❌ | ❌ | ❌ |
| **访问频率限制** | 有重试机制 | 有限速 | 无 | 有限制 |

### 3.2 驱动实现详解

#### 115网盘驱动

**特点：**
- 预加载两层目录以减少API调用
- 路径补全机制处理FileId→Path映射
- 访问频率过高时暂停30秒重试

**STRM生成示例：**
```go
// /115/url/video.mp4?pickcode=xxxxx&userid=12345
u.Path = fmt.Sprintf("/115/url/video%s", ext)
params.Add("pickcode", sf.PickCode)
params.Add("userid", s.Account.UserId)
```

#### OpenList驱动

**特点：**
- 简单直接，路径即ID
- 签名验证机制
- RFC3339时间格式解析

**STRM生成方式：**
```go
// http://openlist.host/sign=xxx
helpers.MakeOpenListUrl(account.BaseUrl, sf.OpenlistSign, sf.GetFileId())
```

#### 本地驱动

**特点：**
- 无需API调用，直接OS操作
- 文件修改时间即mtime
- 支持符号链接

#### 百度网盘驱动

**特点：**
- 支持增量同步（按mtime），提升效率
- 二次元接口获取文件详情
- 与115类似的URL生成

**增量同步逻辑：**
```go
// 只获取 mtime > lastSyncAt 的文件
files, err := d.client.GetFilesByPathMtime(ctx, parentId, lastSyncAt)
```

---

## 4. STRM文件生成机制

### 4.1 生成流程

```
网盘文件 sf
    │
    ├─> CompareStrm(sf)
    │   ├─> 文件不存在? → 需要生成(0)
    │   ├─> 本地源? → 跳过(1)
    │   ├─> 加载现有STRM → 解析URL参数
    │   └─> 比对参数
    │       ├─> BaseUrl ≠ → 重新生成(0)
    │       ├─> PickCode = "" → 补全(0)
    │       ├─> UserId ≠ → 重新生成(0)
    │       └─> 都一致 → 跳过(1)
    │
    ├─> (如果需要生成)
    │   ├─> Driver.MakeStrmContent(sf)
    │   ├─> WriteFileWithPerm()
    │   └─> Chtimes() 设置修改时间
    │
    └─> atomic.AddInt64(&s.NewStrm, 1)
```

### 4.2 核心代码示例

```go
// ProcessStrmFile: 生成STRM文件
func (s *SyncStrm) ProcessStrmFile(sf *SyncFileCache) error {
    // 1. 比对是否需要生成
    rs := s.CompareStrm(sf)
    if rs == 1 {
        return nil  // 无需更新，跳过
    }

    // 2. 由驱动生成STRM内容
    strmContent := s.SyncDriver.MakeStrmContent(sf)
    strmFullPath := sf.GetLocalFilePath(s.TargetPath, s.SourcePath)

    // 3. 写入文件
    err := helpers.WriteFileWithPerm(strmFullPath, []byte(strmContent), 0777)
    if err != nil {
        return err
    }

    // 4. 同步修改时间
    if sf.MTime > 0 {
        os.Chtimes(strmFullPath, time.Unix(sf.MTime, 0), time.Unix(sf.MTime, 0))
    }

    atomic.AddInt64(&s.NewStrm, 1)
    return nil
}
```

### 4.3 STRM内容校验逻辑

比对时检查的关键参数：

| 参数 | 检查内容 | 不一致后果 |
|------|---------|----------|
| BaseUrl | 媒体服务器地址 | 重新生成 |
| PickCode | 115/百度文件标识 | 补全或重新生成 |
| UserId | 账号ID | 重新生成 |
| Sign | OpenList签名 | 重新生成 |
| Path | 文件路径(可选) | 依据配置重新生成 |

---

## 5. 并发和队列管理

### 5.1 任务队列架构

```
Cron Trigger
    │
    ├─> AddNewSyncTask(SyncPath.ID)
    │   └─> Queue.AddTask(task)
    │
    └─> Queue Manager (NewSyncQueuePerType)
        └─> ProcessTask
            ├─> 检查重复(waitingQueue)
            ├─> currentTask = task
            └─> Execute (SyncStrm.Start)
```

**关键队列设计：**
```go
type NewSyncQueuePerType struct {
    sourceType     models.SourceType
    taskChan       chan *NewSyncTask       // 任务通道(缓冲50)
    waitingQueue   map[string]*NewSyncTask // 待处理队列
    currentTask    *NewSyncTask            // 当前任务
    status         string                  // running/paused/stopped
    mutex          sync.RWMutex            // 并发控制
}
```

### 5.2 并发限流机制

#### 115文件处理（errgroup + SetLimit）

```go
eg, ctx := errgroup.WithContext(s.Context)
eg.SetLimit(int(s.PathWorkerMax))  // 限制并发数(默认20)

for _, file := range files {
    currentFile := file
    eg.Go(func() error {
        return s.processNetFile(currentFile)
    })
}
```

#### 目录遍历（无界队列 + sync.Cond）

为防止递归导致内存溢出和死锁，采用自定义队列机制：

```go
type pathQueue struct {
    items   []*pathQueueItem
    cond    *sync.Cond
    pending int64  // 待处理计数
}

// 生产者
for range dirs {
    q.enqueue(dir)
}

// 消费者(受SetLimit控制)
for {
    item := q.dequeue()
    // 处理item
}
```

---

## 6. 数据模型设计

### 6.1 Sync（同步任务）状态机

**状态流转：**
```
SyncStatusPending
    │
    ├─> SyncStatusInProgress
    │   ├─> SyncStatusCompleted (正常完成)
    │   └─> SyncStatusFailed (失败)
```

**关键字段：**

| 字段 | 含义 | 更新时机 |
|------|------|---------|
| `Status` | 任务状态 | 状态转换时 |
| `SubStatus` | 子状态(网盘扫描/本地扫描) | 阶段变化时 |
| `FileOffset` | 文件偏移(恢复用) | 处理中 |
| `NewStrm/NewMeta/NewUpload` | 新增统计 | 处理完文件时 |
| `NetFileStartAt/FinishAt` | 网盘扫描耗时 | 阶段完成时 |
| `LocalFileStartAt/FinishAt` | 本地扫描耗时 | 阶段完成时 |
| `FailReason` | 失败原因 | 失败时 |

### 6.2 SyncPath（同步路径配置）

**两层配置设计：**

1. **全局配置**：默认扩展名、排除规则、元数据策略
2. **路径级覆盖**：可对该路径独立设置，`-1` 表示使用全局配置

**配置示例：**
```go
type SyncPathSetting struct {
    MinVideoSize    int64    // 最小视频大小
    VideoExt        string   // 视频扩展名(JSON)
    ExcludeName     string   // 排除的文件名(JSON)
    UploadMeta      int      // 上传元数据(-1=全局,0=保留,1=上传,2=删除)
    DownloadMeta    int      // 下载元数据(-1=全局,0=不下载,1=下载)
    DeleteDir       int      // 删除目录(-1=全局,0=不删除,1=删除)
}
```

**获取有效值：**
```go
func (sp *SyncPath) GetUploadMeta() int {
    if sp.UploadMeta == -1 {
        return SettingsGlobal.UploadMeta  // 使用全局配置
    }
    return sp.UploadMeta
}
```

---

## 7. 最佳实践与借鉴点

### 7.1 代码设计原则

#### 1) 驱动抽象隔离外部依赖

**优点：**
- 新增网盘来源只需实现 `driverImpl` 接口
- 同步核心逻辑与具体网盘解耦
- 便于测试与维护

**示例：**
```go
// 同步引擎无需关心具体网盘
files, err := s.SyncDriver.GetNetFileFiles(ctx, parentPath, parentPathId)
strmUrl := s.SyncDriver.MakeStrmContent(sf)
```

#### 2) 两层配置提升可维护性

**优点：**
- 全局策略统一管理
- 路径级覆盖支持特例处理
- 减少重复配置

#### 3) 并发限流避免API限制

**解决方案：**
- 115：访问频率过高时暂停30秒重试
- errgroup.SetLimit：控制并发数量
- 无界队列：防止内存溢出

#### 4) 增量同步减少扫描压力

**原理：** 对于支持mtime的网盘，只扫描上次同步后更新的文件。

**优势：**
- 大目录从O(n)降至O(∆n)
- 减少API调用和内存占用
- 缩短同步耗时

#### 5) 预取 + 路径补全优化性能

**115网盘的特殊处理：**
```
预取两层目录 → 缓存目录结构 → 批量路径补全 → 并发文件处理
```

### 7.2 性能优化亮点

| 优化项 | 实现方式 | 提升 |
|-------|--------|------|
| **内存缓存** | memSyncCache 临时表 | 减少DB查询 |
| **批量写回** | 内存汇总后一次更新 | 降低写放大 |
| **增量同步** | mtime增量扫描 | O(∆n)时间复杂度 |
| **路径补全** | 两阶段处理 | 减少重复查询 |
| **并发限流** | errgroup.SetLimit | 避免频率限制 |

### 7.3 可维护性设计

#### 统一通知模块

同步完成/失败即时推送到Telegram、邮件等多渠道。

```go
// 完成通知
notif := &Notification{
    Type:    SyncFinished,
    Title:   "✅ 同步完成",
    Content: "生成STRM: 100, 下载: 50, 上传: 30",
}
notificationmanager.Send(ctx, notif)
```

#### 集中日志管理

每次同步创建独立日志文件，便于问题追踪。

---

## 8. 对STRMSync项目的建议

### 8.1 短期建议（立即可实施）

#### 1. 引入驱动层抽象

```go
type FileDriver interface {
    ListFiles(ctx context.Context, path string) ([]File, error)
    GetFileDetail(ctx context.Context, fileId string) (*FileDetail, error)
    CreateDir(ctx context.Context, path string) error
    DeleteFile(ctx context.Context, fileId string) error
}
```

#### 2. 实现Sync/SyncPath两层模型

分离同步任务与路径配置：

```go
// 同步任务（生命周期较短）
type Sync struct {
    ID       uint
    Status   SyncStatus
    NewStrm  int
    NewMeta  int
}

// 路径配置（长期存在）
type SyncPath struct {
    ID          uint
    SourcePath  string
    TargetPath  string
    Config      SyncPathSetting
    LastSyncAt  int64
}
```

#### 3. STRM生成加入内容校验

避免重复生成STRM文件：

```go
func (s *SyncStrm) CompareStrm(sf *SyncFileCache) int {
    if !helpers.PathExists(strmPath) {
        return 0  // 不存在，需要生成
    }

    // 解析现有STRM，比对关键参数
    strmData := s.LoadDataFromStrm(strmPath)
    if strmData.PickCode != sf.PickCode {
        return 0  // PickCode不匹配，重新生成
    }

    return 1  // 无需更新
}
```

#### 4. 使用errgroup.SetLimit控制并发

```go
import "golang.org/x/sync/errgroup"

eg, ctx := errgroup.WithContext(context.Background())
eg.SetLimit(20)  // 限制并发数

for _, file := range files {
    currentFile := file
    eg.Go(func() error {
        return processFile(currentFile)
    })
}

if err := eg.Wait(); err != nil {
    return err
}
```

### 8.2 中期建议（1-2个月内）

#### 5. 实现增量同步机制

```go
// 记录上次同步时间
lastSyncAt := syncPath.LastSyncAt

// 仅扫描mtime > lastSyncAt的文件
files, err := driver.GetFilesByMtime(ctx, parentId, lastSyncAt)

// 更新同步时间
syncPath.LastSyncAt = time.Now().Unix()
```

#### 6. 添加预取 + 路径补全机制

对于115这样需要路径查询的网盘，分两阶段处理。

### 8.3 长期建议（3-6个月）

#### 7. 多渠道通知系统

集成Telegram、邮件、WebHook等。

#### 8. 完善的状态追踪系统

记录同步每个阶段的耗时、失败原因等。

---

## 9. 立即可用的建议清单

### 🔴 P0（关键，需立即处理）
- [ ] 定义驱动接口，为多源支持做准备
- [ ] 实现Sync/SyncPath两层数据模型
- [ ] STRM生成加入内容一致性校验

### 🟠 P1（重要，需本迭代完成）
- [ ] 引入errgroup.SetLimit控制并发
- [ ] 同步前后暂停/恢复队列
- [ ] 实现统一日志模块

### 🟡 P2（有益，下迭代优先）
- [ ] 实现增量同步机制
- [ ] 添加预取目录+路径补全
- [ ] 集成多渠道通知

### 🟢 P3（优化，可延后）
- [ ] 实现内存缓存临时表
- [ ] 细化状态子状态跟踪
- [ ] 支持同步任务恢复

---

## 10. 总结

QMediaSync 通过**驱动抽象、两层配置、并发限流、增量优化、通知系统**等设计，打造了一个高效、可维护、易扩展的多源网盘同步系统。

**关键要点：**
1. 用策略模式隔离各网盘的差异
2. 用两层配置支持全局+路径级覆盖
3. 用并发限流防止API过载
4. 用内存缓存减少数据库压力
5. 用通知系统提升用户感知

这些设计原则和实现方式值得在 STRMSync 项目中参考和借鉴。

---

**分析完成。**
**分析人：** Claude Haiku 4.5 (Explore Agent)
**文档版本：** v1.0
