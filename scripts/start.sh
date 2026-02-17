#!/bin/bash
# STRMSync 项目启动脚本

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "==> STRMSync 项目启动"

# 检查环境文件
if [ ! -f "test.env" ]; then
    echo "错误: test.env 文件不存在"
    exit 1
fi

# 加载环境变量
set -a
source test.env
set +a

echo "==> 清理旧进程"
pkill -f strmsync || true
sleep 1

echo "==> 编译后端"
cd backend
go build -o ../strmsync ./cmd/server
cd ..

echo "==> 启动后端服务 (端口: ${SERVER_PORT:-6754})"
nohup ./strmsync > logs/strmsync.log 2>&1 &
echo $! > strmsync.pid
sleep 2

# 检查进程是否启动
if ps -p $(cat strmsync.pid) > /dev/null; then
    echo "==> 后端服务启动成功 (PID: $(cat strmsync.pid))"
    echo "==> API地址: http://${SERVER_HOST:-0.0.0.0}:${SERVER_PORT:-6754}"
    echo "==> 日志文件: logs/strmsync.log"
else
    echo "错误: 后端服务启动失败"
    cat logs/strmsync.log
    exit 1
fi

echo "==> 启动完成!"
echo ""
echo "使用以下命令查看日志:"
echo "  tail -f logs/strmsync.log"
echo ""
echo "使用以下命令停止服务:"
echo "  ./scripts/stop.sh"
