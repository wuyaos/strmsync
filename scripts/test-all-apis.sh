#!/bin/bash
#
# STRMSync - 全面API测试脚本
# 测试所有后端API端点的功能
#

set -e

# 配置
API_BASE="${API_BASE:-http://localhost:6754}"
TEST_LOG="test-results-$(date +%Y%m%d-%H%M%S).log"
PASSED=0
FAILED=0

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 辅助函数
log() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$TEST_LOG"
}

success() {
    echo -e "${GREEN}[PASS]${NC} $1" | tee -a "$TEST_LOG"
    ((PASSED++))
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1" | tee -a "$TEST_LOG"
    ((FAILED++))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" | tee -a "$TEST_LOG"
}

test_api() {
    local name="$1"
    local method="$2"
    local endpoint="$3"
    local data="$4"
    local expected_code="$5"

    log "测试: $name"

    if [ -z "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$API_BASE$endpoint" 2>&1)
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$API_BASE$endpoint" 2>&1)
    fi

    http_code=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "$expected_code" ]; then
        success "$name (HTTP $http_code)"
        echo "  Response: $(echo "$body" | head -c 200)" >> "$TEST_LOG"
        return 0
    else
        fail "$name (Expected: $expected_code, Got: $http_code)"
        echo "  Response: $body" >> "$TEST_LOG"
        return 1
    fi
}

# 开始测试
echo "========================================" | tee "$TEST_LOG"
echo "STRMSync API 全面测试" | tee -a "$TEST_LOG"
echo "时间: $(date)" | tee -a "$TEST_LOG"
echo "API Base: $API_BASE" | tee -a "$TEST_LOG"
echo "========================================" | tee -a "$TEST_LOG"
echo "" | tee -a "$TEST_LOG"

# 1. 健康检查
log "==== 1. 系统健康检查 ===="
test_api "Health Check" "GET" "/api/health" "" "200"
echo "" | tee -a "$TEST_LOG"

# 2. DataServer (文件系统服务器) API
log "==== 2. DataServer API 测试 ===="

# 2.1 创建DataServer
test_api "Create DataServer (OpenList)" "POST" "/api/servers/data" \
'{
  "name": "Test OpenList Server",
  "type": "openlist",
  "host": "127.0.0.1",
  "port": 5244,
  "api_key": "test_key",
  "enabled": true
}' "201"

# 2.2 列出DataServers
test_api "List DataServers" "GET" "/api/servers/data" "" "200"

# 2.3 获取单个DataServer (假设ID=1)
test_api "Get DataServer by ID" "GET" "/api/servers/data/1" "" "200" || warn "DataServer ID=1 不存在"

# 2.4 更新DataServer
test_api "Update DataServer" "PUT" "/api/servers/data/1" \
'{
  "name": "Updated OpenList Server",
  "type": "openlist",
  "host": "127.0.0.1",
  "port": 5244,
  "api_key": "updated_key",
  "enabled": false
}' "200" || warn "无法更新DataServer ID=1"

echo "" | tee -a "$TEST_LOG"

# 3. MediaServer API
log "==== 3. MediaServer API 测试 ===="

# 3.1 创建MediaServer
test_api "Create MediaServer (Emby)" "POST" "/api/servers/media" \
'{
  "name": "Test Emby Server",
  "type": "emby",
  "host": "127.0.0.1",
  "port": 8096,
  "api_key": "emby_test_key",
  "enabled": true
}' "201"

# 3.2 列出MediaServers
test_api "List MediaServers" "GET" "/api/servers/media" "" "200"

# 3.3 获取单个MediaServer
test_api "Get MediaServer by ID" "GET" "/api/servers/media/1" "" "200" || warn "MediaServer ID=1 不存在"

echo "" | tee -a "$TEST_LOG"

# 4. Job API
log "==== 4. Job API 测试 ===="

# 4.1 创建Job (local模式)
test_api "Create Job (Local Mode)" "POST" "/api/jobs" \
'{
  "name": "Test Local Job",
  "enabled": true,
  "watch_mode": "local",
  "source_path": "/mnt/source",
  "target_path": "/mnt/target",
  "strm_path": "/mnt/media"
}' "201"

# 4.2 列出Jobs
test_api "List Jobs" "GET" "/api/jobs" "" "200"

# 4.3 获取单个Job
test_api "Get Job by ID" "GET" "/api/jobs/1" "" "200" || warn "Job ID=1 不存在"

# 4.4 更新Job
test_api "Update Job" "PUT" "/api/jobs/1" \
'{
  "name": "Updated Test Job",
  "enabled": false,
  "watch_mode": "local",
  "source_path": "/mnt/source",
  "target_path": "/mnt/target",
  "strm_path": "/mnt/media"
}' "200" || warn "无法更新Job ID=1"

echo "" | tee -a "$TEST_LOG"

# 5. TaskRun API
log "==== 5. TaskRun API 测试 ===="

# 5.1 列出TaskRuns
test_api "List TaskRuns" "GET" "/api/runs" "" "200"

# 5.2 按Job筛选TaskRuns
test_api "List TaskRuns by Job" "GET" "/api/runs?job_id=1" "" "200"

# 5.3 获取单个TaskRun
test_api "Get TaskRun by ID" "GET" "/api/runs/1" "" "200" || warn "TaskRun ID=1 不存在"

echo "" | tee -a "$TEST_LOG"

# 6. Log API
log "==== 6. Log API 测试 ===="

# 6.1 列出日志
test_api "List Logs" "GET" "/api/logs" "" "200"

# 6.2 按级别筛选日志
test_api "List Logs (Error Level)" "GET" "/api/logs?level=error" "" "200"

echo "" | tee -a "$TEST_LOG"

# 7. Setting API
log "==== 7. Setting API 测试 ===="

# 7.1 获取所有设置
test_api "Get All Settings" "GET" "/api/settings" "" "200"

# 7.2 更新设置
test_api "Update Settings" "PUT" "/api/settings" \
'{
  "log_level": "debug",
  "max_workers": "4"
}' "200"

echo "" | tee -a "$TEST_LOG"

# 8. 文件操作 API
log "==== 8. File API 测试 ===="

# 8.1 列出文件（需要有效的server_id）
test_api "List Files" "GET" "/api/files/list?server_id=1&path=/&recursive=false" "" "200" || warn "Server ID=1 不存在或不可用"

echo "" | tee -a "$TEST_LOG"

# 9. 错误处理测试
log "==== 9. 错误处理测试 ===="

# 9.1 无效的端点
test_api "Invalid Endpoint" "GET" "/api/invalid" "" "404"

# 9.2 无效的ID
test_api "Invalid Job ID" "GET" "/api/jobs/99999" "" "404"

# 9.3 无效的请求体
test_api "Invalid JSON Body" "POST" "/api/servers/data" \
'{invalid json}' "400"

# 9.4 缺少必填字段
test_api "Missing Required Fields" "POST" "/api/jobs" \
'{
  "name": "Incomplete Job"
}' "400"

echo "" | tee -a "$TEST_LOG"

# 10. 删除操作测试 (放在最后)
log "==== 10. 删除操作测试 ===="

# 10.1 删除Job
test_api "Delete Job" "DELETE" "/api/jobs/1" "" "200" || warn "无法删除Job ID=1"

# 10.2 删除DataServer
test_api "Delete DataServer" "DELETE" "/api/servers/data/1" "" "200" || warn "无法删除DataServer ID=1"

# 10.3 删除MediaServer
test_api "Delete MediaServer" "DELETE" "/api/servers/media/1" "" "200" || warn "无法删除MediaServer ID=1"

echo "" | tee -a "$TEST_LOG"

# 总结
echo "========================================" | tee -a "$TEST_LOG"
echo "测试完成" | tee -a "$TEST_LOG"
echo "========================================" | tee -a "$TEST_LOG"
echo -e "${GREEN}通过: $PASSED${NC}" | tee -a "$TEST_LOG"
echo -e "${RED}失败: $FAILED${NC}" | tee -a "$TEST_LOG"
echo "详细日志: $TEST_LOG"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}存在测试失败，请查看日志文件${NC}"
    exit 1
else
    echo -e "${GREEN}所有测试通过！${NC}"
    exit 0
fi
