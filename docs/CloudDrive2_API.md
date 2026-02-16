# CloudDrive2 gRPC API 文档

## 基础信息

- **协议**: gRPC/HTTP2
- **数据格式**: Protocol Buffers
- **API 版本**: v0.9.24
- **默认地址**: http://localhost:19798
- **官方文档**: https://www.clouddrive2.com/api/CloudDrive2_gRPC_API_Guide.html

---

## 认证方式

CloudDrive2 使用 **JWT Bearer Token** 进行认证。

### 获取 Token

#### 方法 1: 用户凭据
```protobuf
rpc GetToken(GetTokenRequest) returns (TokenResponse);

message GetTokenRequest {
  string username = 1;
  string password = 2;
}

message TokenResponse {
  string token = 1;  // JWT Token
}
```

#### 方法 2: API Token
通过 Web UI 或 `CreateToken` RPC 创建专用 API Token，支持细粒度权限控制。

### 使用 Token

在 gRPC 请求的 metadata 中添加：

```
Authorization: Bearer <token>
```

### 公开方法（无需认证）

- `GetSystemInfo`
- `GetToken`
- `Login`
- `Register`
- `GetApiTokenInfo`

---

## 核心 API

### 1. 系统信息

#### GetSystemInfo（公开接口）

```protobuf
rpc GetSystemInfo(Empty) returns (SystemInfo);

message SystemInfo {
  string version = 1;
  string buildTime = 2;
  string platform = 3;
}
```

**用途**: 获取 CloudDrive2 版本信息，可用于健康检查。

---

### 2. 文件操作

#### List - 列出目录

```protobuf
rpc List(ListRequest) returns (ListResponse);

message ListRequest {
  string path = 1;           // 目录路径，例如 "/115/Movies"
  int32 page = 2;            // 页码，从 1 开始
  int32 perPage = 3;         // 每页数量，0 表示不分页
  bool refresh = 4;          // 是否刷新缓存
}

message ListResponse {
  bool success = 1;
  string errorMessage = 2;
  repeated FileInfo files = 3;
}

message FileInfo {
  string name = 1;           // 文件名
  int64 size = 2;            // 文件大小（字节）
  int64 modifyTime = 3;      // 修改时间（Unix 时间戳，秒）
  bool isDir = 4;            // 是否为目录
  string path = 5;           // 完整路径
  string hash = 6;           // 文件哈希（如果可用）
}
```

**示例**:
```json
{
  "path": "/115/Movies",
  "page": 1,
  "perPage": 100,
  "refresh": false
}
```

---

#### GetFileInfo - 获取文件信息

```protobuf
rpc GetFileInfo(GetFileInfoRequest) returns (FileInfo);

message GetFileInfoRequest {
  string path = 1;           // 文件或目录的完整路径
}
```

**响应**: 返回 `FileInfo` 对象（同上）。

---

#### CreateFolder - 创建目录

```protobuf
rpc CreateFolder(CreateFolderRequest) returns (Response);

message CreateFolderRequest {
  string path = 1;           // 新目录的完整路径
}

message Response {
  bool success = 1;
  string errorMessage = 2;
}
```

---

#### Rename - 重命名文件/目录

```protobuf
rpc Rename(RenameRequest) returns (Response);

message RenameRequest {
  string path = 1;           // 源路径
  string newName = 2;        // 新名称（不含路径）
}
```

---

#### Move - 移动文件/目录

```protobuf
rpc Move(MoveRequest) returns (Response);

message MoveRequest {
  string srcPath = 1;        // 源路径
  string dstPath = 2;        // 目标路径
}
```

---

#### Copy - 复制文件/目录

```protobuf
rpc Copy(CopyRequest) returns (Response);

message CopyRequest {
  string srcPath = 1;        // 源路径
  string dstPath = 2;        // 目标路径
}
```

---

#### Delete - 删除文件/目录

```protobuf
rpc Delete(DeleteRequest) returns (Response);

message DeleteRequest {
  string path = 1;           // 要删除的路径
}
```

---

### 3. 挂载管理

#### GetMountPoints - 获取挂载点

```protobuf
rpc GetMountPoints(Empty) returns (MountPointsResponse);

message MountPointsResponse {
  bool success = 1;
  string errorMessage = 2;
  repeated MountPoint mountPoints = 3;
}

message MountPoint {
  string cloudName = 1;      // 云盘名称（如 "115", "Aliyun"）
  string mountPath = 2;      // 本地挂载路径（如 "/mnt/clouddrive/115"）
  bool enabled = 3;          // 是否启用
  string status = 4;         // 状态（mounted/unmounted/error）
}
```

**用途**:
- 获取所有云盘的挂载信息
- 判断是否使用本地挂载模式或 API 模式

---

### 4. 云盘集成

#### 支持的云盘

- **115**: OAuth + QR Code
- **阿里云盘**: OAuth
- **百度网盘**: OAuth
- **OneDrive**: OAuth
- **Google Drive**: OAuth
- **S3 兼容存储**: Access Key + Secret Key
- **WebDAV**: 用户名 + 密码

#### 登录流程（以 115 为例）

1. **获取二维码**:
```protobuf
rpc APILogin115OpenQRCode(Empty) returns (stream QRCodeResponse);

message QRCodeResponse {
  string qrCodeUrl = 1;      // 二维码 URL
  string status = 2;         // 状态：pending/success/expired
}
```

2. **轮询状态**: 通过 stream 接收状态更新，直到登录成功或超时。

3. **完成登录**: status 为 "success" 时，云盘自动添加到 CloudDrive2。

---

### 5. 传输任务

#### ListTransferTasks - 列出传输任务

```protobuf
rpc ListTransferTasks(Empty) returns (TransferTasksResponse);

message TransferTasksResponse {
  bool success = 1;
  string errorMessage = 2;
  repeated TransferTask tasks = 3;
}

message TransferTask {
  string id = 1;             // 任务 ID
  string name = 2;           // 任务名称
  string type = 3;           // 类型：upload/download/copy
  string status = 4;         // 状态：running/paused/completed/failed
  int64 progress = 5;        // 进度（0-100）
  int64 speed = 6;           // 速度（字节/秒）
}
```

---

### 6. 缓存管理

#### SetDiskCache - 配置磁盘缓存

```protobuf
rpc SetDiskCache(SetDiskCacheRequest) returns (Response);

message SetDiskCacheRequest {
  string cloudName = 1;      // 云盘名称
  bool enabled = 2;          // 是否启用缓存
  int64 maxSize = 3;         // 最大缓存大小（字节）
}
```

**版本**: v0.9.18+

---

## 错误处理

### 响应格式

所有响应都包含：
- `success` (bool): 操作是否成功
- `errorMessage` (string): 错误信息（仅在失败时有值）

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 401 | Unauthorized（Token 无效或过期） |
| 403 | Forbidden（权限不足） |
| 404 | Not Found（路径不存在） |
| 429 | Too Many Requests（触发限流） |
| 500 | Internal Server Error |

---

## 限流策略

- **默认限制**: 100 QPS（每秒请求数）
- **可配置**: 通过 API 为每个云盘设置独立的 QPS 限制
- **建议**:
  - 扫描大量文件时使用批量接口
  - 避免频繁调用 `refresh=true`

---

## 使用建议

### 1. 判断使用模式

```go
// 伪代码
mountPoints := GetMountPoints()
for _, mp := range mountPoints {
    if mp.mountPath != "" && mp.status == "mounted" {
        // 使用本地挂载模式（直接访问文件系统）
        useLocalMount(mp.mountPath)
    } else {
        // 使用 API 模式（通过 gRPC 操作）
        useAPIMode(mp.cloudName)
    }
}
```

### 2. 分页策略

- **小目录**（< 1000 文件）: `perPage=0`（不分页）
- **大目录**（> 1000 文件）: `perPage=100`，逐页读取

### 3. 缓存策略

- **首次扫描**: `refresh=false`（使用缓存）
- **强制刷新**: `refresh=true`（刷新缓存，适用于检测新文件）
- **定时刷新**: 建议每 5 分钟刷新一次

### 4. 路径格式

CloudDrive2 路径格式：`/<云盘名称>/<相对路径>`

示例：
- `/115/Movies/Action/Movie.mkv`
- `/Aliyun/TVShows/Show/S01E01.mkv`

---

## 项目中的应用

### STRMSync 集成方案

#### 本地挂载模式（推荐）

```yaml
source:
  type: clouddrive2
  mode: mount
  mount_path: /mnt/clouddrive
  api_url: http://localhost:19798  # 仅用于健康检查
  api_token: xxx

strm_content: |
  /mnt/clouddrive/115/Movies/Movie.mkv
```

**优点**:
- 性能最佳（直接文件系统访问）
- 无需 API 调用
- Emby/Plex/Jellyfin 可直接播放

#### API 模式（备选）

```yaml
source:
  type: clouddrive2
  mode: api
  api_url: http://localhost:19798
  api_token: xxx

strm_content: |
  http://localhost:19798/api/v1/download?path=/115/Movies/Movie.mkv&token=xxx
```

**优点**:
- 无需挂载
- 跨网络访问
- 适用于 Docker 环境

**缺点**:
- 性能较低
- 需要暴露 CloudDrive2 API

---

## 开发注意事项

### 1. Go gRPC 客户端

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
)

// 创建连接
conn, err := grpc.Dial("localhost:19798", grpc.WithInsecure())
defer conn.Close()

client := pb.NewCloudDriveFileSrvClient(conn)

// 添加认证
ctx := metadata.AppendToOutgoingContext(context.Background(),
    "authorization", "Bearer "+token)

// 调用接口
resp, err := client.List(ctx, &pb.ListRequest{
    Path: "/115/Movies",
    Page: 1,
    PerPage: 100,
})
```

### 2. 错误重试

```go
func RetryOnError(fn func() error) error {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }

        // 处理特定错误
        if isRateLimitError(err) {
            time.Sleep(time.Second * 5)
            continue
        }

        if isTokenExpiredError(err) {
            refreshToken()
            continue
        }

        return err  // 其他错误直接返回
    }
    return errors.New("max retries exceeded")
}
```

### 3. 并发限制

```go
// 使用 Goroutine 池限制并发
semaphore := make(chan struct{}, 10)  // 限制 10 并发

for _, path := range paths {
    semaphore <- struct{}{}  // 获取信号量
    go func(p string) {
        defer func() { <-semaphore }()  // 释放信号量
        processFile(p)
    }(path)
}
```

---

## 参考资源

- **官方文档**: https://www.clouddrive2.com/api/
- **Markdown 源文件**:
  - 中文: `CloudDrive2_gRPC_API_Guide_zh-CN.md`
  - 英文: `CloudDrive2_gRPC_API_Guide.md`
- **许可证**: AGPL-3.0

---

**文档版本**: v0.9.24
**最后更新**: 2024-02-16
**作者**: STRMSync Team
