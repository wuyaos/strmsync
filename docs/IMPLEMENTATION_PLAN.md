# STRMSync 详细实施方案

## 目录

- [1. STRM 生成规则](#1-strm-生成规则)
- [2. 数据库设计](#2-数据库设计)
- [3. 核心服务架构](#3-核心服务架构)
- [4. 数据源适配器设计](#4-数据源适配器设计)
- [5. 实施步骤](#5-实施步骤)
- [6. API 设计](#6-api-设计)
- [7. Docker 部署方案](#7-docker-部署方案)

---

## 1. STRM 生成规则

### 1.1 CloudDrive2（仅本地挂载模式）

#### 配置示例

```yaml
type: clouddrive2
config:
  mount_path: /mnt/clouddrive
mapping:
  source_prefix: /mnt/clouddrive/115/Movies
  target_prefix: /media/library/Movies
```

#### 生成规则

| 源文件路径 | STRM 文件路径 | STRM 文件内容 |
|-----------|--------------|--------------|
| `/mnt/clouddrive/115/Movies/Action/Movie.mkv` | `/media/library/Movies/Action/Movie.strm` | `/mnt/clouddrive/115/Movies/Action/Movie.mkv` |
| `/mnt/clouddrive/Aliyun/TVShows/Show/S01E01.mkv` | `/media/library/TVShows/Show/S01E01.strm` | `/mnt/clouddrive/Aliyun/TVShows/Show/S01E01.mkv` |

#### Go 实现

```go
func (a *CloudDrive2Adapter) GenerateSTRMContent(sourcePath string) string {
    // CloudDrive2 STRM 内容就是本地挂载路径
    return sourcePath
}

// 示例
// 输入: /mnt/clouddrive/115/Movies/Action/Movie.mkv
// 输出: /mnt/clouddrive/115/Movies/Action/Movie.mkv
```

---

### 1.2 OpenList（两种模式）

#### 模式 1: HTTP 模式（默认）

**配置示例**

```yaml
type: openlist
config:
  api_url: http://localhost:5244
  strm_mode: http
mapping:
  source_prefix: /Movies
  target_prefix: /media/library/Movies
```

**生成规则**

| OpenList 路径 | STRM 文件路径 | STRM 文件内容 |
|--------------|--------------|--------------|
| `/Movies/Action/Movie.mkv` | `/media/library/Movies/Action/Movie.strm` | `http://localhost:5244/d/Movies/Action/Movie.mkv` |
| `/TVShows/Show/S01E01.mkv` | `/media/library/TVShows/Show/S01E01.strm` | `http://localhost:5244/d/TVShows/Show/S01E01.mkv` |

**Go 实现**

```go
func (a *OpenListAdapter) GenerateSTRMContent(openlistPath string) string {
    if a.config.STRMMode == "http" {
        // HTTP 模式：生成下载链接
        return fmt.Sprintf("%s/d%s", a.config.APIURL, openlistPath)
    }
    // ...
}

// 示例
// 输入: /Movies/Action/Movie.mkv
// 输出: http://localhost:5244/d/Movies/Action/Movie.mkv
```

---

#### 模式 2: 本地挂载模式

**前置条件**：用户需要通过 rclone 或 CloudDrive2 将 OpenList 挂载到本地。

**配置示例**

```yaml
type: openlist
config:
  api_url: http://localhost:5244
  strm_mode: local              # 本地模式
  mount_path: /mnt/openlist     # 手动指定的挂载目录
mapping:
  source_prefix: /Movies        # OpenList 内部路径
  target_prefix: /media/library/Movies
```

**路径转换规则**

```
OpenList API 路径:  /Movies/Action/Movie.mkv
本地挂载路径:       /mnt/openlist/Movies/Action/Movie.mkv
STRM 文件路径:      /media/library/Movies/Action/Movie.strm
STRM 文件内容:      /mnt/openlist/Movies/Action/Movie.mkv
```

**Go 实现**

```go
func (a *OpenListAdapter) GenerateSTRMContent(openlistPath string) string {
    if a.config.STRMMode == "http" {
        return fmt.Sprintf("%s/d%s", a.config.APIURL, openlistPath)
    } else if a.config.STRMMode == "local" {
        // 本地模式：转换为挂载路径
        // OpenList 路径: /Movies/Action/Movie.mkv
        // 挂载路径: /mnt/openlist + /Movies/Action/Movie.mkv
        return filepath.Join(a.config.MountPath, openlistPath)
    }
    return ""
}

// 示例
// 输入: /Movies/Action/Movie.mkv
// 输出: /mnt/openlist/Movies/Action/Movie.mkv
```

---

### 1.3 本地文件系统

#### 配置示例

```yaml
type: local
mapping:
  source_prefix: /volume1/Media/Movies
  target_prefix: /media/library/Movies
```

#### 生成规则

| 源文件路径 | STRM 文件路径 | STRM 文件内容 |
|-----------|--------------|--------------|
| `/volume1/Media/Movies/Action/Movie.mkv` | `/media/library/Movies/Action/Movie.strm` | `/volume1/Media/Movies/Action/Movie.mkv` |

#### Go 实现

```go
func (a *LocalAdapter) GenerateSTRMContent(sourcePath string) string {
    // 本地文件系统 STRM 内容就是源文件路径
    return sourcePath
}
```

---

### 1.4 STRM 生成流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                    扫描数据源（API 或本地）                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              遍历文件，过滤视频文件（.mkv/.mp4/...）             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                计算快速哈希（头 1MB + 尾 1MB + size）            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                 检查数据库：是否已存在相同哈希                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                    已存在 ──┴── 不存在
                      │              │
                      │              ▼
                      │    ┌─────────────────────────────────┐
                      │    │   生成目标 STRM 文件路径        │
                      │    │   (source_prefix → target_prefix)│
                      │    └─────────────────────────────────┘
                      │              │
                      │              ▼
                      │    ┌─────────────────────────────────┐
                      │    │   根据数据源类型生成 STRM 内容  │
                      │    │   - CloudDrive2: 本地路径       │
                      │    │   - OpenList HTTP: HTTP URL     │
                      │    │   - OpenList Local: 本地路径    │
                      │    │   - Local: 本地路径             │
                      │    └─────────────────────────────────┘
                      │              │
                      │              ▼
                      │    ┌─────────────────────────────────┐
                      │    │   写入 STRM 文件                │
                      │    └─────────────────────────────────┘
                      │              │
                      │              ▼
                      │    ┌─────────────────────────────────┐
                      │    │   复制元数据文件                │
                      │    │   (.nfo/.jpg/.srt 等)           │
                      │    └─────────────────────────────────┘
                      │              │
                      └──────────────┼───────────────────────┐
                                     ▼                       │
                           ┌─────────────────────┐          │
                           │   写入数据库索引    │          │
                           └─────────────────────┘          │
                                     │                       │
                                     ▼                       │
                           ┌─────────────────────┐          │
                           │   发送媒体库通知    │◄─────────┘
                           │   (Emby/Jellyfin)   │
                           └─────────────────────┘
```

---

## 2. 数据库设计

### 2.1 表结构

#### sources（数据源表）

```sql
CREATE TABLE sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL UNIQUE,          -- 数据源名称
    type VARCHAR(50) NOT NULL,                  -- 类型：local/clouddrive2/openlist
    enabled BOOLEAN NOT NULL DEFAULT TRUE,       -- 是否启用
    config TEXT NOT NULL,                        -- JSON 配置（认证、挂载路径等）
    source_prefix VARCHAR(500) NOT NULL,         -- 源路径前缀
    target_prefix VARCHAR(500) NOT NULL,         -- 目标路径前缀
    options TEXT,                                -- JSON 高级选项
    status VARCHAR(50) DEFAULT 'idle',           -- 状态：idle/scanning/error
    last_scan_at DATETIME,                       -- 最后扫描时间
    file_count INTEGER DEFAULT 0,                -- 文件数量
    error_message TEXT,                          -- 错误信息
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**索引**:
```sql
CREATE INDEX idx_sources_type ON sources(type);
CREATE INDEX idx_sources_enabled ON sources(enabled);
```

---

#### files（文件索引表）

```sql
CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id INTEGER NOT NULL,                  -- 关联 sources.id
    source_path VARCHAR(1000) NOT NULL,          -- 源文件路径
    target_path VARCHAR(1000) NOT NULL,          -- 目标 STRM 路径
    strm_content TEXT NOT NULL,                  -- STRM 文件内容
    file_name VARCHAR(500) NOT NULL,             -- 文件名
    file_size BIGINT NOT NULL,                   -- 文件大小（字节）
    file_hash VARCHAR(64) NOT NULL,              -- 快速哈希
    modified_at DATETIME NOT NULL,               -- 文件修改时间
    is_dir BOOLEAN NOT NULL DEFAULT FALSE,       -- 是否为目录
    strm_generated BOOLEAN DEFAULT FALSE,        -- STRM 是否已生成
    metadata_synced BOOLEAN DEFAULT FALSE,       -- 元数据是否已同步
    notified BOOLEAN DEFAULT FALSE,              -- 是否已通知媒体库
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
);
```

**索引**:
```sql
CREATE INDEX idx_files_source_id ON files(source_id);
CREATE INDEX idx_files_file_hash ON files(file_hash);
CREATE INDEX idx_files_source_path ON files(source_path);
CREATE UNIQUE INDEX idx_files_unique ON files(source_id, source_path);
```

---

#### metadata_files（元数据文件表）

```sql
CREATE TABLE metadata_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,                    -- 关联 files.id
    source_path VARCHAR(1000) NOT NULL,          -- 源元数据路径
    target_path VARCHAR(1000) NOT NULL,          -- 目标元数据路径
    file_type VARCHAR(50) NOT NULL,              -- 类型：nfo/poster/fanart/subtitle
    synced BOOLEAN DEFAULT FALSE,                -- 是否已同步
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);
```

**索引**:
```sql
CREATE INDEX idx_metadata_files_file_id ON metadata_files(file_id);
CREATE INDEX idx_metadata_files_synced ON metadata_files(synced);
```

---

#### tasks（任务表）

```sql
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,                  -- 任务名称
    type VARCHAR(50) NOT NULL,                   -- 类型：scan/watch/sync/notify
    source_id INTEGER,                           -- 关联 sources.id（可为空）
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 状态：pending/running/completed/failed
    progress INTEGER DEFAULT 0,                  -- 进度（0-100）
    total_files INTEGER DEFAULT 0,               -- 总文件数
    processed_files INTEGER DEFAULT 0,           -- 已处理文件数
    failed_files INTEGER DEFAULT 0,              -- 失败文件数
    error_message TEXT,                          -- 错误信息
    started_at DATETIME,                         -- 开始时间
    completed_at DATETIME,                       -- 完成时间
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE SET NULL
);
```

**索引**:
```sql
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_source_id ON tasks(source_id);
CREATE INDEX idx_tasks_created_at ON tasks(created_at DESC);
```

---

#### settings（系统设置表）

```sql
CREATE TABLE settings (
    key VARCHAR(255) PRIMARY KEY,                -- 设置键
    value TEXT NOT NULL,                         -- 设置值（JSON）
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

### 2.2 GORM 模型定义

```go
// internal/database/models.go
package database

import (
    "time"
    "gorm.io/gorm"
)

// Source 数据源模型
type Source struct {
    ID           uint      `gorm:"primaryKey" json:"id"`
    Name         string    `gorm:"uniqueIndex;not null" json:"name"`
    Type         string    `gorm:"index;not null" json:"type"` // local/clouddrive2/openlist
    Enabled      bool      `gorm:"not null;default:true" json:"enabled"`
    Config       string    `gorm:"type:text;not null" json:"config"` // JSON
    SourcePrefix string    `gorm:"not null" json:"source_prefix"`
    TargetPrefix string    `gorm:"not null" json:"target_prefix"`
    Options      string    `gorm:"type:text" json:"options"` // JSON
    Status       string    `gorm:"default:'idle'" json:"status"`
    LastScanAt   *time.Time `json:"last_scan_at"`
    FileCount    int       `gorm:"default:0" json:"file_count"`
    ErrorMessage string    `gorm:"type:text" json:"error_message"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`

    // 关联
    Files []File `gorm:"foreignKey:SourceID;constraint:OnDelete:CASCADE" json:"files,omitempty"`
    Tasks []Task `gorm:"foreignKey:SourceID;constraint:OnDelete:SET NULL" json:"tasks,omitempty"`
}

// File 文件索引模型
type File struct {
    ID             uint      `gorm:"primaryKey" json:"id"`
    SourceID       uint      `gorm:"index;not null" json:"source_id"`
    SourcePath     string    `gorm:"index;not null" json:"source_path"`
    TargetPath     string    `gorm:"not null" json:"target_path"`
    STRMContent    string    `gorm:"type:text;not null" json:"strm_content"`
    FileName       string    `gorm:"not null" json:"file_name"`
    FileSize       int64     `gorm:"not null" json:"file_size"`
    FileHash       string    `gorm:"index;not null" json:"file_hash"`
    ModifiedAt     time.Time `gorm:"not null" json:"modified_at"`
    IsDir          bool      `gorm:"not null;default:false" json:"is_dir"`
    STRMGenerated  bool      `gorm:"default:false" json:"strm_generated"`
    MetadataSynced bool      `gorm:"default:false" json:"metadata_synced"`
    Notified       bool      `gorm:"default:false" json:"notified"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`

    // 关联
    Source        Source         `gorm:"foreignKey:SourceID" json:"source,omitempty"`
    MetadataFiles []MetadataFile `gorm:"foreignKey:FileID;constraint:OnDelete:CASCADE" json:"metadata_files,omitempty"`
}

// MetadataFile 元数据文件模型
type MetadataFile struct {
    ID         uint      `gorm:"primaryKey" json:"id"`
    FileID     uint      `gorm:"index;not null" json:"file_id"`
    SourcePath string    `gorm:"not null" json:"source_path"`
    TargetPath string    `gorm:"not null" json:"target_path"`
    FileType   string    `gorm:"not null" json:"file_type"` // nfo/poster/fanart/subtitle
    Synced     bool      `gorm:"index;default:false" json:"synced"`
    CreatedAt  time.Time `json:"created_at"`

    // 关联
    File File `gorm:"foreignKey:FileID" json:"file,omitempty"`
}

// Task 任务模型
type Task struct {
    ID             uint       `gorm:"primaryKey" json:"id"`
    Name           string     `gorm:"not null" json:"name"`
    Type           string     `gorm:"not null" json:"type"` // scan/watch/sync/notify
    SourceID       *uint      `gorm:"index" json:"source_id"`
    Status         string     `gorm:"index;not null;default:'pending'" json:"status"`
    Progress       int        `gorm:"default:0" json:"progress"`
    TotalFiles     int        `gorm:"default:0" json:"total_files"`
    ProcessedFiles int        `gorm:"default:0" json:"processed_files"`
    FailedFiles    int        `gorm:"default:0" json:"failed_files"`
    ErrorMessage   string     `gorm:"type:text" json:"error_message"`
    StartedAt      *time.Time `json:"started_at"`
    CompletedAt    *time.Time `json:"completed_at"`
    CreatedAt      time.Time  `gorm:"index:idx_tasks_created_at,sort:desc" json:"created_at"`

    // 关联
    Source *Source `gorm:"foreignKey:SourceID" json:"source,omitempty"`
}

// Setting 系统设置模型
type Setting struct {
    Key       string    `gorm:"primaryKey" json:"key"`
    Value     string    `gorm:"type:text;not null" json:"value"` // JSON
    UpdatedAt time.Time `json:"updated_at"`
}

// TableName 方法用于指定表名
func (Source) TableName() string       { return "sources" }
func (File) TableName() string         { return "files" }
func (MetadataFile) TableName() string { return "metadata_files" }
func (Task) TableName() string         { return "tasks" }
func (Setting) TableName() string      { return "settings" }
```

---

## 3. 核心服务架构

### 3.1 架构图

```
┌──────────────────────────────────────────────────────────────────┐
│                          HTTP API Layer (Gin)                     │
│  /api/sources, /api/tasks, /api/files, /api/settings, /api/health│
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│                      Orchestrator Service                         │
│  (任务调度、协调各服务、事件分发)                                  │
└──────────────────────────────────────────────────────────────────┘
       │              │              │              │
       ▼              ▼              ▼              ▼
┌─────────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐
│   Scanner   │ │  Watcher │ │Metadata  │ │   Notifier    │
│   Service   │ │  Service │ │  Sync    │ │   Service     │
│             │ │          │ │ Service  │ │               │
│ 文件扫描    │ │文件监控  │ │元数据同步│ │ Emby/Jellyfin │
└─────────────┘ └──────────┘ └──────────┘ └───────────────┘
       │              │              │              │
       └──────────────┴──────────────┴──────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│                       Adapter Layer                               │
│  LocalAdapter │ CloudDrive2Adapter │ OpenListAdapter             │
└──────────────────────────────────────────────────────────────────┘
       │                      │                      │
       ▼                      ▼                      ▼
┌─────────────┐    ┌──────────────────┐    ┌──────────────┐
│Local FileSystem│  │CloudDrive2 gRPC  │    │OpenList REST │
└─────────────┘    └──────────────────┘    └──────────────┘
```

---

### 3.2 服务职责

#### 3.2.1 Orchestrator（编排器）

**职责**:
- 任务调度和队列管理
- 协调各个服务
- 事件分发
- 状态管理

**接口**:
```go
type Orchestrator interface {
    // 任务管理
    CreateTask(task *Task) error
    StartTask(taskID uint) error
    StopTask(taskID uint) error
    GetTaskStatus(taskID uint) (*TaskStatus, error)

    // 事件处理
    OnFileChanged(event FileEvent) error
    OnScanCompleted(sourceID uint) error
}
```

---

#### 3.2.2 Scanner（扫描器）

**职责**:
- 遍历数据源文件
- 计算文件哈希
- 生成 STRM 文件
- 写入数据库索引

**接口**:
```go
type Scanner interface {
    Scan(sourceID uint) error
    ScanDirectory(sourceID uint, path string) error
    GetProgress(taskID uint) (*Progress, error)
}
```

**实现**:
```go
// internal/services/scanner.go
type ScannerService struct {
    db       *gorm.DB
    pool     *ants.Pool
    adapters map[string]Adapter
}

func (s *ScannerService) Scan(sourceID uint) error {
    // 1. 加载数据源配置
    source, err := s.loadSource(sourceID)
    if err != nil {
        return err
    }

    // 2. 获取适配器
    adapter := s.adapters[source.Type]

    // 3. 获取文件列表
    files, err := adapter.ListFiles(source.SourcePrefix, true)
    if err != nil {
        return err
    }

    // 4. 并发处理
    for _, file := range files {
        file := file
        s.pool.Submit(func() {
            s.processFile(source, file)
        })
    }

    return nil
}

func (s *ScannerService) processFile(source *Source, file FileInfo) error {
    // 1. 过滤视频文件
    if !isVideoFile(file.Name) {
        return nil
    }

    // 2. 计算快速哈希
    hash, err := calculateFastHash(file.Path, 1024*1024)
    if err != nil {
        return err
    }

    // 3. 检查数据库
    exists, err := s.checkFileExists(source.ID, file.Path, hash)
    if err != nil {
        return err
    }
    if exists {
        return nil // 文件已存在，跳过
    }

    // 4. 生成目标路径
    targetPath := s.generateTargetPath(source, file.Path)

    // 5. 生成 STRM 内容
    adapter := s.adapters[source.Type]
    strmContent := adapter.GenerateSTRMContent(file.Path)

    // 6. 写入 STRM 文件
    err = s.writeSTRMFile(targetPath, strmContent)
    if err != nil {
        return err
    }

    // 7. 写入数据库
    dbFile := &File{
        SourceID:      source.ID,
        SourcePath:    file.Path,
        TargetPath:    targetPath,
        STRMContent:   strmContent,
        FileName:      file.Name,
        FileSize:      file.Size,
        FileHash:      hash,
        ModifiedAt:    file.ModifiedAt,
        STRMGenerated: true,
    }
    return s.db.Create(dbFile).Error
}
```

---

#### 3.2.3 Watcher（监控器）

**职责**:
- 监控文件系统变化（仅本地和挂载模式）
- 处理文件创建/修改/删除事件
- 触发增量更新

**接口**:
```go
type Watcher interface {
    Watch(sourceID uint) error
    Stop(sourceID uint) error
}
```

**实现**:
```go
// internal/services/watcher.go
type WatcherService struct {
    watchers map[uint]*fsnotify.Watcher
    scanner  *ScannerService
}

func (w *WatcherService) Watch(sourceID uint) error {
    source, err := w.loadSource(sourceID)
    if err != nil {
        return err
    }

    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }

    w.watchers[sourceID] = watcher

    // 添加监控路径
    err = watcher.Add(source.SourcePrefix)
    if err != nil {
        return err
    }

    // 事件处理
    go func() {
        for {
            select {
            case event := <-watcher.Events:
                w.handleEvent(source, event)
            case err := <-watcher.Errors:
                log.Error("watcher error", zap.Error(err))
            }
        }
    }()

    return nil
}

func (w *WatcherService) handleEvent(source *Source, event fsnotify.Event) {
    // 去抖处理
    time.Sleep(time.Second * 2)

    switch event.Op {
    case fsnotify.Create, fsnotify.Write:
        // 文件创建或修改
        w.scanner.processFile(source, event.Name)
    case fsnotify.Remove:
        // 文件删除
        w.handleFileDelete(source, event.Name)
    }
}
```

---

#### 3.2.4 Metadata Sync（元数据同步）

**职责**:
- 识别元数据文件（.nfo/.jpg/.srt）
- 复制元数据到目标目录
- 管理元数据索引

**接口**:
```go
type MetadataSync interface {
    SyncMetadata(fileID uint) error
    SyncMetadataForSource(sourceID uint) error
}
```

**元数据类型**:
```go
const (
    MetadataTypeNFO      = "nfo"      // NFO 元数据
    MetadataTypePoster   = "poster"   // 海报 (.*-poster.jpg)
    MetadataTypeFanart   = "fanart"   // 背景图 (.*-fanart.jpg)
    MetadataTypeSubtitle = "subtitle" // 字幕 (.srt/.ass/.ssa)
)
```

---

#### 3.2.5 Notifier（通知器）

**职责**:
- 通知 Emby/Jellyfin 刷新媒体库
- 管理通知队列
- 处理通知失败重试

**接口**:
```go
type Notifier interface {
    NotifyLibraryUpdate(path string) error
    NotifyBatch(paths []string) error
}
```

**实现**:
```go
// internal/services/notifier.go
type NotifierService struct {
    emby     *EmbyNotifier
    jellyfin *JellyfinNotifier
}

func (n *NotifierService) NotifyLibraryUpdate(path string) error {
    var errs []error

    if n.emby != nil && n.emby.Enabled {
        err := n.emby.Refresh(path)
        if err != nil {
            errs = append(errs, err)
        }
    }

    if n.jellyfin != nil && n.jellyfin.Enabled {
        err := n.jellyfin.Refresh(path)
        if err != nil {
            errs = append(errs, err)
        }
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}
```

---

## 4. 数据源适配器设计

### 4.1 适配器接口

```go
// internal/adapters/adapter.go
package adapters

type FileInfo struct {
    Name       string
    Path       string
    Size       int64
    ModifiedAt time.Time
    IsDir      bool
}

type Adapter interface {
    // 连接和认证
    Connect() error
    Disconnect() error
    HealthCheck() error

    // 文件操作
    ListFiles(path string, recursive bool) ([]FileInfo, error)
    GetFileInfo(path string) (*FileInfo, error)

    // STRM 生成
    GenerateSTRMContent(sourcePath string) string

    // 能力查询
    SupportsWatch() bool
    SupportsMetadata() bool
}
```

---

### 4.2 LocalAdapter（本地文件系统）

```go
// internal/adapters/local.go
type LocalAdapter struct {
    config *LocalConfig
}

type LocalConfig struct {
    SourcePrefix string
    TargetPrefix string
}

func (a *LocalAdapter) ListFiles(path string, recursive bool) ([]FileInfo, error) {
    var files []FileInfo

    if recursive {
        err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
            if err != nil {
                return err
            }

            info, _ := d.Info()
            files = append(files, FileInfo{
                Name:       d.Name(),
                Path:       p,
                Size:       info.Size(),
                ModifiedAt: info.ModTime(),
                IsDir:      d.IsDir(),
            })
            return nil
        })
        return files, err
    }

    // 非递归
    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, err
    }

    for _, entry := range entries {
        info, _ := entry.Info()
        files = append(files, FileInfo{
            Name:       entry.Name(),
            Path:       filepath.Join(path, entry.Name()),
            Size:       info.Size(),
            ModifiedAt: info.ModTime(),
            IsDir:      entry.IsDir(),
        })
    }

    return files, nil
}

func (a *LocalAdapter) GenerateSTRMContent(sourcePath string) string {
    return sourcePath // 本地路径直接返回
}

func (a *LocalAdapter) SupportsWatch() bool {
    return true // 本地文件系统支持监控
}
```

---

### 4.3 CloudDrive2Adapter

```go
// internal/adapters/clouddrive2.go
type CloudDrive2Adapter struct {
    client     pb.CloudDriveFileSrvClient
    config     *CloudDrive2Config
    token      string
}

type CloudDrive2Config struct {
    APIURL    string
    AuthMode  string // password / token
    Username  string
    Password  string
    APIToken  string
    MountPath string
}

func (a *CloudDrive2Adapter) Connect() error {
    conn, err := grpc.Dial(a.config.APIURL, grpc.WithInsecure())
    if err != nil {
        return err
    }

    a.client = pb.NewCloudDriveFileSrvClient(conn)

    // 获取 Token
    if a.config.AuthMode == "password" {
        resp, err := a.client.GetToken(context.Background(), &pb.GetTokenRequest{
            Username: a.config.Username,
            Password: a.config.Password,
        })
        if err != nil {
            return err
        }
        a.token = resp.Token
    } else {
        a.token = a.config.APIToken
    }

    return nil
}

func (a *CloudDrive2Adapter) ListFiles(path string, recursive bool) ([]FileInfo, error) {
    ctx := metadata.AppendToOutgoingContext(context.Background(),
        "authorization", "Bearer "+a.token)

    resp, err := a.client.List(ctx, &pb.ListRequest{
        Path:    path,
        Page:    1,
        PerPage: 0, // 不分页
        Refresh: false,
    })

    if err != nil {
        return nil, err
    }

    if !resp.Success {
        return nil, errors.New(resp.ErrorMessage)
    }

    var files []FileInfo
    for _, f := range resp.Files {
        files = append(files, FileInfo{
            Name:       f.Name,
            Path:       f.Path,
            Size:       f.Size,
            ModifiedAt: time.Unix(f.ModifyTime, 0),
            IsDir:      f.IsDir,
        })
    }

    return files, nil
}

func (a *CloudDrive2Adapter) GenerateSTRMContent(sourcePath string) string {
    // CloudDrive2 只使用本地挂载路径
    // sourcePath 应该已经是挂载路径（如 /mnt/clouddrive/115/Movies/Movie.mkv）
    return sourcePath
}

func (a *CloudDrive2Adapter) SupportsWatch() bool {
    return true // 挂载模式支持监控
}
```

---

### 4.4 OpenListAdapter

```go
// internal/adapters/openlist.go
type OpenListAdapter struct {
    client *http.Client
    config *OpenListConfig
    token  string
}

type OpenListConfig struct {
    APIURL    string
    AuthMode  string // password / token
    Username  string
    Password  string
    Token     string
    STRMMode  string // http / local
    MountPath string // 仅 local 模式需要
}

func (a *OpenListAdapter) Connect() error {
    a.client = &http.Client{Timeout: 30 * time.Second}

    if a.config.AuthMode == "password" {
        // 登录获取 Token
        token, err := a.login()
        if err != nil {
            return err
        }
        a.token = token
    } else {
        a.token = a.config.Token
    }

    return nil
}

func (a *OpenListAdapter) login() (string, error) {
    data := map[string]string{
        "username": a.config.Username,
        "password": a.config.Password,
    }

    jsonData, _ := json.Marshal(data)
    resp, err := a.client.Post(
        a.config.APIURL+"/api/auth/login",
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result struct {
        Code    int `json:"code"`
        Message string `json:"message"`
        Data    struct {
            Token string `json:"token"`
        } `json:"data"`
    }

    json.NewDecoder(resp.Body).Decode(&result)

    if result.Code != 200 {
        return "", errors.New(result.Message)
    }

    return result.Data.Token, nil
}

func (a *OpenListAdapter) ListFiles(path string, recursive bool) ([]FileInfo, error) {
    data := map[string]interface{}{
        "path":     path,
        "password": "",
        "page":     1,
        "per_page": 0,
        "refresh":  false,
    }

    jsonData, _ := json.Marshal(data)
    req, _ := http.NewRequest("POST", a.config.APIURL+"/api/fs/list", bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", a.token)

    resp, err := a.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        Code    int `json:"code"`
        Message string `json:"message"`
        Data    struct {
            Content []struct {
                Name     string `json:"name"`
                Size     int64  `json:"size"`
                IsDir    bool   `json:"is_dir"`
                Modified string `json:"modified"`
            } `json:"content"`
        } `json:"data"`
    }

    json.NewDecoder(resp.Body).Decode(&result)

    if result.Code != 200 {
        return nil, errors.New(result.Message)
    }

    var files []FileInfo
    for _, f := range result.Data.Content {
        modTime, _ := time.Parse(time.RFC3339, f.Modified)
        files = append(files, FileInfo{
            Name:       f.Name,
            Path:       filepath.Join(path, f.Name),
            Size:       f.Size,
            ModifiedAt: modTime,
            IsDir:      f.IsDir,
        })
    }

    return files, nil
}

func (a *OpenListAdapter) GenerateSTRMContent(openlistPath string) string {
    if a.config.STRMMode == "http" {
        // HTTP 模式：生成下载链接
        return fmt.Sprintf("%s/d%s", a.config.APIURL, openlistPath)
    } else if a.config.STRMMode == "local" {
        // 本地模式：转换为挂载路径
        return filepath.Join(a.config.MountPath, openlistPath)
    }
    return ""
}

func (a *OpenListAdapter) SupportsWatch() bool {
    // 仅本地模式支持监控
    return a.config.STRMMode == "local"
}
```

---

## 5. 实施步骤

### 阶段 1: 项目骨架（第 1 周）

**目标**: 搭建基础架构和 Docker 环境

**任务清单**:
- [ ] 初始化 Go 项目（`go mod init`）
- [ ] 创建项目目录结构
- [ ] 配置 Makefile
- [ ] 实现配置管理（Viper）
- [ ] 实现数据库层（GORM + SQLite）
- [ ] 实现日志系统（Zap）
- [ ] 创建 Docker Compose
- [ ] 实现健康检查接口

**验收标准**:
- Docker 容器正常启动
- 数据库表自动创建
- `/api/health` 接口返回 200

---

### 阶段 2: 本地文件扫描（第 2 周）

**目标**: 实现本地文件系统的扫描和索引

**任务清单**:
- [ ] 实现 LocalAdapter
- [ ] 实现 Scanner 服务（并发扫描）
- [ ] 实现快速哈希算法
- [ ] 实现 STRM 文件生成
- [ ] 实现文件索引写入数据库
- [ ] 实现数据源管理 API

**验收标准**:
- 能扫描 10 万文件并建立索引
- 哈希计算准确
- STRM 文件正确生成

---

### 阶段 3: CloudDrive2 集成（第 3 周）

**目标**: 支持 CloudDrive2 数据源

**任务清单**:
- [ ] 实现 CloudDrive2Adapter（gRPC 客户端）
- [ ] 实现认证（密码/Token 两种模式）
- [ ] 实现文件列表获取
- [ ] 实现 STRM 生成（本地挂载模式）
- [ ] 实现 API 降级逻辑
- [ ] 添加健康检查

**验收标准**:
- 能连接 CloudDrive2
- 能列出挂载文件
- STRM 指向正确的挂载路径

---

### 阶段 4: OpenList 集成（第 4 周）

**目标**: 支持 OpenList 数据源（HTTP 和本地两种模式）

**任务清单**:
- [ ] 实现 OpenListAdapter（REST 客户端）
- [ ] 实现认证（密码/Token 两种模式）
- [ ] 实现 HTTP 模式 STRM 生成
- [ ] 实现本地模式 STRM 生成
- [ ] 实现文件搜索
- [ ] 添加配置验证

**验收标准**:
- HTTP 模式：STRM 包含正确的下载 URL
- 本地模式：STRM 包含正确的挂载路径
- 两种模式可正确切换

---

### 阶段 5: 文件监控和增量更新（第 5 周）

**目标**: 实时监控文件变化

**任务清单**:
- [ ] 实现 Watcher 服务（fsnotify）
- [ ] 实现事件处理器（创建/修改/删除）
- [ ] 实现事件去抖
- [ ] 实现增量更新逻辑
- [ ] 添加监控开关

**验收标准**:
- 文件变更 < 5 秒检测到
- 增量更新正确触发
- 无重复处理

---

### 阶段 6: 元数据同步（第 6 周）

**目标**: 自动同步元数据文件

**任务清单**:
- [ ] 实现元数据文件识别
- [ ] 实现元数据复制
- [ ] 实现元数据索引
- [ ] 支持 NFO/海报/字幕同步

**验收标准**:
- 元数据文件正确复制
- 目录结构保持一致

---

### 阶段 7: 媒体库通知（第 7 周）

**目标**: 集成 Emby/Jellyfin

**任务清单**:
- [ ] 实现 EmbyNotifier
- [ ] 实现 JellyfinNotifier
- [ ] 实现通知队列
- [ ] 实现失败重试
- [ ] 实现批量通知

**验收标准**:
- Emby/Jellyfin 能收到通知
- 媒体库正确刷新

---

### 阶段 8: Web 前端（第 8-9 周）

**目标**: 构建 Vue 3 管理界面

**任务清单**:
- [ ] 初始化 Vue 3 项目
- [ ] 集成 Element Plus
- [ ] 实现仪表盘
- [ ] 实现数据源管理
- [ ] 实现任务管理
- [ ] 实现文件浏览
- [ ] 实现系统设置

**验收标准**:
- 所有页面功能正常
- UI 响应流畅

---

### 阶段 9: 测试和优化（第 10 周）

**目标**: 测试和性能优化

**任务清单**:
- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能测试（10 万文件）
- [ ] 内存优化
- [ ] 并发优化
- [ ] 文档完善

---

## 6. API 设计

### 6.1 数据源 API

#### GET /api/sources
列出所有数据源

**响应**:
```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 1,
      "name": "本地电影",
      "type": "local",
      "enabled": true,
      "source_prefix": "/volume1/Media/Movies",
      "target_prefix": "/media/library/Movies",
      "status": "idle",
      "file_count": 85320,
      "last_scan_at": "2024-02-16T10:30:00Z"
    }
  ]
}
```

---

#### POST /api/sources
创建数据源

**请求**:
```json
{
  "name": "本地电影",
  "type": "local",
  "enabled": true,
  "config": {
    "mount_path": "/volume1/Media"
  },
  "mapping": {
    "source_prefix": "/volume1/Media/Movies",
    "target_prefix": "/media/library/Movies"
  }
}
```

---

#### POST /api/sources/:id/scan
触发扫描

**响应**:
```json
{
  "code": 200,
  "message": "Scan task created",
  "data": {
    "task_id": 123
  }
}
```

---

#### POST /api/sources/:id/test
测试连接

---

### 6.2 任务 API

#### GET /api/tasks
列出任务

#### GET /api/tasks/:id
获取任务详情

#### POST /api/tasks/:id/stop
停止任务

---

### 6.3 文件 API

#### GET /api/files
列出文件（分页）

#### GET /api/files/:id
获取文件详情

---

## 7. Docker 部署方案

### 7.1 后端 Dockerfile

```dockerfile
# backend/Dockerfile
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git gcc musl-dev sqlite-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a \
    -ldflags '-extldflags "-static"' \
    -o strmsync ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/strmsync .

RUN mkdir -p /app/config /app/data /app/logs

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD wget --spider -q http://localhost:3000/api/health || exit 1

CMD ["./strmsync"]
```

---

### 7.2 前端 Dockerfile

```dockerfile
# frontend/Dockerfile
FROM node:18-alpine AS builder

WORKDIR /build

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:alpine

COPY --from=builder /build/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

---

### 7.3 docker-compose.yml

```yaml
version: '3.8'

services:
  backend:
    build: ./backend
    container_name: strmsync-backend
    ports:
      - "3000:3000"
    volumes:
      - ./config:/app/config:ro
      - ./data:/app/data
      - ./logs:/app/logs
      - ${MEDIA_SOURCE_PATH}:/media:ro
      - ${STRM_OUTPUT_PATH}:/strm
    environment:
      - CONFIG_PATH=/app/config/config.yaml
      - SOURCES_PATH=/app/config/sources.yaml
      - LOG_LEVEL=info
      - TZ=Asia/Shanghai
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:3000/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  frontend:
    build: ./frontend
    container_name: strmsync-frontend
    ports:
      - "8080:80"
    environment:
      - API_URL=http://backend:3000
    depends_on:
      - backend
    restart: unless-stopped
```

---

## 总结

本文档详细描述了 STRMSync 项目的技术实施方案，包括：

1. **STRM 生成规则**：明确了 CloudDrive2（仅本地挂载）和 OpenList（HTTP/本地两种模式）的 STRM 生成逻辑
2. **数据库设计**：完整的表结构和 GORM 模型定义
3. **服务架构**：清晰的服务职责划分和接口设计
4. **适配器设计**：统一的适配器接口和三种数据源的具体实现
5. **实施步骤**：分阶段的开发计划（10 周）
6. **API 设计**：RESTful API 接口定义
7. **Docker 部署**：完整的容器化部署方案

---

**文档版本**: 1.0.0
**最后更新**: 2024-02-16
**作者**: STRMSync Team
