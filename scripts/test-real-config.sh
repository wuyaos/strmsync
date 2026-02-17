#!/bin/bash
# STRMSync 实际配置测试脚本

set -e

API_BASE="http://localhost:6754/api"
CONTENT_TYPE="Content-Type: application/json"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================"
echo "STRMSync 实际配置测试"
echo "========================================"
echo ""

# 测试函数
test_and_print() {
    local name="$1"
    local response="$2"

    echo -e "${BLUE}$name${NC}"
    echo "$response" | python3 -m json.tool 2>/dev/null || echo "$response"
    echo ""
}

echo "========== 1. 创建CloudDrive2服务器 =========="
cd2_response=$(curl -s -X POST -H "$CONTENT_TYPE" \
    -d '{
        "name": "CloudDrive2-生产",
        "type": "clouddrive2",
        "host": "192.168.123.179",
        "port": 19798,
        "api_key": "68cc50f8-e946-49c4-8e65-0be228e45df8",
        "enabled": true,
        "options": "{\"mount_path\":\"/mnt/a\",\"source_path\":\"/115open/FL/AV/日本/已刮削/other\"}"
    }' \
    "${API_BASE}/servers/data")

cd2_id=$(echo "$cd2_response" | python3 -c "import sys, json; print(json.load(sys.stdin)['server']['id'])" 2>/dev/null || echo "")
if [ -z "$cd2_id" ]; then
    echo -e "${RED}✗ 创建失败${NC}"
    echo "$cd2_response" | python3 -m json.tool
    exit 1
fi

echo -e "${GREEN}✓ CloudDrive2服务器创建成功 (ID: ${cd2_id})${NC}"
echo "$cd2_response" | python3 -m json.tool
echo ""

echo "========== 2. 测试CloudDrive2连接 =========="
cd2_test=$(curl -s -X POST "${API_BASE}/servers/data/${cd2_id}/test")
test_and_print "CloudDrive2连接测试结果" "$cd2_test"

echo "========== 3. 创建OpenList服务器 =========="
openlist_response=$(curl -s -X POST -H "$CONTENT_TYPE" \
    -d '{
        "name": "OpenList-生产",
        "type": "openlist",
        "host": "192.168.123.179",
        "port": 5244,
        "api_key": "openlist-469275dc-37d6-4f4a-9525-1ae36209cb0bOZV39A8rxtg0htFCNPCWPbdqoHH6aEYT6YBfWKNzARjn5NNfqMrDgFGdWG1DUmfs",
        "enabled": true,
        "options": "{\"mount_path\":\"/mnt/a\",\"source_path\":\"/115open/FL/AV/日本/已刮削/other\"}"
    }' \
    "${API_BASE}/servers/data")

openlist_id=$(echo "$openlist_response" | python3 -c "import sys, json; print(json.load(sys.stdin)['server']['id'])" 2>/dev/null || echo "")
if [ -z "$openlist_id" ]; then
    echo -e "${RED}✗ 创建失败${NC}"
    echo "$openlist_response" | python3 -m json.tool
    exit 1
fi

echo -e "${GREEN}✓ OpenList服务器创建成功 (ID: ${openlist_id})${NC}"
echo "$openlist_response" | python3 -m json.tool
echo ""

echo "========== 4. 测试OpenList连接 =========="
openlist_test=$(curl -s -X POST "${API_BASE}/servers/data/${openlist_id}/test")
test_and_print "OpenList连接测试结果" "$openlist_test"

echo "========== 5. 创建Emby服务器 =========="
emby_response=$(curl -s -X POST -H "$CONTENT_TYPE" \
    -d '{
        "name": "Emby-生产",
        "type": "emby",
        "host": "192.168.123.179",
        "port": 8096,
        "api_key": "fa58c9680ed34ffeb31f19f3e19f4ca3",
        "enabled": true
    }' \
    "${API_BASE}/servers/media")

emby_id=$(echo "$emby_response" | python3 -c "import sys, json; print(json.load(sys.stdin)['server']['id'])" 2>/dev/null || echo "")
if [ -z "$emby_id" ]; then
    echo -e "${RED}✗ 创建失败${NC}"
    echo "$emby_response" | python3 -m json.tool
    exit 1
fi

echo -e "${GREEN}✓ Emby服务器创建成功 (ID: ${emby_id})${NC}"
echo "$emby_response" | python3 -m json.tool
echo ""

echo "========== 6. 测试Emby连接 =========="
emby_test=$(curl -s -X POST "${API_BASE}/servers/media/${emby_id}/test")
test_and_print "Emby连接测试结果" "$emby_test"

echo "========== 7. 查询所有数据服务器 =========="
data_servers=$(curl -s "${API_BASE}/servers/data")
test_and_print "数据服务器列表" "$data_servers"

echo "========== 8. 查询所有媒体服务器 =========="
media_servers=$(curl -s "${API_BASE}/servers/media")
test_and_print "媒体服务器列表" "$media_servers"

echo "========== 9. 测试文件系统浏览（CloudDrive2） =========="
files_cd2=$(curl -s "${API_BASE}/files/directories?mode=api&type=clouddrive2&host=192.168.123.179&port=19798&apiKey=68cc50f8-e946-49c4-8e65-0be228e45df8&path=/115open/FL/AV/日本/已刮削/other")
test_and_print "CloudDrive2文件列表" "$files_cd2"

echo "========== 10. 测试文件系统浏览（OpenList） =========="
files_openlist=$(curl -s "${API_BASE}/files/directories?mode=api&type=openlist&host=192.168.123.179&port=5244&apiKey=openlist-469275dc-37d6-4f4a-9525-1ae36209cb0bOZV39A8rxtg0htFCNPCWPbdqoHH6aEYT6YBfWKNzARjn5NNfqMrDgFGdWG1DUmfs&path=/115open/FL/AV/日本/已刮削/other")
test_and_print "OpenList文件列表" "$files_openlist"

echo ""
echo "========================================"
echo -e "${GREEN}✓ 实际配置测试完成！${NC}"
echo "========================================"
echo ""
echo "测试结果汇总:"
echo "- CloudDrive2 服务器 ID: $cd2_id"
echo "- OpenList 服务器 ID: $openlist_id"
echo "- Emby 服务器 ID: $emby_id"
echo ""
echo "查看日志:"
echo "  tail -f logs/strmsync.log"
