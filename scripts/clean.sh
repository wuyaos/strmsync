#!/bin/bash
# STRMSync 项目清理脚本

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/.."
cd "$PROJECT_ROOT"

echo "==> 清理STRMSync项目"

# 停止服务
./scripts/stop.sh

echo "==> 清理旧数据库"
rm -f data/test.db data/strmsync.db

echo "==> 清理日志文件"
rm -f logs/*.log logs/test-server.log

echo "==> 清理编译产物"
rm -f strmsync backend/strmsync

echo "==> 清理临时文件"
rm -f /tmp/backend-*.log /tmp/backend-*.pid
rm -f strmsync.pid

echo "==> 清理node_modules和构建产物"
rm -rf frontend/node_modules
rm -rf frontend/dist

echo "==> 清理完成!"
echo ""
echo "重新开始开发:"
echo "  1. cd frontend && npm install"
echo "  2. ./scripts/start.sh"
