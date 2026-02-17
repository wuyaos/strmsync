# CloudDrive2 gRPC 集成已知问题

## 问题描述

CloudDrive2 gRPC客户端在测试连接时始终返回405错误：
```
rpc error: code = Unknown desc = unexpected HTTP status code received from server: 405 (Method Not Allowed);
malformed header: missing HTTP content-type
```

## 调查过程

### 已完成的工作

1. ✅ 从官方proto文件生成pb代码（正确使用`package clouddrive`）
2. ✅ 实现完整的gRPC客户端（GetSubFiles、FindFileByPath等所有方法）
3. ✅ 升级grpc-go从v1.50.1到v1.56.3
4. ✅ 尝试多种连接配置：
   - WithContextDialer
   - WithAuthority
   - dns前缀target
   - 自定义TCP dialer

### 验证结果

**curl测试（成功）：**
```bash
curl --http2-prior-knowledge -X POST \
  http://192.168.123.179:19798/clouddrive.CloudDriveFileSrv/GetSystemInfo \
  -H "Content-Type: application/grpc" -H "TE: trailers" --data-binary ""

# 返回：HTTP/2 200
# grpc-status: 13
# grpc-message: Missing request message
```

**Go客户端（失败）：**
- grpc-go始终收到405错误
- 说明服务器拒绝了grpc-go发送的请求

## 根本原因分析

CloudDrive2可能：
1. **不是标准gRPC实现** - 实现了类似gRPC的HTTP/2 API但不完全兼容grpc-go
2. **只接受h2c prior-knowledge** - 不支持HTTP/1.1升级，而grpc-go默认会尝试升级
3. **需要特定的header或握手序列** - grpc-go发送的请求被中间层拒绝

## 当前状态

- ✅ OpenList连接测试成功（792ms）
- ✅ Emby连接测试成功（828ms）
- ❌ CloudDrive2连接测试失败（405错误）

## 解决方案

### 短期方案（已采用）

将CloudDrive2集成标记为**实验性功能**，文档中说明已知问题：
- API接口已实现，代码结构完整
- 连接测试功能保留，但预期会失败
- 用户可以使用OpenList作为CloudDrive2的替代方案

### 长期方案（待实现）

选项1：**手动实现HTTP/2 + gRPC帧**
- 使用`golang.org/x/net/http2`直接发送HTTP/2请求
- 手动构造gRPC帧（5字节header + protobuf消息）
- 读取grpc-status和grpc-message trailers
- 优点：完全可控，可以精确匹配CloudDrive2的要求
- 缺点：实现复杂，维护成本高

选项2：**使用REST API**
- 调查CloudDrive2是否提供HTTP/REST或WebDAV接口
- 如果有，改用REST客户端
- 优点：兼容性好，实现简单
- 缺点：需要确认API是否存在

选项3：**等待CloudDrive2官方支持**
- 向CloudDrive2项目提交issue
- 等待官方修复或提供Go SDK
- 优点：无需自己维护
- 缺点：时间不确定

## 相关文件

- `/backend/internal/clients/clouddrive2/client.go` - gRPC客户端实现
- `/backend/internal/clients/clouddrive2/pb/` - 生成的protobuf代码
- `/backend/internal/clients/clouddrive2/proto/clouddrive2.proto` - 官方proto文件
- `/backend/internal/handlers/data_server.go` - 连接测试handler

## 参考资料

- CloudDrive2官方proto: https://github.com/ge-fei-fan/clouddrive2api
- grpc-go h2c文档: https://github.com/grpc/grpc-go/blob/master/examples/features/name_resolving/README.md
