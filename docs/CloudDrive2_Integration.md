# CloudDrive2 gRPC 集成说明

## 概述

本项目已集成CloudDrive2 gRPC客户端（版本 0.9.24），用于与CloudDrive2服务进行通信。

## 版本信息

- **CloudDrive2 Proto**: 0.9.24
- **gRPC**: v1.79.1
- **protobuf**: v1.36.10
- **协议**: h2c (HTTP/2 cleartext)

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

更多API请参考：`backend/internal/clients/clouddrive2/client.go`

## 测试工具

### 连接测试程序

位置：`backend/cmd/test_clouddrive2/main.go`

#### 使用方法

```bash
# 基础连接测试（只测试GetSystemInfo，无需认证）
CD2_HOST=127.0.0.1:19798 go run backend/cmd/test_clouddrive2/main.go

# 完整测试（包括认证API）
CD2_HOST=127.0.0.1:19798 CD2_TOKEN=your_jwt_token go run backend/cmd/test_clouddrive2/main.go
```

#### 测试内容

1. **GetSystemInfo** - 验证gRPC连接和系统状态
   - 检查是否登录
   - 检查系统就绪状态
   - 检查错误标志

2. **GetMountPoints** - 验证认证和挂载点查询
   - 列出所有挂载点
   - 显示挂载状态

3. **GetSubFiles** - 验证流式API
   - 列出目录内容
   - 显示文件信息

## 代码示例

### 基础连接

```go
import (
    "context"
    "time"
    "github.com/strmsync/strmsync/internal/clients/clouddrive2"
)

func main() {
    client := clouddrive2.NewClient(
        "127.0.0.1:19798",  // gRPC地址
        "your_jwt_token",    // Token（可为空）
        clouddrive2.WithTimeout(10*time.Second),
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

## 开发指南

### 重新生成Proto代码

如果需要更新proto定义：

```bash
# 1. 更新proto文件
cp new_clouddrive2.proto backend/internal/clients/clouddrive2/proto/

# 2. 确保go_package选项存在
# option go_package = "github.com/strmsync/strmsync/internal/clients/clouddrive2/pb;pb";

# 3. 运行生成脚本
bash scripts/gen_clouddrive2_proto.sh

# 4. 重新编译
go build ./...
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

## 相关文档

- [CloudDrive2 官方文档](https://www.clouddrive2.com)
- [gRPC Go 快速开始](https://grpc.io/docs/languages/go/quickstart/)
- [Protocol Buffers](https://protobuf.dev/)
