#!/bin/bash

# STRMSync 测试启动脚本

echo "========================================="
echo "  STRMSync 测试环境启动"
echo "========================================="
echo ""

# 检查构建产物
if [ ! -f "./build/strmsync" ]; then
    echo "❌ 构建产物不存在，正在构建..."
    cd backend
    go build -o ../build/strmsync ./cmd/server
    cd ..
    echo "✅ 构建完成"
fi

# 创建必要的目录
mkdir -p data logs

# 加载测试环境变量
export $(cat .env.test | grep -v '^#' | xargs)

echo "📋 测试配置："
echo "   - 端口: $PORT"
echo "   - 数据库: $DB_PATH"
echo "   - 日志级别: $LOG_LEVEL"
echo "   - Emby服务器: $NOTIFIER_BASE_URL"
echo ""

echo "🚀 启动服务..."
./build/strmsync
