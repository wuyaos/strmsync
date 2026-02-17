#!/bin/bash
#
# STRMSync - 生产环境覆盖测试脚本
# 使用test.env中的真实服务器配置进行全面测试
# 注意：遇到错误会输出但继续运行，方便调试
#

# 不使用 set -e，让脚本在错误时继续运行
set +e

# 配置
API_BASE="${API_BASE:-http://localhost:6754}"
TEST_LOG="test-prod-$(date +%Y%m%d-%H%M%S).log"
PASSED=0
FAILED=0

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 辅助函数
log() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$TEST_LOG"
}

success() {
    echo -e "${GREEN}[PASS]${NC} $1" | tee -a "$TEST_LOG"
    PASSED=$((PASSED + 1))
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1" | tee -a "$TEST_LOG"
    FAILED=$((FAILED + 1))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" | tee -a "$TEST_LOG"
}

step() {
    echo -e "\n${CYAN}==== $1 ====${NC}" | tee -a "$TEST_LOG"
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
        # 返回body供后续使用
        echo "$body"
        return 0
    else
        fail "$name (Expected: $expected_code, Got: $http_code)"
        echo "  Response: $body" >> "$TEST_LOG"
        return 1
    fi
}

# 开始测试
echo "========================================" | tee "$TEST_LOG"
echo "STRMSync 生产环境覆盖测试" | tee -a "$TEST_LOG"
echo "时间: $(date)" | tee -a "$TEST_LOG"
echo "API Base: $API_BASE" | tee -a "$TEST_LOG"
echo "========================================" | tee -a "$TEST_LOG"

# ==================== 阶段1: 系统基础测试 ====================
step "阶段1: 系统基础测试"

# 1.1 健康检查
test_api "Health Check" "GET" "/api/health" "" "200"

# 1.2 获取所有设置
test_api "Get All Settings" "GET" "/api/settings" "" "200"

# ==================== 阶段2: 创建服务器配置 ====================
step "阶段2: 创建服务器配置"

# 2.1 创建CloudDrive2服务器
log "创建CloudDrive2服务器（192.168.123.179:19798）"
cd2_response=$(test_api "Create CloudDrive2 Server" "POST" "/api/servers/data" \
'{
  "name": "CloudDrive2-生产",
  "type": "clouddrive2",
  "host": "192.168.123.179",
  "port": 19798,
  "api_key": "68cc50f8-e946-49c4-8e65-0be228e45df8",
  "enabled": true,
  "options": "{\"mount_path\":\"/mnt/a\",\"source_path\":\"/115open/FL/AV/日本/已刮削/other\"}"
}' "201" || true)

cd2_id=$(echo "$cd2_response" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
log "CloudDrive2 Server ID: $cd2_id"

# 2.2 创建OpenList服务器
log "创建OpenList服务器（192.168.123.179:5244）"
openlist_response=$(test_api "Create OpenList Server" "POST" "/api/servers/data" \
'{
  "name": "OpenList-生产",
  "type": "openlist",
  "host": "192.168.123.179",
  "port": 5244,
  "api_key": "openlist-469275dc-37d6-4f4a-9525-1ae36209cb0bOZV39A8rxtg0htFCNPCWPbdqoHH6aEYT6YBfWKNzARjn5NNfqMrDgFGdWG1DUmfs",
  "enabled": true,
  "options": "{\"mount_path\":\"/mnt/a\",\"source_path\":\"/115open/FL/AV/日本/已刮削/other\"}"
}' "201" || true)

openlist_id=$(echo "$openlist_response" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
log "OpenList Server ID: $openlist_id"

# 2.3 创建Emby媒体服务器
log "创建Emby媒体服务器（192.168.123.179:8096）"
emby_response=$(test_api "Create Emby Server" "POST" "/api/servers/media" \
'{
  "name": "Emby-生产",
  "type": "emby",
  "host": "192.168.123.179",
  "port": 8096,
  "api_key": "fa58c9680ed34ffeb31f19f3e19f4ca3",
  "enabled": true
}' "201" || true)

emby_id=$(echo "$emby_response" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
log "Emby Server ID: $emby_id"

# ==================== 阶段3: 验证服务器配置 ====================
step "阶段3: 验证服务器配置"

# 3.1 列出所有DataServer
test_api "List All DataServers" "GET" "/api/servers/data" "" "200"

# 3.2 获取CloudDrive2服务器详情
if [ -n "$cd2_id" ]; then
    test_api "Get CloudDrive2 Server Details" "GET" "/api/servers/data/$cd2_id" "" "200"
else
    warn "跳过CloudDrive2详情测试（创建失败）"
fi

# 3.3 获取OpenList服务器详情
if [ -n "$openlist_id" ]; then
    test_api "Get OpenList Server Details" "GET" "/api/servers/data/$openlist_id" "" "200"
else
    warn "跳过OpenList详情测试（创建失败）"
fi

# 3.4 列出所有MediaServer
test_api "List All MediaServers" "GET" "/api/servers/media" "" "200"

# 3.5 获取Emby服务器详情
if [ -n "$emby_id" ]; then
    test_api "Get Emby Server Details" "GET" "/api/servers/media/$emby_id" "" "200"
else
    warn "跳过Emby详情测试（创建失败）"
fi

# ==================== 阶段4: 连接测试 ====================
step "阶段4: 服务器连接测试"

# 4.1 测试CloudDrive2连接（如果创建成功）
if [ -n "$cd2_id" ]; then
    log "测试CloudDrive2连接..."
    test_api "Test CloudDrive2 Connection" "POST" "/api/servers/data/$cd2_id/test" "" "200" || warn "CloudDrive2连接测试失败"
else
    warn "跳过CloudDrive2连接测试"
fi

# 4.2 测试OpenList连接（如果创建成功）
if [ -n "$openlist_id" ]; then
    log "测试OpenList连接..."
    test_api "Test OpenList Connection" "POST" "/api/servers/data/$openlist_id/test" "" "200" || warn "OpenList连接测试失败"
else
    warn "跳过OpenList连接测试"
fi

# 4.3 测试Emby连接（如果创建成功）
if [ -n "$emby_id" ]; then
    log "测试Emby连接..."
    test_api "Test Emby Connection" "POST" "/api/servers/media/$emby_id/test" "" "200" || warn "Emby连接测试失败"
else
    warn "跳过Emby连接测试"
fi

# ==================== 阶段5: 创建Job ====================
step "阶段5: 创建同步任务"

# 5.1 创建CloudDrive2 Job（如果服务器创建成功）
if [ -n "$cd2_id" ] && [ -n "$emby_id" ]; then
    log "创建CloudDrive2同步任务..."
    cd2_job_response=$(test_api "Create CloudDrive2 Job" "POST" "/api/jobs" \
'{
  "name": "CloudDrive2同步任务",
  "enabled": true,
  "watch_mode": "remote",
  "data_server_id": '"$cd2_id"',
  "media_server_id": '"$emby_id"',
  "source_path": "/115open/FL/AV/日本/已刮削/other",
  "target_path": "/mnt/clouddrive2/media",
  "strm_path": "/mnt/media/clouddrive2"
}' "201" || true)

    cd2_job_id=$(echo "$cd2_job_response" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
    log "CloudDrive2 Job ID: $cd2_job_id"
else
    warn "跳过CloudDrive2 Job创建（依赖服务器未创建）"
fi

# 5.2 创建OpenList Job（如果服务器创建成功）
if [ -n "$openlist_id" ] && [ -n "$emby_id" ]; then
    log "创建OpenList同步任务..."
    openlist_job_response=$(test_api "Create OpenList Job" "POST" "/api/jobs" \
'{
  "name": "OpenList同步任务",
  "enabled": true,
  "watch_mode": "remote",
  "data_server_id": '"$openlist_id"',
  "media_server_id": '"$emby_id"',
  "source_path": "/115open/FL/AV/日本/已刮削/other",
  "target_path": "/mnt/openlist/media",
  "strm_path": "/mnt/media/openlist"
}' "201" || true)

    openlist_job_id=$(echo "$openlist_job_response" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
    log "OpenList Job ID: $openlist_job_id"
else
    warn "跳过OpenList Job创建（依赖服务器未创建）"
fi

# ==================== 阶段6: 验证Job配置 ====================
step "阶段6: 验证任务配置"

# 6.1 列出所有Jobs
test_api "List All Jobs" "GET" "/api/jobs" "" "200"

# 6.2 获取CloudDrive2 Job详情
if [ -n "$cd2_job_id" ]; then
    test_api "Get CloudDrive2 Job Details" "GET" "/api/jobs/$cd2_job_id" "" "200"
fi

# 6.3 获取OpenList Job详情
if [ -n "$openlist_job_id" ]; then
    test_api "Get OpenList Job Details" "GET" "/api/jobs/$openlist_job_id" "" "200"
fi

# ==================== 阶段7: 文件列表测试 ====================
step "阶段7: 文件系统操作测试"

# 7.1 列出CloudDrive2文件（如果服务器可用）
if [ -n "$cd2_id" ]; then
    log "尝试列出CloudDrive2文件..."
    test_api "List CloudDrive2 Files" "GET" "/api/files/list?server_id=$cd2_id&path=/&recursive=false" "" "200" || warn "CloudDrive2文件列表获取失败"
fi

# 7.2 列出OpenList文件（如果服务器可用）
if [ -n "$openlist_id" ]; then
    log "尝试列出OpenList文件..."
    test_api "List OpenList Files" "GET" "/api/files/list?server_id=$openlist_id&path=/&recursive=false" "" "200" || warn "OpenList文件列表获取失败"
fi

# ==================== 阶段8: 更新操作测试 ====================
step "阶段8: 更新操作测试"

# 8.1 更新CloudDrive2服务器
if [ -n "$cd2_id" ]; then
    test_api "Update CloudDrive2 Server" "PUT" "/api/servers/data/$cd2_id" \
'{
  "name": "CloudDrive2-生产（已更新）",
  "type": "clouddrive2",
  "host": "192.168.123.179",
  "port": 19798,
  "api_key": "68cc50f8-e946-49c4-8e65-0be228e45df8",
  "enabled": true,
  "options": "{\"mount_path\":\"/mnt/a\",\"source_path\":\"/115open/FL/AV/日本/已刮削/other\"}"
}' "200" || warn "CloudDrive2更新失败"
fi

# 8.2 更新Job状态
if [ -n "$cd2_job_id" ]; then
    test_api "Disable CloudDrive2 Job" "PUT" "/api/jobs/$cd2_job_id" \
'{
  "name": "CloudDrive2同步任务",
  "enabled": false,
  "watch_mode": "remote",
  "data_server_id": '"$cd2_id"',
  "media_server_id": '"$emby_id"',
  "source_path": "/115open/FL/AV/日本/已刮削/other",
  "target_path": "/mnt/clouddrive2/media",
  "strm_path": "/mnt/media/clouddrive2"
}' "200" || warn "Job更新失败"
fi

# ==================== 阶段9: 查询操作测试 ====================
step "阶段9: 查询和统计测试"

# 9.1 查询TaskRuns
test_api "List All TaskRuns" "GET" "/api/runs" "" "200"

# 9.2 按Job查询TaskRuns
if [ -n "$cd2_job_id" ]; then
    test_api "List TaskRuns by Job" "GET" "/api/runs?job_id=$cd2_job_id" "" "200"
fi

# 9.3 查询日志
test_api "List Logs" "GET" "/api/logs" "" "200"

# 9.4 按级别查询日志
test_api "List Error Logs" "GET" "/api/logs?level=error" "" "200"

# ==================== 阶段10: 清理测试 ====================
step "阶段10: 清理测试数据"

# 10.1 删除Jobs
if [ -n "$cd2_job_id" ]; then
    test_api "Delete CloudDrive2 Job" "DELETE" "/api/jobs/$cd2_job_id" "" "200" || warn "Job删除失败"
fi

if [ -n "$openlist_job_id" ]; then
    test_api "Delete OpenList Job" "DELETE" "/api/jobs/$openlist_job_id" "" "200" || warn "Job删除失败"
fi

# 10.2 删除服务器
if [ -n "$cd2_id" ]; then
    test_api "Delete CloudDrive2 Server" "DELETE" "/api/servers/data/$cd2_id" "" "200" || warn "DataServer删除失败"
fi

if [ -n "$openlist_id" ]; then
    test_api "Delete OpenList Server" "DELETE" "/api/servers/data/$openlist_id" "" "200" || warn "DataServer删除失败"
fi

if [ -n "$emby_id" ]; then
    test_api "Delete Emby Server" "DELETE" "/api/servers/media/$emby_id" "" "200" || warn "MediaServer删除失败"
fi

# ==================== 总结 ====================
echo "" | tee -a "$TEST_LOG"
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
