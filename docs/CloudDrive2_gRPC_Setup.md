# CloudDrive2 gRPC 集成 - 安装指南

## 概述

CloudDrive2 使用 gRPC 协议，需要生成 Protocol Buffers 代码才能使用。

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

## 生成 gRPC 代码

### 方法 1: 使用 Make（推荐）

```bash
make gen-clouddrive2-proto
```

### 方法 2: 使用脚本

```bash
./scripts/gen_clouddrive2_proto.sh
```

### 方法 3: 手动生成

```bash
cd backend
mkdir -p internal/clients/clouddrive2/pb

protoc -I internal/clients/clouddrive2/proto \
  --go_out=internal/clients/clouddrive2/pb --go_opt=paths=source_relative \
  --go-grpc_out=internal/clients/clouddrive2/pb --go-grpc_opt=paths=source_relative \
  internal/clients/clouddrive2/proto/clouddrive2.proto
```

## 验证生成

生成成功后应该看到以下文件：

```
backend/internal/clients/clouddrive2/pb/
├── clouddrive2.pb.go         # Protocol Buffers 定义
└── clouddrive2_grpc.pb.go    # gRPC 服务定义
```

## 更新依赖

```bash
cd backend
go mod tidy
```

## 常见问题

### Q: protoc 找不到 google/protobuf/empty.proto

**A:** protoc 需要能找到标准 proto 文件。确保 protoc 正确安装。

### Q: Go 版本兼容性问题

**A:** 项目使用 Go 1.19，确保安装的插件版本兼容：
- google.golang.org/protobuf@v1.28.1
- google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0

### Q: 权限问题

**A:** 如果没有 sudo 权限，可以手动下载 protoc 二进制文件并放到 PATH 中。

## 下一步

生成完成后，可以运行 CloudDrive2 连接测试：

```bash
curl -X POST http://localhost:6754/api/servers/data/1/test
```

## 参考资料

- [Protocol Buffers](https://protobuf.dev/)
- [gRPC Go](https://grpc.io/docs/languages/go/)
- [CloudDrive2 API 文档](../docs/CloudDrive2_API.md)
