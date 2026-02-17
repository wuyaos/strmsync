#!/bin/bash

# STRMSync 快速功能测试脚本（使用已存在的数据源）
BASE_URL="http://localhost:18888/api"
SOURCE_ID=1

echo "========================================="
echo "  STRMSync 快速功能测试"
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

# 2. 获取数据源详情
echo "2️⃣  获取数据源详情（ID=$SOURCE_ID）..."
curl -s "$BASE_URL/sources/$SOURCE_ID"
echo ""
echo ""

# 3. 测试扫描功能
echo "3️⃣  触发文件扫描..."
SCAN_RESPONSE=$(curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/scan")
echo "$SCAN_RESPONSE"
echo "✅ 扫描任务已提交"
echo ""

# 等待扫描完成
echo "⏳ 等待10秒让扫描完成..."
sleep 10
echo ""

# 4. 再次查看数据源状态
echo "4️⃣  查看扫描后的数据源状态..."
SCAN_RESULT=$(curl -s "$BASE_URL/sources/$SOURCE_ID")
echo "$SCAN_RESULT"
echo ""
echo ""

# 5. 检查生成的STRM文件
echo "5️⃣  检查生成的STRM文件..."
if [ -d "/mnt/c/Users/wff19/Desktop/strm/test/output" ]; then
    STRM_COUNT=$(find /mnt/c/Users/wff19/Desktop/strm/test/output -name "*.strm" 2>/dev/null | wc -l)
    echo "找到 $STRM_COUNT 个STRM文件"
    if [ $STRM_COUNT -gt 0 ]; then
        echo "✅ STRM文件生成成功"
        echo "示例文件："
        find /mnt/c/Users/wff19/Desktop/strm/test/output -name "*.strm" | head -3
        echo ""
        echo "示例文件内容："
        find /mnt/c/Users/wff19/Desktop/strm/test/output -name "*.strm" | head -1 | xargs cat
    else
        echo "⚠️  未找到STRM文件"
    fi
else
    echo "❌ 输出目录不存在"
fi
echo ""

# 6. 测试文件监控
echo "6️⃣  启动文件监控..."
WATCH_RESPONSE=$(curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/watch/start")
echo "$WATCH_RESPONSE"
echo ""

# 7. 查询监控状态
echo "7️⃣  查询监控状态..."
WATCH_STATUS=$(curl -s "$BASE_URL/sources/$SOURCE_ID/watch/status")
echo "$WATCH_STATUS"
echo ""
echo ""

# 8. 测试元数据同步
echo "8️⃣  测试元数据同步..."
METADATA_RESPONSE=$(curl -s -X POST "$BASE_URL/sources/$SOURCE_ID/metadata/sync")
echo "$METADATA_RESPONSE"
echo ""
echo ""

# 9. 等待元数据同步
echo "⏳ 等待5秒让元数据同步完成..."
sleep 5
echo ""

# 10. 检查元数据文件
echo "🔟 检查元数据文件..."
if [ -d "/mnt/c/Users/wff19/Desktop/strm/test/output" ]; then
    NFO_COUNT=$(find /mnt/c/Users/wff19/Desktop/strm/test/output -name "*.nfo" 2>/dev/null | wc -l)
    echo "找到 $NFO_COUNT 个NFO文件"
    if [ $NFO_COUNT -gt 0 ]; then
        echo "✅ 元数据文件同步成功"
    fi
fi
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
echo "  ✅ 快速测试完成！"
echo "========================================="
echo ""
echo "测试摘要："
echo "- 数据源ID: $SOURCE_ID"
echo "- STRM文件数量: $STRM_COUNT"
echo "- NFO文件数量: $NFO_COUNT"
echo "- 服务端口: 18888"
echo ""
