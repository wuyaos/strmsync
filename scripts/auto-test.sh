#!/bin/bash

# STRMSync 自动化测试脚本
BASE_URL="http://localhost:18888/api"

echo "========================================="
echo "  STRMSync 自动化功能测试"
echo "========================================="
echo ""

# 1. 测试健康检查
echo "1️⃣  测试健康检查 API..."
HEALTH=$(curl -s "$BASE_URL/health")
echo "$HEALTH"
if [ $? -eq 0 ]; then
    echo "✅ 健康检查通过"
else
    echo "❌ 健康检查失败"
    exit 1
fi
echo ""

# 2. 获取初始数据源列表
echo "2️⃣  获取数据源列表（应为空）..."
curl -s "$BASE_URL/sources"
echo ""
echo ""

# 3. 创建本地数据源
echo "3️⃣  创建本地测试数据源..."
CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/sources" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "本地测试媒体库",
    "type": "local",
    "enabled": true,
    "source_prefix": "/mnt/c/Users/wff19/Desktop/strm/test/media",
    "target_prefix": "/mnt/c/Users/wff19/Desktop/strm/test/output"
  }')

echo "$CREATE_RESPONSE"
SOURCE_ID=$(echo "$CREATE_RESPONSE" | grep -oP '"id":\K\d+' | head -1)

if [ -z "$SOURCE_ID" ]; then
    echo "❌ 创建数据源失败，无法获取ID"
    exit 1
fi

echo "✅ 数据源创建成功，ID: $SOURCE_ID"
echo ""

# 4. 获取数据源详情
echo "4️⃣  获取数据源详情..."
curl -s "$BASE_URL/sources/$SOURCE_ID"
echo ""
echo ""

# 5. 测试扫描功能
echo "5️⃣  触发文件扫描..."
SCAN_RESPONSE=$(curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/scan")
echo "$SCAN_RESPONSE"
echo "✅ 扫描任务已提交"
echo ""

# 等待扫描完成
echo "⏳ 等待10秒让扫描完成..."
sleep 10
echo ""

# 6. 再次查看数据源状态
echo "6️⃣  查看扫描后的数据源状态..."
curl -s "$BASE_URL/sources/$SOURCE_ID"
echo ""
echo ""

# 7. 检查生成的STRM文件
echo "7️⃣  检查生成的STRM文件..."
if [ -d "/mnt/c/Users/wff19/Desktop/strm/test/output" ]; then
    STRM_COUNT=$(find /mnt/c/Users/wff19/Desktop/strm/test/output -name "*.strm" | wc -l)
    echo "找到 $STRM_COUNT 个STRM文件"
    if [ $STRM_COUNT -gt 0 ]; then
        echo "✅ STRM文件生成成功"
        echo "示例文件："
        find /mnt/c/Users/wff19/Desktop/strm/test/output -name "*.strm" | head -3
    else
        echo "⚠️  未找到STRM文件"
    fi
else
    echo "❌ 输出目录不存在"
fi
echo ""

# 8. 测试文件监控
echo "8️⃣  启动文件监控..."
WATCH_RESPONSE=$(curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/watch/start")
echo "$WATCH_RESPONSE"
echo ""

# 9. 查询监控状态
echo "9️⃣  查询监控状态..."
curl -s "$BASE_URL/sources/$SOURCE_ID/watch/status"
echo ""
echo ""

# 10. 测试元数据同步
echo "🔟 测试元数据同步..."
METADATA_RESPONSE=$(curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/metadata/sync")
echo "$METADATA_RESPONSE"
echo ""
echo ""

# 11. 停止监控
echo "1️⃣1️⃣  停止文件监控..."
STOP_RESPONSE=$(curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/watch/stop")
echo "$STOP_RESPONSE"
echo ""
echo ""

# 12. 最终状态检查
echo "1️⃣2️⃣  最终状态检查..."
curl -s "$BASE_URL/sources"
echo ""
echo ""

echo "========================================="
echo "  ✅ 自动化测试完成！"
echo "========================================="
echo ""
echo "测试摘要："
echo "- 数据源ID: $SOURCE_ID"
echo "- STRM文件数量: $STRM_COUNT"
echo "- 服务端口: 18888"
echo ""
