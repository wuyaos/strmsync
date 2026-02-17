#!/bin/bash
# DataServer API 测试脚本

set -e

API_BASE="http://localhost:6754/api"
CONTENT_TYPE="Content-Type: application/json"

echo "========================================"
echo "DataServer API 测试"
echo "========================================"
echo ""

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试函数
test_api() {
    local name="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local expect_status="$5"

    echo -e "${YELLOW}测试: ${name}${NC}"

    if [ -z "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$url")
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" -H "$CONTENT_TYPE" -d "$data" "$url")
    fi

    body=$(echo "$response" | head -n -1)
    status=$(echo "$response" | tail -n 1)

    if [ "$status" = "$expect_status" ]; then
        echo -e "${GREEN}✓ 状态码: ${status}${NC}"
        echo "$body" | python3 -m json.tool 2>/dev/null || echo "$body"
    else
        echo -e "${RED}✗ 预期状态码: ${expect_status}, 实际: ${status}${NC}"
        echo "$body"
        return 1
    fi
    echo ""
}

echo "1. 测试健康检查"
test_api "健康检查" "GET" "${API_BASE}/health" "" "200"

echo "2. 创建数据服务器（CloudDrive2）"
server1_response=$(curl -s -X POST -H "$CONTENT_TYPE" \
    -d '{
        "name": "测试CloudDrive2",
        "type": "clouddrive2",
        "host": "192.168.1.100",
        "port": 19798,
        "api_key": "test-api-key-123",
        "enabled": true,
        "options": "{\"timeout\": 30}"
    }' \
    "${API_BASE}/servers/data")
server1_id=$(echo "$server1_response" | python3 -c "import sys, json; print(json.load(sys.stdin)['server']['id'])" 2>/dev/null || echo "")

if [ -z "$server1_id" ]; then
    echo -e "${RED}✗ 创建失败${NC}"
    echo "$server1_response" | python3 -m json.tool
    exit 1
fi

echo -e "${GREEN}✓ 创建成功, ID: ${server1_id}${NC}"
echo "$server1_response" | python3 -m json.tool
echo ""

echo "3. 创建数据服务器（OpenList）"
server2_response=$(curl -s -X POST -H "$CONTENT_TYPE" \
    -d '{
        "name": "测试OpenList",
        "type": "openlist",
        "host": "10.0.0.50",
        "port": 5244,
        "enabled": true
    }' \
    "${API_BASE}/servers/data")
server2_id=$(echo "$server2_response" | python3 -c "import sys, json; print(json.load(sys.stdin)['server']['id'])" 2>/dev/null || echo "")

if [ -z "$server2_id" ]; then
    echo -e "${RED}✗ 创建失败${NC}"
    echo "$server2_response" | python3 -m json.tool
    exit 1
fi

echo -e "${GREEN}✓ 创建成功, ID: ${server2_id}${NC}"
echo "$server2_response" | python3 -m json.tool
echo ""

echo "4. 测试创建重复名称（应失败）"
test_api "重复名称" "POST" "${API_BASE}/servers/data" \
    '{"name":"测试CloudDrive2","type":"clouddrive2","host":"192.168.1.101","port":19798}' \
    "409"

echo "5. 测试创建危险地址（应失败）"
test_api "localhost地址" "POST" "${API_BASE}/servers/data" \
    '{"name":"危险服务器","type":"clouddrive2","host":"localhost","port":19798}' \
    "400"

test_api "回环地址" "POST" "${API_BASE}/servers/data" \
    '{"name":"危险服务器2","type":"clouddrive2","host":"127.0.0.1","port":19798}' \
    "400"

echo "6. 测试无效参数"
test_api "缺少必填字段" "POST" "${API_BASE}/servers/data" \
    '{"name":"测试","type":"clouddrive2"}' \
    "400"

test_api "无效端口" "POST" "${API_BASE}/servers/data" \
    '{"name":"测试端口","type":"clouddrive2","host":"192.168.1.1","port":99999}' \
    "400"

test_api "无效类型" "POST" "${API_BASE}/servers/data" \
    '{"name":"测试类型","type":"invalid","host":"192.168.1.1","port":8080}' \
    "400"

test_api "无效JSON选项" "POST" "${API_BASE}/servers/data" \
    '{"name":"测试JSON","type":"clouddrive2","host":"192.168.1.1","port":8080,"options":"not a json"}' \
    "400"

echo "7. 获取服务器列表"
test_api "列表查询" "GET" "${API_BASE}/servers/data" "" "200"

test_api "列表查询（类型过滤）" "GET" "${API_BASE}/servers/data?type=clouddrive2" "" "200"

test_api "列表查询（分页）" "GET" "${API_BASE}/servers/data?page=1&page_size=10" "" "200"

echo "8. 获取单个服务器"
test_api "获取服务器1" "GET" "${API_BASE}/servers/data/${server1_id}" "" "200"

test_api "获取不存在的服务器" "GET" "${API_BASE}/servers/data/99999" "" "404"

echo "9. 更新服务器"
test_api "更新服务器名称" "PUT" "${API_BASE}/servers/data/${server1_id}" \
    "{\"name\":\"更新后的CloudDrive2\",\"type\":\"clouddrive2\",\"host\":\"192.168.1.100\",\"port\":19798,\"enabled\":false}" \
    "200"

echo "10. 测试连接（会失败，因为服务器不存在）"
test_api "测试连接" "POST" "${API_BASE}/servers/data/${server1_id}/test" "" "200"

echo "11. 测试禁用服务器连接"
# 服务器已在步骤9中禁用
test_api "测试禁用服务器" "POST" "${API_BASE}/servers/data/${server1_id}/test" "" "200"

echo "12. 删除服务器"
test_api "删除服务器2" "DELETE" "${API_BASE}/servers/data/${server2_id}" "" "200"

test_api "删除不存在的服务器" "DELETE" "${API_BASE}/servers/data/99999" "" "404"

echo "13. 清理：删除剩余服务器"
test_api "删除服务器1" "DELETE" "${API_BASE}/servers/data/${server1_id}" "" "200"

echo ""
echo "========================================"
echo -e "${GREEN}✓ 所有测试完成！${NC}"
echo "========================================"
