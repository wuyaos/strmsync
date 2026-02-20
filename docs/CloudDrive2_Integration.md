# CloudDrive2 gRPC 集成完整指南

> 本文档整合了 CloudDrive2 的集成说明和开发环境设置指南

**最后更新**: 2026-02-20

---

## 📋 目录

- [概述](#概述)
- [版本信息](#版本信息)
- [前置要求](#前置要求)
- [环境设置](#环境设置)
- [客户端特性](#客户端特性)
- [代码示例](#代码示例)
- [配置说明](#配置说明)
- [常见问题](#常见问题)
- [开发指南](#开发指南)

---

## 概述

本项目已集成 CloudDrive2 gRPC 客户端（版本 0.9.24），用于与 CloudDrive2 服务进行通信。CloudDrive2 使用 gRPC 协议，需要生成 Protocol Buffers 代码才能使用。

---

## 版本信息

- **CloudDrive2 Proto**: 0.9.24
- **gRPC**: v1.79.1
- **protobuf**: v1.36.10
- **协议**: h2c (HTTP/2 cleartext)

---

## 前置要求

### 1. 安装 Protocol Buffers 编译器 (protoc)

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y protobuf-compiler
```

**macOS:**
```bash
brew install protobuf
```

**Windows:**
下载并安装: https://github.com/protocolbuffers/protobuf/releases

验证安装：
```bash
protoc --version
# 输出：libprotoc 3.x.x 或更高
```

### 2. 安装 Go 插件

```bash
# protoc-gen-go (Protocol Buffers 生成器)
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1

# protoc-gen-go-grpc (gRPC 生成器)
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
```

确保 `$GOPATH/bin` 在 PATH 中：
```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

验证安装：
```bash
which protoc-gen-go
which protoc-gen-go-grpc
```

---

## 环境设置

### 生成 gRPC 代码

**方法 1: 使用 Make（推荐）**
```bash
make gen-clouddrive2-proto
```

**方法 2: 使用脚本**
```bash
./scripts/gen_clouddrive2_proto.sh
```

**方法 3: 手动生成**
```bash
cd backend
mkdir -p filesystem/clouddrive2_proto

protoc -I filesystem/clouddrive2_proto \
  --go_out=filesystem/clouddrive2_proto --go_opt=paths=source_relative \
  --go-grpc_out=filesystem/clouddrive2_proto --go-grpc_opt=paths=source_relative \
  filesystem/clouddrive2_proto/clouddrive2.proto
```

### 验证生成

生成成功后应该看到以下文件：

```
backend/filesystem/clouddrive2_proto/
├── clouddrive2.pb.go         # Protocol Buffers 定义
└── clouddrive2_grpc.pb.go    # gRPC 服务定义
```

### 更新依赖

```bash
cd backend
go mod tidy
```

---

## 客户端特性

### 核心功能
- ✅ 连接管理（自动重连、连接复用）
- ✅ 认证支持（Bearer Token in metadata）
- ✅ 流式API支持（Server Streaming）
- ✅ Functional Options模式
- ✅ 完整错误处理

### 支持的API

#### 公开接口（无需认证）
- `GetSystemInfo()` - 获取系统信息和健康状态
- `GetToken()` - 通过用户名密码获取JWT Token
- `Login()` - 登录到CloudFS服务器

#### 认证接口（需要Token）
- `GetMountPoints()` - 获取所有挂载点
- `GetSubFiles()` - 列出目录内容（流式）
- `FindFileByPath()` - 查找文件信息
- `CreateFolder()` - 创建目录
- `RenameFile()` - 重命名文件
- `MoveFile()` - 移动文件
- `DeleteFile()` - 删除文件

更多API请参考：`backend/filesystem/clouddrive2.go`

---

## 代码示例

### 基础连接

```go
import (
    "context"
    "time"
    "github.com/strmsync/strmsync/filesystem"
)

func main() {
    client := filesystem.NewCloudDrive2Client(
        "127.0.0.1:19798",  // gRPC地址
        "your_jwt_token",    // Token（可为空）
        filesystem.WithTimeout(10*time.Second),
    )

    ctx := context.Background()

    // 测试连接
    info, err := client.GetSystemInfo(ctx)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("系统就绪: %v\n", info.GetSystemReady())
}
```

### 列出文件

```go
func listFiles(client *clouddrive2.Client, path string) error {
    ctx := context.Background()

    files, err := client.GetSubFiles(ctx, path, false)
    if err != nil {
        return err
    }

    for _, file := range files {
        fmt.Printf("%s (%d bytes)\n", file.GetName(), file.GetSize())
    }

    return nil
}
```

---

## 配置说明

### CloudDrive2 服务端配置

- **gRPC端口**: 默认 `19798`
- **协议要求**: h2c (HTTP/2 cleartext)
- **认证方式**: Bearer Token (JWT)

### 客户端配置

```go
client := clouddrive2.NewClient(
    target,  // "host:port"格式
    token,   // JWT token（可选）
    clouddrive2.WithTimeout(10*time.Second),  // 超时设置
)
```

---

## 常见问题

### Q: 405 Method Not Allowed 错误

**可能原因**：
1. 端口错误（连接到HTTP UI端口而非gRPC端口）
2. 反向代理未正确配置gRPC/h2c转发
3. 协议不匹配（服务端要求TLS但客户端使用h2c）

**解决方案**：
1. 确认CloudDrive2的gRPC端口（默认19798）
2. 使用测试程序直连服务端，绕过代理排查
3. 检查反向代理配置（nginx需要grpc_pass）

### Q: 认证失败

**可能原因**：
1. Token无效或已过期
2. Token格式错误
3. 未登录CloudDrive2

**解决方案**：
1. 使用`GetToken()`重新获取JWT Token
2. 确保Token格式为标准JWT
3. 先调用`Login()`登录系统

### Q: SystemReady = false

**说明**：系统正在初始化或维护中，需要等待系统就绪后再调用其他API。

---

## 开发指南

### 重新生成Proto代码

如果需要更新proto定义：

```bash
# 1. 更新proto文件（脚本会自动下载最新版本）
./scripts/gen_clouddrive2_proto.sh

# 2. 确保go_package选项存在
# option go_package = "github.com/strmsync/strmsync/internal/clients/clouddrive2/pb;pb";

# 3. 重新编译
go build ./...
```

### 更新 API 文档

下载最新的 CloudDrive2 API 文档：

```bash
./scripts/update_clouddrive2_api.sh
```

### 添加新的API方法

1. 在`client.go`中添加新方法
2. 使用`withAuth()`包装context
3. 调用`c.svc.MethodName()`
4. 处理错误和返回值

示例：

```go
func (c *Client) NewMethod(ctx context.Context, param string) (*pb.Result, error) {
    if err := c.Connect(ctx); err != nil {
        return nil, err
    }

    ctx, cancel := c.withAuth(ctx)
    defer cancel()

    resp, err := c.svc.NewMethod(ctx, &pb.Request{Param: param})
    if err != nil {
        return nil, fmt.Errorf("clouddrive2: NewMethod failed: %w", err)
    }

    return resp, nil
}
```

---

## 相关文档

- [CloudDrive2 官方文档](https://www.clouddrive2.com)
- [CloudDrive2 API 参考](CloudDrive2_API.md)
- [gRPC Go 快速开始](https://grpc.io/docs/languages/go/quickstart/)
- [Protocol Buffers](https://protobuf.dev/)

---

**文档版本**: 2.0.0
**最后更新**: 2026-02-20
**作者**: STRMSync Team
