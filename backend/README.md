# STRMSync 后端 API 文档

> 与当前后端实现保持一致（`backend/internal/transport` + `backend/cmd/server`）

## 基础信息

- **基础URL**: `http://localhost:5677/api`
- **协议**: HTTP/1.1
- **认证**: 无（待实现）
- **Content-Type**: `application/json`

## 通用响应

### 列表响应

列表接口统一返回：

```json
{
  "items": [],
  "total": 0,
  "page": 1,
  "page_size": 50
}
```

说明：实际字段名随资源而变化（如 `servers` / `jobs` / `runs` / `logs`），分页参数固定为 `page` 与 `page_size`。

### 错误响应

统一错误响应：

```json
{
  "code": "invalid_request",
  "message": "错误描述",
  "field_errors": [
    { "field": "name", "message": "不能为空" }
  ]
}
```

字段校验错误（结构化）：

```json
{
  "code": 400,
  "message": "validation failed",
  "errors": {
    "name": ["不能为空"],
    "type": ["无效类型"]
  }
}
```

常见HTTP状态码：
- `200`: 成功
- `201`: 创建成功
- `400`: 请求参数错误
- `403`: 禁用/无权限
- `404`: 资源不存在
- `409`: 冲突（如重复名称/运行中）
- `500`: 服务器内部错误

---

## 系统

### 1. 健康检查

**接口**: `GET /api/health`

**响应示例**:
```json
{
  "status": "healthy",
  "timestamp": 1700000000,
  "database": "ok",
  "version": "2.0.0-alpha",
  "frontend_version": "2.0.0-alpha",
  "note": "Minimal version during refactoring"
}
```

---

### 2. 获取日志

**接口**: `GET /api/logs`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | 否 | 页码（默认1）|
| `page_size` | int | 否 | 每页数量（默认50，最大200）|
| `level` | string | 否 | 日志级别（`debug` / `info` / `warn` / `error`）|
| `module` | string | 否 | 模块过滤（如 `api` / `system` / `worker`）|
| `search` | string | 否 | 消息包含关键词 |
| `job_id` | int | 否 | 任务ID过滤 |
| `start_at` | string | 否 | 起始时间（RFC3339 或 `YYYY-MM-DD HH:mm:ss`）|
| `end_at` | string | 否 | 结束时间（RFC3339 或 `YYYY-MM-DD HH:mm:ss`）|

**响应示例**:
```json
{
  "logs": [
    {
      "id": 1,
      "level": "info",
      "module": "api",
      "message": "系统日志：查询（200）",
      "job_id": 12,
      "request_id": "abc",
      "user_action": "",
      "created_at": "2026-02-20T12:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 50
}
```

### 3. 清理日志

**接口**: `POST /api/logs/cleanup`

**请求体**:
```json
{ "days": 30 }
```

**响应示例**:
```json
{ "message": "清理成功", "deleted": 10, "kept": 120 }
```

---

### 4. 获取系统设置

**接口**: `GET /api/settings`

当前实现为占位：
```json
{ "settings": {} }
```

### 5. 更新系统设置

**接口**: `PUT /api/settings`

当前实现为占位：
```json
{ "message": "设置已更新" }
```

---

## 服务器类型

### 1. 获取服务器类型列表

**接口**: `GET /api/servers/types`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `category` | string | 否 | 类型分类（`data` / `media`）|

**响应示例**:
```json
{ "types": [ { "type": "local", "category": "data", "sections": [] } ] }
```

### 2. 获取服务器类型详情

**接口**: `GET /api/servers/types/:type`

**响应示例**:
```json
{ "type": { "type": "openlist", "category": "data", "sections": [] } }
```

---

## 数据服务器

### 1. 获取数据服务器列表

**接口**: `GET /api/servers/data`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | 否 | 页码（默认1）|
| `page_size` | int | 否 | 每页数量（默认50，最大200）|
| `type` | string | 否 | 服务器类型（`local` / `clouddrive2` / `openlist`）|
| `enabled` | string | 否 | 启用状态（`true` / `false`）|

**响应示例**:
```json
{ "servers": [], "total": 0, "page": 1, "page_size": 50 }
```

### 2. 创建数据服务器

**接口**: `POST /api/servers/data`

**请求体**:
```json
{
  "name": "DataServer-1",
  "type": "clouddrive2",
  "host": "127.0.0.1",
  "port": 19798,
  "api_key": "",
  "enabled": true,
  "options": "{}",
  "request_timeout_ms": 30000,
  "connect_timeout_ms": 10000,
  "retry_max": 3,
  "retry_backoff_ms": 1000,
  "max_concurrent": 10
}
```

**响应示例**:
```json
{ "server": { "id": 1, "name": "DataServer-1" } }
```

说明：`local` 类型会强制使用 `host=localhost`、`port=0`。

### 3. 获取数据服务器详情

**接口**: `GET /api/servers/data/:id`

**响应**: `{"server": { ... }}`

### 4. 更新数据服务器

**接口**: `PUT /api/servers/data/:id`

**请求体**: 与创建相同

**响应**: `{"server": { ... }}`

### 5. 删除数据服务器

**接口**: `DELETE /api/servers/data/:id`

**响应**:
```json
{ "message": "删除成功" }
```

### 6. 测试数据服务器连接

**接口**: `POST /api/servers/data/:id/test`

**响应示例**:
```json
{ "success": true, "message": "连接测试成功", "latency_ms": 120 }
```

### 7. 临时测试数据服务器连接

**接口**: `POST /api/servers/data/test`

**请求体**: 与创建相同（不保存）

**响应**: `ConnectionTestResult`

---

## 媒体服务器

### 1. 获取媒体服务器列表

**接口**: `GET /api/servers/media`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | 否 | 页码（默认1）|
| `page_size` | int | 否 | 每页数量（默认50，最大200）|
| `type` | string | 否 | 服务器类型（`emby` / `jellyfin` / `plex`）|
| `enabled` | string | 否 | 启用状态（`true` / `false`）|

**响应示例**:
```json
{ "servers": [], "total": 0, "page": 1, "page_size": 50 }
```

### 2. 创建媒体服务器

**接口**: `POST /api/servers/media`

**请求体**:
```json
{
  "name": "MediaServer-1",
  "type": "emby",
  "host": "127.0.0.1",
  "port": 8096,
  "api_key": "",
  "enabled": true,
  "options": "{}",
  "request_timeout_ms": 30000,
  "connect_timeout_ms": 10000,
  "retry_max": 3,
  "retry_backoff_ms": 1000,
  "max_concurrent": 10
}
```

**响应示例**:
```json
{ "server": { "id": 1, "name": "MediaServer-1" } }
```

### 3. 获取媒体服务器详情

**接口**: `GET /api/servers/media/:id`

**响应**: `{"server": { ... }}`

### 4. 更新媒体服务器

**接口**: `PUT /api/servers/media/:id`

**请求体**: 与创建相同

**响应**: `{"server": { ... }}`

### 5. 删除媒体服务器

**接口**: `DELETE /api/servers/media/:id`

**响应**:
```json
{ "message": "删除成功" }
```

### 6. 测试媒体服务器连接

**接口**: `POST /api/servers/media/:id/test`

**响应示例**:
```json
{ "success": true, "message": "连接测试成功", "latency_ms": 120 }
```

### 7. 临时测试媒体服务器连接

**接口**: `POST /api/servers/media/test`

**请求体**: 与创建相同（不保存）

**响应**: `ConnectionTestResult`

---

## 文件浏览

### 1. 获取目录列表

**接口**: `GET /api/files/directories`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | 否 | 目录路径（默认 `/`）|
| `mode` | string | 否 | `local` / `api` |
| `type` | string | 否 | `clouddrive2` / `openlist`（仅 mode=api）|
| `host` | string | 否 | 远程主机（仅 mode=api）|
| `port` | string | 否 | 远程端口（仅 mode=api）|
| `apiKey` | string | 否 | 认证密钥（可选）|

**响应示例**:
```json
{
  "path": "/",
  "directories": ["movie", "tv"]
}
```

### 2. 获取文件列表

**接口**: `POST /api/files/list`

**请求体**:
```json
{
  "server_id": 1,
  "path": "/",
  "recursive": false,
  "max_depth": 5
}
```

**响应示例**:
```json
{
  "server_id": 1,
  "path": "/",
  "recursive": false,
  "count": 10,
  "files": []
}
```

---

## 任务管理

### 1. 获取任务列表

**接口**: `GET /api/jobs`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | 否 | 页码（默认1）|
| `page_size` | int | 否 | 每页数量（默认50，最大200）|
| `name` | string | 否 | 名称模糊搜索 |
| `enabled` | string | 否 | `true` / `false` |
| `watch_mode` | string | 否 | `local` / `api` |
| `status` | string | 否 | `idle` / `running` / `error` |
| `data_server_id` | int | 否 | 数据服务器ID |
| `media_server_id` | int | 否 | 媒体服务器ID |

**响应示例**:
```json
{ "jobs": [], "total": 0, "page": 1, "page_size": 50 }
```

### 2. 创建任务

**接口**: `POST /api/jobs`

**请求体**:
```json
{
  "name": "任务A",
  "enabled": true,
  "cron": "0 */6 * * *",
  "watch_mode": "local",
  "source_path": "/media",
  "target_path": "/strm",
  "strm_path": "/media",
  "data_server_id": 1,
  "media_server_id": 1,
  "options": "{}"
}
```

**说明**:
- `watch_mode` 必填，枚举：`local` / `api`。
- `watch_mode=api` 时必须指定 `data_server_id`。
- `options` 需为合法 JSON 字符串。

### 3. 获取任务详情

**接口**: `GET /api/jobs/:id`

**响应**: `{"job": { ... }}`

### 4. 更新任务

**接口**: `PUT /api/jobs/:id`

**请求体**: 与创建相同

**响应**: `{"job": { ... }}`

### 5. 删除任务

**接口**: `DELETE /api/jobs/:id`

**响应**:
```json
{ "message": "删除成功" }
```

### 6. 手动触发任务执行

**接口**: `POST /api/jobs/:id/run`

**响应示例**:
```json
{ "task_run": { "id": 1, "job_id": 1, "status": "pending" } }
```

### 7. 停止任务

**接口**: `POST /api/jobs/:id/stop`

**响应示例**:
```json
{ "message": "已取消 1 个任务", "cancelled": 1, "task_runs": [] }
```

### 8. 启用/禁用任务

**接口**:
- `PUT /api/jobs/:id/enable`
- `PUT /api/jobs/:id/disable`

**响应**: `{"job": { ... }}`

---

## 执行记录

### 1. 获取执行记录列表

**接口**: `GET /api/runs`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | 否 | 页码（默认1）|
| `page_size` | int | 否 | 每页数量（默认50，最大200）|
| `job_id` | int | 否 | 任务ID过滤 |
| `status` | string | 否 | `running` / `completed` / `failed` / `cancelled` |

**响应示例**:
```json
{ "runs": [], "total": 0, "page": 1, "page_size": 50 }
```

### 2. 获取执行记录详情

**接口**: `GET /api/runs/:id`

**响应**: `{"run": { ... }}`

### 3. 取消执行中的任务

**接口**: `POST /api/runs/:id/cancel`

**响应**:
```json
{ "message": "任务已取消" }
```

### 4. 获取执行统计

**接口**: `GET /api/runs/stats`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `job_id` | int | 否 | 任务ID过滤 |
| `from` | string | 否 | 开始时间（ISO字符串）|
| `to` | string | 否 | 结束时间（ISO字符串）|

**响应示例**:
```json
{
  "total": 10,
  "completed": 6,
  "failed": 2,
  "cancelled": 1,
  "running": 1,
  "pending": 0
}
```
