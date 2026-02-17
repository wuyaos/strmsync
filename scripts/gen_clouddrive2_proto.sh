#!/usr/bin/env bash
# CloudDrive2 Proto 代码生成脚本
set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 获取项目根目录
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="${ROOT_DIR}/backend/internal/clients/clouddrive2/proto"
OUT_DIR="${ROOT_DIR}/backend/internal/clients/clouddrive2/pb"

echo "========================================="
echo "CloudDrive2 gRPC 代码生成"
echo "========================================="
echo ""

# 检查 protoc
if ! command -v protoc >/dev/null 2>&1; then
  echo -e "${RED}错误: protoc 未安装${NC}"
  echo ""
  echo "请安装 Protocol Buffers 编译器："
  echo "  macOS:   brew install protobuf"
  echo "  Ubuntu:  sudo apt-get install protobuf-compiler"
  echo "  Windows: https://github.com/protocolbuffers/protobuf/releases"
  exit 1
fi

PROTOC_VERSION=$(protoc --version | awk '{print $2}')
echo -e "${GREEN}✓${NC} protoc 已安装: v${PROTOC_VERSION}"

# 检查 protoc-gen-go
if ! command -v protoc-gen-go >/dev/null 2>&1; then
  echo -e "${RED}错误: protoc-gen-go 未安装${NC}"
  echo ""
  echo "请安装 Go Protocol Buffers 插件："
  echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
  exit 1
fi

PROTOC_GEN_GO_VERSION=$(protoc-gen-go --version 2>&1 | grep -oP 'v\K[0-9.]+' || echo "unknown")
echo -e "${GREEN}✓${NC} protoc-gen-go 已安装: v${PROTOC_GEN_GO_VERSION}"

# 检查 protoc-gen-go-grpc
if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  echo -e "${RED}错误: protoc-gen-go-grpc 未安装${NC}"
  echo ""
  echo "请安装 Go gRPC 插件："
  echo "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
  exit 1
fi

PROTOC_GEN_GRPC_VERSION=$(protoc-gen-go-grpc --version 2>&1 | grep -oP 'protoc-gen-go-grpc \K[0-9.]+' || echo "unknown")
echo -e "${GREEN}✓${NC} protoc-gen-go-grpc 已安装: v${PROTOC_GEN_GRPC_VERSION}"

echo ""
echo "生成代码..."

# 创建输出目录
mkdir -p "${OUT_DIR}"

# 生成 Go 代码
if protoc -I "${PROTO_DIR}" \
  --go_out="${OUT_DIR}" --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" --go-grpc_opt=paths=source_relative \
  "${PROTO_DIR}/clouddrive2.proto"; then
  echo -e "${GREEN}✓${NC} 代码生成成功"
  echo ""
  echo "生成的文件："
  ls -lh "${OUT_DIR}"/*.go 2>/dev/null | awk '{print "  - " $9}' || echo "  无文件生成"
else
  echo -e "${RED}✗${NC} 代码生成失败"
  exit 1
fi

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}CloudDrive2 gRPC 代码生成完成！${NC}"
echo -e "${GREEN}=========================================${NC}"
