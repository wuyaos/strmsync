#!/bin/bash

# STRMSync 开发环境重启脚本
# 先停止所有进程，再启动

set -e

# 颜色定义
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}  STRMSync 开发环境重启${NC}"
echo -e "${BLUE}======================================${NC}"

# 停止服务
"$SCRIPT_DIR/dev-stop.sh"

# 等待一秒确保进程完全退出
sleep 1

# 启动服务
"$SCRIPT_DIR/dev-start.sh"
