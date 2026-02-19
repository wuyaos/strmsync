# STRMSync HTTP API 文档

> 后端HTTP API详细说明

## 基础信息

- **基础URL**: `http://localhost:6754/api`
- **协议**: HTTP/1.1
- **认证**: 无（待实现）
- **Content-Type**: `application/json`

## 响应格式

### 列表响应

所有列表接口遵循统一的响应格式：

```json
{
  "servers": [...],     // 或 "items"、"list"、"jobs"、"runs" 等
  "total": 100,         // 总数
  "page": 1,            // 当前页
  "page_size": 10       // 每页数量
}
```

前端使用 `normalizeListResponse` 函数自动兼容以下字段：
- `data.items` / `data.list` / `items` / `list` / `data`
- `data.total` / `total` / `meta.total` / `pagination.total`

### 错误响应

```json
{
  "error": "错误描述信息"
}
```

常见HTTP状态码：
- `200`: 成功
- `201`: 创建成功
- `400`: 请求参数错误
- `404`: 资源不存在
- `500`: 服务器内部错误

---

## 服务器管理

### 1. 获取服务器列表

**接口**: `GET /api/servers`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 否 | 服务器类型（`data` / `media`）|
| `enabled` | string | 否 | 启用状态（`"true"` / `"false"`）|
| `keyword` | string | 否 | 搜索关键词（名称/主机）|
| `page` | int | 否 | 页码（默认1）|
| `pageSize` | int | 否 | 每页数量（默认10）|

**响应示例**:
```json
{
  "servers": [
    {
      "id": 1,
      "name": "CloudDrive2主服务器",
      "type": "clouddrive2",
      "host": "192.168.123.179",
      "port": 19798,
      "api_key": "",
      "options": "{}",
      "enabled": true,
      "uid": "cd2-abc123",
      "created_at": "2026-02-19T10:00:00Z",
      "updated_at": "2026-02-19T10:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 10
}
```

### 2. 创建服务器

**接口**: `POST /api/servers`

**请求体**:
```json
{
  "name": "CloudDrive2主服务器",
  "type": "clouddrive2",
  "host": "192.168.123.179",
  "port": 19798,
  "api_key": "",
  "options": "{}",
  "enabled": true
}
```

**字段说明**:
- `name`: 服务器名称（必填）
- `type`: 服务器类型（必填）
  - 数据服务器: `local` / `clouddrive2` / `openlist` / `webdav`
  - 媒体服务器: `emby` / `jellyfin` / `plex`
- `host`: 主机地址（必填）
- `port`: 端口号（必填，1-65535）
- `api_key`: API密钥（可选）
- `options`: JSON格式的额外配置（可选）
- `enabled`: 是否启用（默认true）

**响应**: 返回创建的服务器对象

### 3. 获取服务器详情

**接口**: `GET /api/servers/:id`

**响应**: 返回单个服务器对象

### 4. 更新服务器

**接口**: `PUT /api/servers/:id`

**请求体**: 与创建服务器相同

**响应**: 返回更新后的服务器对象

### 5. 删除服务器

**接口**: `DELETE /api/servers/:id`

**响应**: `204 No Content`

### 6. 测试服务器连接

**接口**: `POST /api/servers/:id/test`

**响应示例**:
```json
{
  "success": true,
  "message": "连接测试成功"
}
```

---

## 任务管理

### 1. 获取任务列表

**接口**: `GET /api/jobs`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `enabled` | string | 否 | 启用状态（`"true"` / `"false"`）|
| `keyword` | string | 否 | 搜索关键词（任务名称）|
| `page` | int | 否 | 页码（默认1）|
| `pageSize` | int | 否 | 每页数量（默认10）|

**响应示例**:
```json
{
  "jobs": [
    {
      "id": 1,
      "name": "电影库同步任务",
      "enabled": true,
      "cron": "0 */6 * * *",
      "watch_mode": "cron",
      "source_path": "/media/movies",
      "target_path": "/strm/movies",
      "strm_path": "http://127.0.0.1:19798/media/movies",
      "data_server_id": 1,
      "media_server_id": 1,
      "options": "{}",
      "status": "idle",
      "last_run_at": "2026-02-19T10:00:00Z",
      "created_at": "2026-02-19T09:00:00Z",
      "updated_at": "2026-02-19T10:00:00Z",
      "data_server": {
        "id": 1,
        "name": "CloudDrive2主服务器",
        "type": "clouddrive2"
      },
      "media_server": {
        "id": 1,
        "name": "Emby服务器",
        "type": "emby"
      }
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 10
}
```

### 2. 创建任务

**接口**: `POST /api/jobs`

**请求体**:
```json
{
  "name": "电影库同步任务",
  "enabled": true,
  "cron": "0 */6 * * *",
  "watch_mode": "cron",
  "source_path": "/media/movies",
  "target_path": "/strm/movies",
  "strm_path": "http://127.0.0.1:19798/media/movies",
  "data_server_id": 1,
  "media_server_id": 1,
  "options": "{}"
}
```

**字段说明**:
- `name`: 任务名称（必填）
- `enabled`: 是否启用（默认true）
- `cron`: Cron表达式，如 `"0 */6 * * *"` 表示每6小时执行一次（必填）
- `watch_mode`: 监控模式，固定为 `"cron"`（默认）
- `source_path`: 数据源路径（必填）
- `target_path`: STRM文件输出路径（必填）
- `strm_path`: STRM文件中的媒体URL路径（必填）
- `data_server_id`: 数据服务器ID（必填）
- `media_server_id`: 媒体服务器ID（必填）
- `options`: JSON格式的额外配置（可选）

**响应**: 返回创建的任务对象

### 3. 获取任务详情

**接口**: `GET /api/jobs/:id`

**响应**: 返回单个任务对象

### 4. 更新任务

**接口**: `PUT /api/jobs/:id`

**请求体**: 与创建任务相同

**响应**: 返回更新后的任务对象

### 5. 删除任务

**接口**: `DELETE /api/jobs/:id`

**响应**: `204 No Content`

### 6. 触发任务执行

**接口**: `POST /api/jobs/:id/trigger`

**响应示例**:
```json
{
  "run_id": 123,
  "message": "任务已触发执行"
}
```

### 7. 启用任务

**接口**: `POST /api/jobs/:id/enable`

**响应**: 返回更新后的任务对象

### 8. 禁用任务

**接口**: `POST /api/jobs/:id/disable`

**响应**: 返回更新后的任务对象

---

## 运行记录

### 1. 获取运行记录列表

**接口**: `GET /api/runs`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `status` | string | 否 | 运行状态（`pending` / `running` / `completed` / `failed` / `cancelled`）|
| `from` | string | 否 | 开始时间（ISO 8601格式）|
| `to` | string | 否 | 结束时间（ISO 8601格式）|
| `page` | int | 否 | 页码（默认1）|
| `pageSize` | int | 否 | 每页数量（默认10）|

**响应示例**:
```json
{
  "runs": [
    {
      "id": 123,
      "job_id": 1,
      "job_name": "电影库同步任务",
      "status": "completed",
      "started_at": "2026-02-19T10:00:00Z",
      "finished_at": "2026-02-19T10:05:30Z",
      "error_message": "",
      "stats": {
        "processed": 150,
        "created": 20,
        "updated": 10,
        "deleted": 5,
        "skipped": 115
      },
      "created_at": "2026-02-19T10:00:00Z",
      "updated_at": "2026-02-19T10:05:30Z",
      "job": {
        "id": 1,
        "name": "电影库同步任务"
      }
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 10
}
```

**状态说明**:
- `pending`: 待执行
- `running`: 运行中
- `completed`: 已完成
- `failed`: 失败
- `cancelled`: 已取消

### 2. 获取运行记录详情

**接口**: `GET /api/runs/:id`

**响应**: 返回单个运行记录对象

### 3. 取消运行中的任务

**接口**: `POST /api/runs/:id/cancel`

**响应示例**:
```json
{
  "message": "任务已取消"
}
```

---

## 系统管理

### 1. 健康检查

**接口**: `GET /api/health`

**响应示例**:
```json
{
  "status": "healthy",
  "timestamp": "2026-02-19T10:00:00Z"
}
```

### 2. 获取日志

**接口**: `GET /api/logs`

**查询参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `level` | string | 否 | 日志级别（`debug` / `info` / `warn` / `error`）|
| `from` | string | 否 | 开始时间（ISO 8601格式）|
| `to` | string | 否 | 结束时间（ISO 8601格式）|
| `limit` | int | 否 | 返回条数（默认100）|

**响应示例**:
```json
{
  "logs": [
    {
      "timestamp": "2026-02-19T10:00:00Z",
      "level": "info",
      "message": "任务执行完成",
      "context": {
        "job_id": 1,
        "run_id": 123
      }
    }
  ],
  "total": 1
}
```

### 3. 获取系统设置

**接口**: `GET /api/settings`

**响应示例**:
```json
{
  "log_level": "info",
  "max_concurrent_jobs": 5,
  "database_path": "./data/strmsync.db"
}
```

### 4. 更新系统设置

**接口**: `PUT /api/settings`

**请求体**:
```json
{
  "log_level": "debug",
  "max_concurrent_jobs": 10
}
```

**响应**: 返回更新后的设置对象

---

## 前端集成说明

### 响应标准化

前端使用 `normalizeListResponse` 函数处理所有列表响应：

```javascript
import { normalizeListResponse } from '@/api/normalize'

const response = await getServerList(params)
const { list, total } = normalizeListResponse(response)
```

该函数自动处理：
- 不同的列表字段名称（items/list/data）
- 不同的total字段位置（data.total/meta.total/pagination.total）
- 直接返回数组的情况
- total字段缺失或为0的情况

### 错误处理

前端使用Axios拦截器统一处理错误：

```javascript
// frontend/src/api/request.js
// 请求错误显示 Element Plus ElMessage
// 响应错误根据状态码显示相应提示
```

### 时间格式

- 后端返回ISO 8601格式：`2026-02-19T10:00:00Z`
- 前端使用dayjs处理显示：`fromNow()` 相对时间

---

## 开发建议

1. **分页**: 所有列表接口建议添加分页参数
2. **过滤**: 支持按状态、时间范围、关键词过滤
3. **排序**: 默认按`created_at DESC`排序
4. **幂等性**: POST/PUT操作应支持重复调用
5. **错误信息**: 返回清晰的中文错误描述
