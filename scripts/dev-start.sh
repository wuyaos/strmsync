#!/bin/bash

# STRMSync 开发环境启动脚本
# 在 tests/ 目录下启动，自动加载 .env.test 配置

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
BACKEND_DIR="$PROJECT_ROOT/backend"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}  STRMSync 开发环境启动${NC}"
echo -e "${BLUE}======================================${NC}"

# 检查必要目录
if [ ! -d "$TESTS_DIR" ]; then
    echo -e "${RED}✗ 错误: tests/ 目录不存在${NC}"
    exit 1
fi

if [ ! -f "$TESTS_DIR/.env.test" ]; then
    echo -e "${RED}✗ 错误: tests/.env.test 配置文件不存在${NC}"
    exit 1
fi

# 读取测试环境配置并导出为环境变量
# set -a 使后续的变量赋值自动 export
set -a
source "$TESTS_DIR/.env.test"
set +a

BACKEND_PORT=${PORT:-3000}
FRONTEND_PORT=5676

echo -e "${GREEN}✓ 配置加载成功${NC}"
echo -e "  后端端口: ${BACKEND_PORT}"
echo -e "  前端端口: ${FRONTEND_PORT}"
echo -e "  工作目录: ${TESTS_DIR}"
echo -e "  数据库: ${DB_PATH}"

# 检查端口占用
check_port() {
    local port=$1
    local service=$2
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo -e "${YELLOW}⚠ 警告: ${service} 端口 ${port} 已被占用${NC}"
        echo -e "  使用 ${GREEN}./dev-restart.sh${NC} 强制重启，或手动停止进程"
        return 1
    fi
    return 0
}

# 检查端口
PORTS_OK=true
if ! check_port $BACKEND_PORT "后端"; then
    PORTS_OK=false
fi
if ! check_port $FRONTEND_PORT "前端"; then
    PORTS_OK=false
fi

if [ "$PORTS_OK" = false ]; then
    echo -e "${RED}✗ 端口检查失败，请先停止占用端口的进程${NC}"
    exit 1
fi

# 创建必要目录
mkdir -p "$TESTS_DIR/data"
mkdir -p "$TESTS_DIR/logs"

# 启动后端
echo -e "\n${BLUE}[1/2] 启动后端服务...${NC}"
cd "$TESTS_DIR"

# 启动后端到后台（环境变量已通过 set -a 导出）
cd "$BACKEND_DIR"
nohup go run ./cmd/server > "$TESTS_DIR/logs/backend.log" 2>&1 &
BACKEND_PID=$!
echo $BACKEND_PID > "$TESTS_DIR/backend.pid"

echo -e "${GREEN}✓ 后端启动中...${NC} PID: ${BACKEND_PID}"
echo -e "  日志: ${TESTS_DIR}/logs/backend.log"

# 等待后端启动
echo -n "  等待后端就绪"
for i in {1..30}; do
    if curl -s "http://localhost:${BACKEND_PORT}/api/health" >/dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ $i -eq 30 ]; then
        echo -e " ${RED}✗${NC}"
        echo -e "${RED}✗ 后端启动超时，查看日志: ${TESTS_DIR}/logs/backend.log${NC}"
        kill $BACKEND_PID 2>/dev/null || true
        exit 1
    fi
done

# 启动前端
echo -e "\n${BLUE}[2/2] 启动前端服务...${NC}"
cd "$FRONTEND_DIR"

# 设置前端代理目标为测试环境端口
export VITE_BACKEND_PORT=$BACKEND_PORT

# 启动前端到后台
nohup npm run dev > "$TESTS_DIR/logs/frontend.log" 2>&1 &
FRONTEND_PID=$!
echo $FRONTEND_PID > "$TESTS_DIR/frontend.pid"

echo -e "${GREEN}✓ 前端启动中...${NC} PID: ${FRONTEND_PID}"
echo -e "  日志: ${TESTS_DIR}/logs/frontend.log"

# 等待前端启动
echo -n "  等待前端就绪"
for i in {1..30}; do
    if lsof -Pi :$FRONTEND_PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ $i -eq 30 ]; then
        echo -e " ${RED}✗${NC}"
        echo -e "${RED}✗ 前端启动超时，查看日志: ${TESTS_DIR}/logs/frontend.log${NC}"
        kill $FRONTEND_PID 2>/dev/null || true
        kill $BACKEND_PID 2>/dev/null || true
        exit 1
    fi
done

echo -e "\n${GREEN}======================================${NC}"
echo -e "${GREEN}  ✓ 开发环境启动成功！${NC}"
echo -e "${GREEN}======================================${NC}"
echo -e "  🚀 前端: ${BLUE}http://localhost:${FRONTEND_PORT}${NC}"
echo -e "  🔧 后端: ${BLUE}http://localhost:${BACKEND_PORT}/api/health${NC}"
echo -e "  📁 工作目录: ${TESTS_DIR}"
echo -e "  📊 后端日志: tail -f ${TESTS_DIR}/logs/backend.log"
echo -e "  🎨 前端日志: tail -f ${TESTS_DIR}/logs/frontend.log"
echo -e "\n  停止服务: ${YELLOW}./dev-stop.sh${NC}"
echo -e "  重启服务: ${YELLOW}./dev-restart.sh${NC}"
echo -e "${GREEN}======================================${NC}"
