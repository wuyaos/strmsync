#!/bin/bash
# STRMSync 停止脚本

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/.."
cd "$PROJECT_ROOT"

echo "==> 停止STRMSync服务"

if [ -f "strmsync.pid" ]; then
    PID=$(cat strmsync.pid)
    if ps -p $PID > /dev/null; then
        echo "==> 停止进程 (PID: $PID)"
        kill $PID
        sleep 2
        if ps -p $PID > /dev/null; then
            echo "==> 强制停止进程"
            kill -9 $PID
        fi
    fi
    rm -f strmsync.pid
fi

# 清理可能残留的进程
pkill -f strmsync || true

echo "==> 服务已停止"
