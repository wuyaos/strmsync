#!/bin/bash

# STRMSync API 测试脚本

BASE_URL="http://localhost:3000/api"

echo "========================================="
echo "  STRMSync API 功能测试"
echo "========================================="
echo ""

# 1. 测试健康检查
echo "1️⃣  测试健康检查..."
curl -s "$BASE_URL/health" | jq '.'
echo ""

# 2. 创建本地数据源
echo "2️⃣  创建本地数据源..."
LOCAL_SOURCE=$(curl -s -X POST "$BASE_URL/sources" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "本地测试媒体库",
    "type": "local",
    "enabled": true,
    "source_prefix": "/mnt/c/Users/wff19/Desktop/strm/test/media",
    "target_prefix": "/mnt/c/Users/wff19/Desktop/strm/test/output"
  }')

SOURCE_ID=$(echo $LOCAL_SOURCE | jq -r '.id')
echo "✅ 数据源ID: $SOURCE_ID"
echo ""

# 3. 获取数据源列表
echo "3️⃣  获取数据源列表..."
curl -s "$BASE_URL/sources" | jq '.'
echo ""

# 4. 触发扫描
echo "4️⃣  触发扫描任务..."
curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/scan" | jq '.'
echo ""

# 等待扫描完成
echo "⏳ 等待5秒..."
sleep 5

# 5. 查看数据源状态
echo "5️⃣  查看数据源状态..."
curl -s "$BASE_URL/sources/$SOURCE_ID" | jq '.'
echo ""

# 6. 启动文件监控
echo "6️⃣  启动文件监控..."
curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/watch/start" | jq '.'
echo ""

# 7. 查询监控状态
echo "7️⃣  查询监控状态..."
curl -s "$BASE_URL/sources/$SOURCE_ID/watch/status" | jq '.'
echo ""

# 8. 同步元数据
echo "8️⃣  同步元数据..."
curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/metadata/sync" | jq '.'
echo ""

# 9. 触发媒体库通知
echo "9️⃣  触发媒体库通知..."
curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/notify/refresh" | jq '.'
echo ""

echo "========================================="
echo "  测试完成！"
echo "========================================="
