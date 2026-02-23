# SyncStrm 同步引擎重构指引 (极限架构 & DB 依赖版)

## 一、 核心技术栈选型

1.  **并发与防灾**: `github.com/sourcegraph/conc` (替代手写的 `WaitGroup` 和并发控制)。
2.  **文件树扫描**: `github.com/karrick/godirwalk` (替代标准库，专门优化挂载盘扫描)。
3.  **网络抗性**: `github.com/hashicorp/go-retryablehttp` (替代手写重试和限流退避)。
4.  **分布式任务与锁**: **GORM + 数据库悲观锁** (替代 `asynq`，不引入 Redis)。
5.  **数据映射**: `github.com/mitchellh/mapstructure` (用于解析 Webhook 的异构 JSON)。

## 二、 四大模块重构方案

### 模块 1：底层接口与遍历器 (Scanner) 流式化

*   **接口改造**：将 `Driver` 和 `Client` 接口的 `List` 方法改为返回纯 Channel 的流式接口：`Scan(ctx context.Context, path string, recursive bool, maxDepth int) (<-chan RemoteEntry, <-chan error)`。
*   **API 驱动实现 (`clouddrive2`, `openlist`)**：内部使用 `conc/pool` 构建并发 BFS，限制请求并发数。每次 API 请求作为一个任务抛入池中，拿到结果后推入返回的 `RemoteEntry` Channel。
*   **挂载盘驱动实现 (`local`)**：内部启动一个 Goroutine，调用 `godirwalk.Walk` 且必须开启 `Unsorted: true`。每次拿到文件名立即推入 Channel，将网络 I/O 请求降到最低。

### 模块 2：任务内高并发控制 (Worker Pool 消费者)

*   **废弃旧代码**：彻底删除 `engine.go` 中所有手写的信号量和 WaitGroup 控制逻辑。
*   **拥抱 `conc`**：在 `RunOnce` 中，接收模块 1 吐出的 `fileCh`。
    ```go
    p := pool.New().WithMaxGoroutines(e.opts.MaxConcurrency).WithErrors()
    for file := range fileCh {
        entry := file
        p.Go(func() error {
             if isMedia(entry) { return e.processSTRM(ctx, entry) }
             return e.processMeta(ctx, entry)
        })
    }
    return p.Wait()
    ```

### 模块 3：策略精简与智能 Fallback (核心业务逻辑)

*   **STRM 极速策略**：在 `processSTRM` 中，全盘删去比对文件 `ModTime` 的逻辑。通过 `strings.Compare` 对比预期文本与本地文本。不一致则覆写，一致则秒跳过。
*   **Meta 容错回退**：在 `processMeta` 的拷贝环节，优先 `os.Copy`。如果发生 `syscall.EIO` 等底层错误，**触发降级**，调用底层的 API Client 通过 `Download` 发起 HTTP 下载流重试。

### 模块 4：全局调度互斥锁与 Webhook (基于 DB)

*   **全局串行锁**：在数据库任务队列 (`SyncQueue` 的 `ClaimNext` 方法) 中，**利用 GORM 的事务和悲观锁**：
    ```go
    tx := db.Begin()
    // 锁定 Job 表中对应的行，防止其他 Worker 同时操作
    var job Job
    if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, task.JobID).Error; err != nil {
        // ...
    }
    // 然后安全地领取任务...
    ```
*   **Webhook 数据解耦**：建立 HTTP 接收端点。将接收到的网盘变更载荷，用 `mapstructure` 解析为标准的 `FileEvent`，塞入数据库生成一条 `TaskRun` 记录（标记为 Webhook 模式），交由上述严格的队列锁调度。
*   **引擎直通消费**：引擎 Worker 拿到这个任务后，**跳过模块 1 的扫盘**，直接将这批事件扔进模块 2 的 `conc/pool` 中并发生效。
