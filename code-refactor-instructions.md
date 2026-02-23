# SyncStrm AI 代码重构执行指南 (Codex/Copilot 专用)

这份文档专门为 AI 代码助手（如 Codex, Cursor, GitHub Copilot）编写。它详细拆解了 "Streaming Pipeline v8 (极致架构版)" 的重构步骤，请严格按照以下步骤的顺序和逻辑指示修改代码。

## 🎯 全局重构法则
1.  **全面流式化 (Streaming)**：消灭所有产生 O(N) 内存的切片收集操作，全面改用 O(1) 的 Channel 流式传递（`<-chan RemoteEntry`）。
2.  **拥抱 conc 库 (Concurrency)**：严禁手写 `sync.WaitGroup` 和基于 `chan struct{}` 的信号量。一切需要并发或限流调度的地方，统一使用 `github.com/sourcegraph/conc/pool`。
3.  **消除无效 I/O (Optimize I/O)**：
    *   在目录扫描阶段，禁止对文件进行 `os.Stat` 调用，尤其是面对挂载盘时。
    *   在 STRM 文件的增量判断中，彻底删除对 `.strm` 文件 `ModTime` 的比对，只比对纯文本字符串。

---

## 🛠️ 分步执行指令 (AI Execution Steps)

### Step 1: 适配器接口升级
*   **目标文件**: `backend/internal/infra/filesystem/driver_adapter.go`
*   **指令**:
    *   删除现有的 `List` 方法。
    *   实现新的 `Scan(ctx context.Context, path string, opt syncengine.ListOptions) (<-chan syncengine.RemoteEntry, <-chan error)` 方法。
    *   **逻辑要求**: 在内部调用 `a.client.Scan` 获取底层 `RemoteFile` 通道。启动一个 Goroutine，遍历该通道，将 `RemoteFile` 映射为 `RemoteEntry`，并发送到新的输出通道。

### Step 2: 引擎核心流水线化 (`Engine.RunOnce`)
*   **目标文件**: `backend/internal/engine/engine.go`
*   **指令**:
    *   **强删除**: 彻底删除 `scanRemoteFiles`、`filterFiles`、`processFiles` 这三个私有方法。
    *   **重写 `RunOnce`**:
        1. 调用 `e.driver.Scan` 拿到数据流。
        2. 引入 `github.com/sourcegraph/conc/pool`。
        3. 创建工作池: `p := pool.New().WithMaxGoroutines(e.opts.MaxConcurrency).WithContext(ctx)`。
        4. 遍历通道数据，利用 `p.Go()` 将处理闭包分发到池中。
        5. 在闭包内执行过滤（后缀名/路径），若为媒体文件则调用 `processFile`，若为元数据则预留调用 `processMeta` 的位置。
        6. 在方法末尾调用 `p.Wait()` 收集并返回错误。

### Step 3: STRM 极简比对逻辑 (`processFile`)
*   **目标文件**: `backend/internal/engine/engine.go`
*   **指令**:
    *   定位到 `processFile` 方法。
    *   **强删除**: 删除所有关于获取本地目标 `.strm` 文件修改时间（`ModTime`）的代码，以及任何与时间容差相关的比较逻辑。
    *   **新逻辑**:
        1. 检查配置：如果是 `SkipExisting` 且本地存在，直接 `return nil`。
        2. 读取本地文件文本，与生成的预期文本执行 `strings.Compare`。
        3. 若字符串一致，直接返回；若不一致，执行覆盖写入。

### Step 4: 元数据 Fallback 容错逻辑
*   **目标文件**: `backend/internal/engine/engine.go` (建议新建 `meta.go` 存放)
*   **指令**:
    *   实现一个新的处理方法 `processMeta(ctx context.Context, entry RemoteEntry)`。
    *   **逻辑要求**:
        1. 若配置为 `SkipMeta` 且本地存在，直接跳过。
        2. 若配置为 `UpdateMeta`，比对两端 Size 和 ModTime，一致则跳过。
        3. **Fallback 核心**: 使用标准的 `os.Open` 和 `io.Copy` (结合目标文件的 `os.Create`) 从源挂载路径向本地进行直接覆盖式复制。
        4. 如果在 `os.Open` 或 `io.Copy` 阶段报错，使用 `errors.Is(err, syscall.EIO)` 判断是否为底层 I/O 异常（如网络挂载脱落/超时）。
        5. 如果是，记录 Warn 日志，并调用绑定在配置上的 API Client 执行 `Download` 方法进行 HTTP 兜底拉取。
        6. 成功后，使用 `os.Chtimes` 对齐时间。

### Step 5: 云盘 API 扫描并发改造 (CloudDrive2 / OpenList)
*   **目标文件**: `backend/internal/infra/filesystem/clouddrive2/client.go` & `openlist/client.go`
*   **指令**:
    *   将旧的 `List` 方法替换为实现 `Scan` 接口。
    *   **并发 BFS 设计**:
        1. 引入 `conc/pool`，创建一个并发度（如 `WithMaxGoroutines(5)`）限制的池，专门用于控制对 API 的并发请求量。
        2. 结合递归或任务分发，将请求每个目录及其子目录视为一个独立 Task。
        3. 任务内，在调用 API 前必须调用 `rate.Limiter.Wait()` 进行限流。
        4. 获取到条目后，直接送入结果 Channel，而不是积压在数组中。

### Step 6: 挂载盘极致扫描改造 (Local)
*   **目标文件**: `backend/internal/infra/filesystem/local/client.go`
*   **指令**:
    *   实现 `Scan` 接口。
    *   **强替换**: 废弃标准库的 `filepath.WalkDir`，引入 `github.com/karrick/godirwalk`。
    *   **核心参数**: 调用 `godirwalk.Walk` 时，**必须设置 `Unsorted: true`**。
    *   在回调函数中，将读到的 `Dirent` 直接转换为 `RemoteFile` 送入通道，**绝对禁止在此阶段调用 `os.Stat` 补全文件大小和时间信息**，以此实现秒级挂载盘发现。

### Step 7: 全局任务串行锁防竞态
*   **目标文件**: `backend/internal/queue/queue.go`
*   **指令**:
    *   定位到 `ClaimNext` 方法。
    *   **逻辑要求**: 在提取 `Status = 'pending'` 的任务前，必须在事务中加入前置判定：查询当前是否已存在 `Status = 'running'` 且 `JobID` 与试图领取的任务**相同**的记录。如果有，则跳过该任务，保证同一 Job 绝对串行。
