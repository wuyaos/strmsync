# STRMSync 生产环境测试计划

## 测试概览

本文档描述了文件列表API和日志系统改进的完整测试计划。

---

## 1. 功能测试

### 1.1 文件列表API (`POST /api/files/list`)

#### 测试用例矩阵

| 服务器类型 | 路径 | 递归 | 预期结果 |
|-----------|------|------|---------|
| CloudDrive2 | `/` | false | 返回根目录文件和目录 |
| CloudDrive2 | `/` | true | 递归返回所有文件和目录 |
| CloudDrive2 | `/subdir` | false | 返回子目录内容 |
| OpenList | `/` | false | 返回根目录文件和目录 |
| OpenList | `/` | true | 递归返回所有文件和目录 |
| Local | `/` | false | 返回挂载点根目录内容 |
| Local | `/subdir` | true | 递归返回子目录所有内容 |

#### 测试数据验证

对于每个测试用例，验证返回数据：
- `server_id`: 匹配请求
- `path`: 匹配请求
- `recursive`: 匹配请求
- `count`: 等于 `files` 数组长度
- `files`: 数组，每个元素包含：
  - `path`: 完整虚拟路径
  - `name`: 文件/目录名
  - `size`: 文件大小（字节）
  - `mod_time`: 修改时间（RFC3339格式）
  - `is_dir`: 是否为目录

#### 边界情况测试

| 测试场景 | 预期HTTP状态码 | 预期错误信息 |
|---------|---------------|-------------|
| server_id 不存在 | 404 | `data server not found` |
| server_id 禁用 | 403 | `data server disabled` |
| server_id = 0 | 400 | `server_id is required` |
| 无效的 path | 400/500 | 依赖具体实现 |
| 服务器连接失败 | 500 | 包含底层错误 |
| 无权限访问路径 | 401/403 | 认证/授权错误 |

---

## 2. 日志系统测试

### 2.1 Request ID 跟踪

**测试步骤：**
1. 发送请求时带自定义 `X-Request-ID` header
2. 检查响应header包含相同的 `X-Request-ID`
3. 查询日志表，验证该 request_id 出现在所有相关日志记录中

**验证点：**
- Gin middleware 正确传递 request_id
- Logger 记录包含 request_id
- 数据库日志包含 request_id 字段

### 2.2 Caller 信息

**测试步骤：**
1. 触发不同层级的日志记录（Handler、Service、Filesystem）
2. 检查日志输出包含正确的文件名和行号

**验证点：**
- 日志格式包含 `caller` 字段
- caller 指向实际调用位置（而非logger封装）
- 不同层级的日志有不同的 caller

### 2.3 Stacktrace（错误场景）

**测试步骤：**
1. 触发错误级别日志（如服务器连接失败）
2. 检查日志包含完整的调用栈

**验证点：**
- Error 和以上级别包含 stacktrace
- Stacktrace 有助于定位问题根源

### 2.4 User Action 标记

**测试步骤：**
1. 发送请求时带 `X-User-Action` header（如 "list_files"）
2. 检查日志记录包含该 user_action

**验证点：**
- 数据库日志表有 user_action 字段
- 可按 user_action 筛选用户操作

---

## 3. 性能测试

### 3.1 基准测试

| 场景 | 文件数量 | 递归深度 | 目标响应时间 |
|-----|---------|---------|------------|
| 小型目录 | < 100 | 1 | < 1s |
| 中型目录 | 100-1000 | 3 | < 5s |
| 大型目录 | > 1000 | 5 | < 30s |

### 3.2 并发测试

- 10个并发请求，验证无race condition
- 检查日志系统不阻塞主请求
- 验证request_id正确隔离

---

## 4. 安全测试

### 4.1 路径遍历攻击

测试路径：
- `/../../../etc/passwd`
- `..\\..\\windows\\system32`
- `/mnt/../../root`

预期：全部被拒绝或规范化

### 4.2 SQL注入（间接）

- server_id = `1 OR 1=1`
- path = `'; DROP TABLE files; --`

预期：参数化查询阻止注入

### 4.3 认证绕过

- 访问禁用的服务器
- 使用无效的 API Key/Token

预期：返回401/403

---

## 5. 集成测试

### 5.1 端到端流程

1. **创建数据服务器** → POST `/api/servers/data`
2. **测试连接** → POST `/api/servers/data/:id/test`
3. **列出文件** → POST `/api/files/list`
4. **创建任务** → POST `/api/jobs`（使用上述文件）
5. **运行任务** → POST `/api/jobs/:id/run`
6. **查询执行记录** → GET `/api/runs`
7. **查询日志** → GET `/api/logs?request_id=xxx`

### 5.2 多文件系统混合

- 同时配置CloudDrive2、OpenList、Local
- 分别列出每个文件系统的内容
- 验证结果格式一致

---

## 6. 回归测试

### 6.1 现有功能验证

- ✅ 健康检查 (`GET /api/health`)
- ✅ 数据服务器CRUD
- ✅ 媒体服务器CRUD
- ✅ 任务CRUD
- ✅ 任务运行和停止
- ✅ 执行记录查询
- ✅ 日志查询和清理
- ✅ 系统设置

### 6.2 向后兼容性

- 旧的 `/api/files/directories` 端点仍然工作
- 现有Job配置不受影响

---

## 7. 测试执行

### 自动化测试脚本

```bash
# 1. 启动服务
cd backend
./strmsync &
STRMSYNC_PID=$!

# 2. 等待服务就绪
sleep 3

# 3. 运行测试套件
cd ../tests
./test-production-env.sh

# 4. 检查结果
if [ $? -eq 0 ]; then
    echo "✅ 所有测试通过"
else
    echo "❌ 部分测试失败，查看日志"
fi

# 5. 清理
kill $STRMSYNC_PID
```

### 手动测试检查清单

- [ ] CloudDrive2 文件列表（非递归）
- [ ] CloudDrive2 文件列表（递归）
- [ ] OpenList 文件列表（非递归）
- [ ] OpenList 文件列表（递归）
- [ ] Local 文件列表
- [ ] Request ID 在日志中出现
- [ ] Caller 信息准确
- [ ] 错误场景有 stacktrace
- [ ] User Action 记录正确
- [ ] 性能符合预期
- [ ] 安全漏洞不存在

---

## 8. 问题跟踪

### 已知问题（来自Codex Review）

| 问题 | 严重性 | 状态 | 备注 |
|-----|-------|------|------|
| ~~Import别名错误~~ | High | ✅已修复 | main.go使用handlers别名 |
| ~~OpenList丢失目录~~ | High | ✅已修复 | 修改为返回文件+目录 |
| ~~错误判断耦合~~ | Medium | ✅已修复 | 改用errors.Is |
| `/api/files/directories`越权风险 | Medium | ⏳待修复 | 需添加鉴权或限制路径 |
| 日志落库不等待flush | Low | ⏳待修复 | 优雅关闭时应等待 |
| User-Action header未验证 | Low | ⏳待修复 | 应限制长度 |

### 测试发现的问题

_测试执行后填写_

---

## 9. 发布检查清单

部署到生产前：

- [ ] 所有自动化测试通过
- [ ] 手动测试验证核心功能
- [ ] 性能测试达标
- [ ] 安全审计无高危问题
- [ ] 文档已更新
- [ ] CHANGELOG已记录
- [ ] 数据库迁移脚本准备（如需要）
- [ ] 回滚方案已准备

---

**文档版本：** v1.0
**创建日期：** 2026-02-18
**最后更新：** 2026-02-18
