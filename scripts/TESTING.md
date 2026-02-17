# STRMSync 测试指南

## 测试环境说明

测试环境位于 `tests/` 目录，包含：
- `strmsync` - 编译的服务器二进制文件
- `data.db` - 数据库文件（与执行文件同级）
- `test.env` - 测试环境配置
- `server.log` - 服务器运行日志

## 测试脚本

### 1. 生产环境覆盖测试 (`test-production-env.sh`)

**功能**: 使用test.env中的真实服务器配置进行全面API测试

**特点**:
- ✅ 遇到错误输出但继续运行（方便调试）
- ✅ 测试所有API端点（服务器、任务、日志等）
- ✅ 自动创建、测试、清理测试数据
- ✅ 生成详细的测试日志

**运行方式**:
```bash
cd tests
./test-production-env.sh
```

**测试阶段**:
1. 系统基础测试（健康检查、设置）
2. 创建服务器配置（CloudDrive2、OpenList、Emby）
3. 验证服务器配置（列表、详情）
4. 服务器连接测试（测试真实连接）
5. 创建同步任务（Job）
6. 验证任务配置
7. 文件系统操作测试
8. 更新操作测试
9. 查询和统计测试
10. 清理测试数据

**输出**:
- 屏幕输出：彩色的测试结果
- 日志文件：`test-prod-YYYYMMDD-HHMMSS.log`

### 2. 持续性监控测试 (`continuous-test.sh`)

**功能**: 在后台持续运行，监控API可用性和性能

**特点**:
- ✅ 后台运行，持续监控
- ✅ 记录每个API的成功率和平均延迟
- ✅ 每10轮自动输出统计报告
- ✅ 保存统计数据到JSON文件
- ✅ 支持优雅退出（Ctrl+C）

**运行方式**:
```bash
cd tests

# 启动持续测试（默认60秒间隔）
nohup ./continuous-test.sh > continuous-test.out 2>&1 &
echo $! > continuous-test.pid

# 自定义测试间隔（例如30秒）
TEST_INTERVAL=30 nohup ./continuous-test.sh > continuous-test.out 2>&1 &
echo $! > continuous-test.pid

# 查看实时输出
tail -f continuous-test.out

# 停止测试
kill $(cat continuous-test.pid)
```

**输出文件**:
- `continuous-test.out` - 实时测试输出
- `continuous-test-logs/` - 详细测试日志
- `continuous-test-logs/stats.json` - 统计数据

**统计指标**:
- 测试总数
- 成功次数
- 失败次数
- 平均延迟（毫秒）
- 成功率（百分比）

## 服务器管理

### 启动服务器
```bash
cd tests

# 方式1: 使用test.env配置
source test.env
./strmsync > server.log 2>&1 &
echo $! > server.pid

# 方式2: 手动指定环境变量
export ENCRYPTION_KEY="test-key-12345678901234567890123456789012"
export LOG_LEVEL="debug"
export ALLOW_LOOPBACK="true"
./strmsync > server.log 2>&1 &
echo $! > server.pid
```

### 停止服务器
```bash
kill $(cat server.pid)
```

### 查看服务器日志
```bash
tail -f server.log

# 或查看最近100行
tail -100 server.log
```

## 测试结果分析

### 最近一次覆盖测试结果

**通过**: 18个测试
**失败**: 2个测试

**失败原因**:
- 文件列表API未实现（返回404）
  - `/api/files/list` - 正在重构中

**成功测试**:
- ✅ CloudDrive2 连接测试（延迟1572ms）
- ✅ OpenList 连接测试（延迟761ms）
- ✅ Emby 连接测试（延迟759ms）
- ✅ 所有CRUD操作（创建、读取、更新、删除）

## 环境变量说明

### 测试环境变量 (test.env)

```bash
# 基础配置
ENCRYPTION_KEY="test-key-12345678901234567890123456789012"  # 加密密钥
LOG_LEVEL="debug"                                          # 日志级别
LOG_PATH="logs/test-server.log"                           # 日志路径
SERVER_HOST="0.0.0.0"                                      # 监听地址
SERVER_PORT="6754"                                         # 监听端口

# 测试模式
ALLOW_LOOPBACK="true"                                      # 允许回环地址（测试用）

# 扫描器配置
SCANNER_CONCURRENCY="10"                                   # 并发数
SCANNER_BATCH_SIZE="100"                                   # 批量大小

# 通知器配置
NOTIFIER_ENABLED="false"                                   # 是否启用通知
```

### 真实服务器配置 (在test.env中)

#### CloudDrive2 服务器
- Host: 192.168.123.179
- Port: 19798
- Type: clouddrive2

#### OpenList 服务器
- Host: 192.168.123.179
- Port: 5244
- Type: openlist

#### Emby 媒体服务器
- Host: 192.168.123.179
- Port: 8096
- Type: emby

## 常见问题

### 1. 端口被占用
```bash
# 检查端口占用
netstat -tulpn | grep 6754

# 或
lsof -i :6754

# 停止占用进程
kill <PID>
```

### 2. 数据库锁定
```bash
# 停止所有strmsync进程
pkill -f strmsync

# 删除数据库并重新启动
rm -f data.db
./strmsync
```

### 3. 测试脚本权限问题
```bash
chmod +x test-production-env.sh
chmod +x continuous-test.sh
```

## 最佳实践

1. **开发测试**: 使用 `test-production-env.sh` 进行一次性全面测试
2. **持续监控**: 使用 `continuous-test.sh` 在后台监控API稳定性
3. **日志查看**: 定期检查 `server.log` 和测试日志
4. **统计分析**: 查看 `continuous-test-logs/stats.json` 分析性能趋势

## 下一步

- [ ] 实现文件列表API (`/api/files/list`)
- [ ] 添加更多集成测试场景
- [ ] 实现性能基准测试
- [ ] 添加压力测试脚本
