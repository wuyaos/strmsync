#!/bin/bash
#
# STRMSync - 持续性监控测试脚本
# 在后台运行，持续测试API的可用性和性能
#

# 配置
API_BASE="${API_BASE:-http://localhost:6754}"
TEST_INTERVAL="${TEST_INTERVAL:-60}"  # 测试间隔（秒）
LOG_DIR="continuous-test-logs"
STATS_FILE="$LOG_DIR/stats.json"

# 创建日志目录
mkdir -p "$LOG_DIR"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# 统计数据
declare -A test_counts
declare -A test_success
declare -A test_failures
declare -A test_latency

# 辅助函数
log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

# 测试API并记录统计
test_api() {
    local name="$1"
    local method="$2"
    local endpoint="$3"
    local data="$4"
    local expected_code="$5"

    # 初始化统计
    if [ -z "${test_counts[$name]}" ]; then
        test_counts[$name]=0
        test_success[$name]=0
        test_failures[$name]=0
        test_latency[$name]=0
    fi

    # 执行测试
    local start_time=$(date +%s%N)

    if [ -z "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$API_BASE$endpoint" 2>&1)
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$API_BASE$endpoint" 2>&1)
    fi

    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))  # 转换为毫秒

    http_code=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | sed '$d')

    # 更新统计
    test_counts[$name]=$((test_counts[$name] + 1))
    test_latency[$name]=$(( (test_latency[$name] * (test_counts[$name] - 1) + latency) / test_counts[$name] ))

    if [ "$http_code" = "$expected_code" ]; then
        test_success[$name]=$((test_success[$name] + 1))
        success "$name (${latency}ms)"
        return 0
    else
        test_failures[$name]=$((test_failures[$name] + 1))
        fail "$name - Expected: $expected_code, Got: $http_code (${latency}ms)"
        # 输出错误详情
        if [ -n "$body" ]; then
            echo "  Error: $(echo "$body" | head -c 200)"
        fi
        return 1
    fi
}

# 打印统计信息
print_stats() {
    local total_tests=0
    local total_success=0
    local total_failures=0

    echo ""
    echo -e "${MAGENTA}========================================${NC}"
    echo -e "${MAGENTA}      累计测试统计 ($(date))${NC}"
    echo -e "${MAGENTA}========================================${NC}"

    printf "${CYAN}%-40s %8s %8s %8s %10s${NC}\n" "测试名称" "总数" "成功" "失败" "平均延迟"
    echo "--------------------------------------------------------------------------------"

    for test_name in "${!test_counts[@]}"; do
        local count=${test_counts[$test_name]}
        local succ=${test_success[$test_name]}
        local fail=${test_failures[$test_name]}
        local lat=${test_latency[$test_name]}
        local rate=$(awk "BEGIN {printf \"%.1f\", ($succ/$count)*100}")

        total_tests=$((total_tests + count))
        total_success=$((total_success + succ))
        total_failures=$((total_failures + fail))

        if [ $fail -eq 0 ]; then
            color=$GREEN
        elif [ $fail -lt $succ ]; then
            color=$YELLOW
        else
            color=$RED
        fi

        printf "${color}%-40s %8d %8d %8d %8dms (${rate}%%)${NC}\n" \
            "$test_name" "$count" "$succ" "$fail" "$lat"
    done

    echo "--------------------------------------------------------------------------------"
    printf "${CYAN}%-40s %8d %8d %8d${NC}\n" \
        "总计" "$total_tests" "$total_success" "$total_failures"

    echo ""
}

# 保存统计到JSON
save_stats() {
    local json="{"
    json+="\"timestamp\":\"$(date -Iseconds)\","
    json+="\"tests\":["

    local first=true
    for test_name in "${!test_counts[@]}"; do
        if [ "$first" = false ]; then
            json+=","
        fi
        first=false

        json+="{"
        json+="\"name\":\"$test_name\","
        json+="\"total\":${test_counts[$test_name]},"
        json+="\"success\":${test_success[$test_name]},"
        json+="\"failures\":${test_failures[$test_name]},"
        json+="\"avg_latency_ms\":${test_latency[$test_name]}"
        json+="}"
    done

    json+="]}"

    echo "$json" > "$STATS_FILE"
}

# 信号处理
cleanup() {
    echo ""
    info "收到退出信号，保存统计数据..."
    save_stats
    print_stats
    info "持续测试已停止"
    exit 0
}

trap cleanup SIGINT SIGTERM

# 主测试循环
run_continuous_tests() {
    local iteration=0

    info "开始持续性测试..."
    info "API Base: $API_BASE"
    info "测试间隔: ${TEST_INTERVAL}秒"
    info "按 Ctrl+C 停止测试"
    echo ""

    while true; do
        iteration=$((iteration + 1))
        local test_log="$LOG_DIR/test-$(date +%Y%m%d-%H%M%S).log"

        echo -e "${MAGENTA}==================== 第 $iteration 轮测试 ($(date)) ====================${NC}" | tee "$test_log"

        # 基础健康检查
        test_api "Health Check" "GET" "/api/health" "" "200" | tee -a "$test_log"

        # 服务器管理API
        test_api "List DataServers" "GET" "/api/servers/data" "" "200" | tee -a "$test_log"
        test_api "List MediaServers" "GET" "/api/servers/media" "" "200" | tee -a "$test_log"

        # Job管理API
        test_api "List Jobs" "GET" "/api/jobs" "" "200" | tee -a "$test_log"

        # TaskRun查询API
        test_api "List TaskRuns" "GET" "/api/runs" "" "200" | tee -a "$test_log"

        # 日志查询API
        test_api "List Logs" "GET" "/api/logs" "" "200" | tee -a "$test_log"

        # 设置API
        test_api "Get Settings" "GET" "/api/settings" "" "200" | tee -a "$test_log"

        echo "" | tee -a "$test_log"

        # 每10轮打印一次统计
        if [ $((iteration % 10)) -eq 0 ]; then
            print_stats
            save_stats
        fi

        # 等待下一轮测试
        sleep "$TEST_INTERVAL"
    done
}

# 启动测试
run_continuous_tests
