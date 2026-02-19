#!/bin/bash

# STRMSync 开发环境停止脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TESTS_DIR="$PROJECT_ROOT/tests"

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}  STRMSync 开发环境停止${NC}"
echo -e "${BLUE}======================================${NC}"

# 停止后端
if [ -f "$TESTS_DIR/backend.pid" ]; then
    BACKEND_PID=$(cat "$TESTS_DIR/backend.pid")
    if ps -p $BACKEND_PID > /dev/null 2>&1; then
        echo -e "${YELLOW}停止后端服务...${NC} PID: $BACKEND_PID"
        kill $BACKEND_PID 2>/dev/null || true
        # 等待进程退出
        for i in {1..10}; do
            if ! ps -p $BACKEND_PID > /dev/null 2>&1; then
                break
            fi
            sleep 0.5
        done
        # 如果还没退出，强制kill
        if ps -p $BACKEND_PID > /dev/null 2>&1; then
            echo -e "${YELLOW}  强制停止后端...${NC}"
            kill -9 $BACKEND_PID 2>/dev/null || true
        fi
        echo -e "${GREEN}✓ 后端已停止${NC}"
    else
        echo -e "${YELLOW}⚠ 后端进程不存在 (PID: $BACKEND_PID)${NC}"
    fi
    rm -f "$TESTS_DIR/backend.pid"
else
    echo -e "${YELLOW}⚠ 未找到后端 PID 文件${NC}"
fi

# 停止前端
if [ -f "$TESTS_DIR/frontend.pid" ]; then
    FRONTEND_PID=$(cat "$TESTS_DIR/frontend.pid")
    if ps -p $FRONTEND_PID > /dev/null 2>&1; then
        echo -e "${YELLOW}停止前端服务...${NC} PID: $FRONTEND_PID"
        kill $FRONTEND_PID 2>/dev/null || true
        # 等待进程退出
        for i in {1..10}; do
            if ! ps -p $FRONTEND_PID > /dev/null 2>&1; then
                break
            fi
            sleep 0.5
        done
        # 如果还没退出，强制kill
        if ps -p $FRONTEND_PID > /dev/null 2>&1; then
            echo -e "${YELLOW}  强制停止前端...${NC}"
            kill -9 $FRONTEND_PID 2>/dev/null || true
        fi
        echo -e "${GREEN}✓ 前端已停止${NC}"
    else
        echo -e "${YELLOW}⚠ 前端进程不存在 (PID: $FRONTEND_PID)${NC}"
    fi
    rm -f "$TESTS_DIR/frontend.pid"
else
    echo -e "${YELLOW}⚠ 未找到前端 PID 文件${NC}"
fi

# 额外清理：查找并停止可能遗留的进程
echo -e "\n${YELLOW}检查遗留进程...${NC}"

# 读取测试环境配置获取端口
if [ -f "$TESTS_DIR/.env.test" ]; then
    source "$TESTS_DIR/.env.test"
    BACKEND_PORT=${PORT:-3000}
    FRONTEND_PORT=5676

    # 检查后端端口
    BACKEND_PIDS=$(lsof -ti :$BACKEND_PORT 2>/dev/null || true)
    if [ -n "$BACKEND_PIDS" ]; then
        echo -e "${YELLOW}  发现后端端口 ${BACKEND_PORT} 占用: $BACKEND_PIDS${NC}"
        for pid in $BACKEND_PIDS; do
            kill -9 $pid 2>/dev/null || true
        done
        echo -e "${GREEN}  ✓ 已清理${NC}"
    fi

    # 检查前端端口
    FRONTEND_PIDS=$(lsof -ti :$FRONTEND_PORT 2>/dev/null || true)
    if [ -n "$FRONTEND_PIDS" ]; then
        echo -e "${YELLOW}  发现前端端口 ${FRONTEND_PORT} 占用: $FRONTEND_PIDS${NC}"
        for pid in $FRONTEND_PIDS; do
            kill -9 $pid 2>/dev/null || true
        done
        echo -e "${GREEN}  ✓ 已清理${NC}"
    fi
fi

# 查找 strmsync 相关进程
STRMSYNC_PIDS=$(pgrep -f "strmsync|cmd/server" 2>/dev/null || true)
if [ -n "$STRMSYNC_PIDS" ]; then
    echo -e "${YELLOW}  发现 strmsync 进程: $STRMSYNC_PIDS${NC}"
    for pid in $STRMSYNC_PIDS; do
        kill -9 $pid 2>/dev/null || true
    done
    echo -e "${GREEN}  ✓ 已清理${NC}"
fi

echo -e "\n${GREEN}======================================${NC}"
echo -e "${GREEN}  ✓ 开发环境已完全停止${NC}"
echo -e "${GREEN}======================================${NC}"
