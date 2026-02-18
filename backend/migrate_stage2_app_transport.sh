#!/bin/bash
# Stage 2: 迁移应用层和传输层
# - service/ → internal/app/service/
# - handler/ → internal/transport/http/

set -e

echo "===== Stage 2: 应用层和传输层迁移 ====="

# 创建目标目录
echo "创建目标目录..."
mkdir -p internal/app/service
mkdir -p internal/transport/http

# 迁移 service/ → internal/app/service/
echo "迁移 service/ 到 internal/app/service/..."
if [ -d "service" ]; then
    mv service/*.go internal/app/service/ 2>/dev/null || true
    # 删除空目录
    rmdir service 2>/dev/null || true
fi

# 迁移 handler/ → internal/transport/http/
echo "迁移 handler/ 到 internal/transport/http/..."
if [ -d "handler" ]; then
    mv handler/*.go internal/transport/http/ 2>/dev/null || true
    # 删除空目录
    rmdir handler 2>/dev/null || true
fi

echo "===== Stage 2 文件移动完成 ====="
echo "接下来需要："
echo "1. 更新 internal/app/service/*.go 中的 package 声明为 'package service'"
echo "2. 更新 internal/transport/http/*.go 中的 package 声明为 'package http'"
echo "3. 更新 main.go 中的导入路径"
echo "4. 更新所有引用 service 和 handlers 的文件"
