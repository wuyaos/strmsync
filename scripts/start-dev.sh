#!/bin/bash

# 开发环境启动脚本
# 设置必要的环境变量并启动STRMSync服务

export ENCRYPTION_KEY="test-key-32-bytes-long-for-dev-only"
export GIN_MODE="debug"
export PORT="18888"

cd "$(dirname "$0")/.." || exit 1

# 停止旧服务
if [ -f server.pid ]; then
    OLD_PID=$(cat server.pid)
    if ps -p "$OLD_PID" > /dev/null 2>&1; then
        echo "停止旧服务 (PID: $OLD_PID)..."
        kill "$OLD_PID" 2>/dev/null || true
        sleep 1
    fi
    rm -f server.pid
fi

# 启动新服务
echo "启动STRMSync服务..."
nohup ./build/strmsync > logs/test-server.log 2>&1 &
NEW_PID=$!
echo "$NEW_PID" > server.pid

echo "服务已启动 (PID: $NEW_PID)"
echo "等待服务就绪..."
sleep 3

# 健康检查
if curl -s http://127.0.0.1:18888/api/health > /dev/null; then
    echo "✓ 服务启动成功，健康检查通过"
else
    echo "✗ 服务健康检查失败，请查看日志: logs/test-server.log"
    exit 1
fi
