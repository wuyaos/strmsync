# SyncStrm 后端同步引擎架构设计 (v8)

## 一、 核心概念与关系模型

### 1. 任务的调度与并发关系 (Task Relationship)
在 SyncStrm 中，必须严格区分全局排队与局部并发：
* **全局：任务间串行 (Sequential Between Jobs)**
    * 引擎应该有一个全局的 Job Queue（目前系统中的 SyncQueue 已经具备这个雏形）。
    * 无论是定时触发、手动点击还是 Webhook 触发，所有的同步请求都会被打包成一个 TaskRun 丢入全局队列。
    * 限制：全局同一时刻只允许一个（或少量严格隔离的）TaskRun 处于运行状态。这彻底避免了不同同步配置（Job）同时操作同一个本地目标目录时产生的文件读写锁冲突或竞态删除。
* **局部：任务内高并发 (Concurrent Within a Task)**
    * 当一个 TaskRun 开始执行时，它内部会有海量的文件需要处理。此时受该配置的 MaxConcurrency 参数控制（比如 Concurrency = 10），启动 10 个 Worker Goroutine 并发处理这上万个文件。

### 2. 核心配置选项字典
* **同步策略 (Sync Strategy)**
    * **FullResync (全量)**：从根目录开始完整扫盘，扫到底（最大限制 25 层）。不仅同步文件，还能揪出本地远端已删除的“孤儿”进行清理。
    * **Incremental (增量)**：
        * **被动事件驱动 (Webhook)**：直接接收外部发来的“变动文件清单”，指哪打哪，完全不扫盘。
        * **极速存在跳过 (Skip-Existing)**：依然扫盘，但只要发现目标文件存在就直接跳过（不对比内容），极速完成新增文件同步。
* **传输与处理模式**
    * **STRMMode**: 生成直链 (URL) 还是本地挂载物理路径 (Local)。
    * **MetadataOptions**: SkipMeta (本地有就跳过) 或 UpdateMeta (增量对比大小和修改时间)。
    * **MetadataMode**: None (不处理) / Copy (系统复制) / Download (调用 API 流下载)。

---

## 二、 流式同步流水线执行流程

### 阶段零：Webhook 常驻接收层 (API Server)
这是一个独立于同步引擎的主流 HTTP 进程（使用 Gin 或 Fiber 启动）：
1. **暴露端点**：监听如 POST `/api/v1/webhook/clouddrive`，持续等待网盘推送。
2. **接收与转换**：接收到 JSON 请求体后，校验来源。将其中的变更事件提取出来，转换为统一的 `FileEvent`。
3. **触发调度**：将这批事件塞入对应 Job 的缓冲队列，并向全局 Job Queue 提交一个带有事件负载的增量 TaskRun 任务排队等待执行。

### 第一阶段：任务定型与管道准备 (全局串行锁内执行)
1. 判断当前 TaskRun 的模式：事件积压处理 / 极速扫描增量 / 全量同步。
2. 创建并发安全的字典 `Sync Index`（用于全量模式扫描清理）。
3. 初始化带缓冲的 `strmTaskCh` 和 `metaTaskCh` 任务通道。

### 第二阶段：统一生产者 (Scanner / Event Extractor)
* **【事件驱动模式】**：从缓冲池拉取积压的 Webhook 事件。若是“删除”事件，当场执行 `os.Remove`；若是“新增/更新”，直接扔进后续的 TaskCh。
* **【主动扫描模式 (全量/极速)】**：
    1. 将指定的扫描根目录入队（此时 `Depth = 0`）。
    2. 多 Goroutine 循环出队，等待 `rate.Limiter` 放行，发起 `List` 目录拉取。
    3. 记录该层级文件的相对路径入 `Sync Index`。将匹配的文件送入 TaskCh。
    4. **深度拦截**：遇到子目录时，判断当前目录的 `Depth + 1` 是否大于 25。如果大于，丢弃该子目录并打印警告；否则压入队列继续。

### 第三阶段：消费者 (Worker Pool) 并发策略处理
启动 `MaxConcurrency` 个 Worker 并发抢任务：

#### 组 A：STRM 处理器
1. 内存中动态拼接期望写入的 STRM 纯文本。
2. （极速扫描增量模式）：本地存在 ➡️ **立刻跳过**。
3. （全量/事件模式）：读取本地现存 STRM，执行**纯字符串比对**。文本完全一致 ➡️ **跳过**。(绝不去比对或更新 STRM 的时间戳)。
4. **覆盖/写入**：原子化写入新生成的 STRM。

#### 组 B：元数据处理器
1. （SkipMeta / 极速模式）：本地存在 ➡️ **立刻跳过**。
2. （UpdateMeta）：比对远近端 Size 和 ModTime，一致 ➡️ **跳过**。
3. **传输与容错 (Fallback)**：
    * **首选机制**：无论怎样都优先尝试底层的 `os.Copy` 直接从源路径物理读取。
    * **⚠️ 自动回退**：若 `os.Copy` 发生 I/O Error（挂载盘读失败/超时），无缝回退，从对应的 CloudDrive2 / OpenList API Client 发起 HTTP Download 下载流兜底。
4. **对齐时间戳**：调用 `os.Chtimes` 将本地时间戳同步为远端 ModTime。

### 第四阶段：收尾清理 (Collector)
(仅在全量同步模式下，且扫描未中断时执行)
反向遍历本地输出目录，提取每个文件的相对路径。如果不在 `Sync Index` 中，执行本地 `os.Remove` 清除。

---

## 三、 推荐架构库与核心伪代码

* **HTTP Server**: `github.com/gin-gonic/gin`
* **高性能并发池**: `github.com/panjf2000/ants/v2`
* **全局任务队列**: `github.com/hibiken/asynq` (基于 Redis，支持延时/重试) 或手写基于 DB 的排队机 (项目内目前的 `syncqueue`)。

### 核心流程伪代码演示

```go
// ===== 阶段 0: Webhook 接收端点 (Gin Controller) =====
func WebhookHandler(c *gin.Context) {
    var payload CloudDriveWebhookPayload
    if err := c.BindJSON(&payload); err != nil {
        c.JSON(400, gin.H{"error": "bad request"})
        return
    }

    // 收到 webhook 事件，将其转换为系统任务并塞入全局数据库排队队列
    // 这样保证了同一个时刻只有一个任务在执行
    EnqueueGlobalTask(JobEvent{
        JobID: payload.JobID,
        Event: payload.FileChange,
    })
    c.JSON(200, gin.H{"status": "ok"})
}

// ===== 阶段 1~4: 引擎核心 (在 Worker 中取出任务后串行执行) =====
func (e *Engine) RunTaskPipeline(ctx context.Context, task TaskRun) error {
    // 此时已经持有了全局执行锁，是安全的单任务环境

    workerPool, _ := ants.NewPool(task.JobConfig.MaxConcurrency)
    defer workerPool.Release()

    syncIndex := &sync.Map{}
    strmCh, metaCh := make(chan RemoteEntry, 1000), make(chan RemoteEntry, 1000)
    var wg sync.WaitGroup

    // 启动消费者分配器
    go e.dispatchToAnts(ctx, strmCh, metaCh, workerPool, &wg)

    if task.HasWebhookEvents {
        // [阶段 2A] Webhook 事件模式
        e.feedEventsToChannel(task.Events, strmCh, metaCh)
    } else {
        // [阶段 2B] BFS 队列扫盘 (带最大深度与限流)
        e.runQueueScanner(ctx, task.JobConfig, syncIndex, strmCh, metaCh)
    }

    // 等待所有处理完成
    close(strmCh)
    close(metaCh)
    wg.Wait()

    // [阶段 4] 收尾清理
    if task.IsFullResync {
        e.cleanupOrphans(task.JobConfig.OutputPath, syncIndex)
    }
    return nil
}

// ===== 扫描器实现 =====
func (e *Engine) runQueueScanner(ctx context.Context, job Job, index *sync.Map, strmCh, metaCh chan<- RemoteEntry) {
    type queueItem struct {
        Path  string
        Depth int // 根目录算作 0
    }
    queue := []queueItem{{Path: "/", Depth: 0}}
    limiter := rate.NewLimiter(rate.Limit(job.QPS), 1)

    for len(queue) > 0 {
        if ctx.Err() != nil { return }

        item := queue[0]
        queue = queue[1:]

        // 深度限制 25 层
        if item.Depth >= 25 {
            log.Warn("Max depth 25 reached at", item.Path)
            continue
        }

        // 等待限流器
        _ = limiter.Wait(ctx)

        entries, _ := e.driver.List(ctx, item.Path)
        for _, entry := range entries {
            index.Store(entry.Path, struct{}{}) // 记入全量索引

            if entry.IsDir {
                queue = append(queue, queueItem{Path: entry.Path, Depth: item.Depth + 1})
            } else if isMedia(entry) {
                strmCh <- entry
            } else if isMeta(entry) {
                metaCh <- entry
            }
        }
    }
}

// ===== 元数据 Fallback 容错实现 (运行在 ants 的 goroutine 中) =====
func (e *Engine) processMeta(entry RemoteEntry, config JobConfig) {
    // 假设已经通过了 UpdateMeta 大小/时间的比对判断

    // 首选 os.Copy (速度极快)
    err := os.Copy(targetPath, entry.MountedLocalPath)

    if err != nil && isIOError(err) {
        log.Warn("OS Copy failed, fallback to API Download", entry.Path)
        // 挂载盘掉线或阻塞，启动无缝回退，调用 API 下载流
        err = e.apiClient.Download(entry.RemoteAPIPath, targetPath)
    }

    if err == nil {
        os.Chtimes(targetPath, entry.ModTime, entry.ModTime)
    }
}
```
