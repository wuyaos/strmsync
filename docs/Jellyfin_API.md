# Jellyfin API 文档

## 基础信息
- **协议**: HTTP REST
- **默认地址**: http://localhost:8096
- **官方文档**: https://api.jellyfin.org/
- **API 版本**: 10.8+

---

## 认证方式

### 方式 1: API Key（推荐）

**创建 API Key**:
1. 以管理员身份登录 Jellyfin
2. Dashboard -> Advanced -> API Keys
3. 点击 + 按钮，输入 API Key 名称

**使用 API Key**:

**方法 A**: 请求头（推荐）
```http
GET /System/Info
Authorization: MediaBrowser Token="YOUR_API_KEY"
```

完整格式（可选参数）：
```http
Authorization: MediaBrowser Token="YOUR_API_KEY", Client="MyApp", Device="Desktop", DeviceId="unique_id", Version="1.0"
```

**方法 B**: 查询参数
```http
GET /System/Info?ApiKey=YOUR_API_KEY
```

**方法 C**: X-Emby-Token 头（兼容）
```http
GET /System/Info
X-Emby-Token: YOUR_API_KEY
```

---

### 方式 2: 用户名密码

**登录获取 Token**:
```http
POST /Users/AuthenticateByName
Content-Type: application/json
Authorization: MediaBrowser Client="MyApp", Device="Desktop", DeviceId="xxx", Version="1.0"

{
  "Username": "admin",
  "Pw": "password"
}
```

**响应**:
```json
{
  "User": {...},
  "SessionInfo": {...},
  "AccessToken": "your-access-token"
}
```

---

## 媒体库刷新 API

### 方法 1: 刷新特定库项目

**端点**: `POST /Items/{ItemId}/Refresh`

**请求**:
```http
POST /Items/{ItemId}/Refresh?Recursive=true&ImageRefreshMode=Default&MetadataRefreshMode=Default
Authorization: MediaBrowser Token="YOUR_API_KEY"
```

**查询参数**:
- `Recursive`: 是否递归刷新（true/false）
- `ImageRefreshMode`: 图片刷新模式（Default/FullRefresh/ValidationOnly）
- `MetadataRefreshMode`: 元数据刷新模式（Default/FullRefresh/ValidationOnly）
- `ReplaceAllImages`: 是否替换所有图片（true/false）
- `ReplaceAllMetadata`: 是否替换所有元数据（true/false）

**cURL 示例**:
```bash
curl -X POST "http://localhost:8096/Items/{ItemId}/Refresh?Recursive=true&ImageRefreshMode=Default&MetadataRefreshMode=Default" \
  -H "Authorization: MediaBrowser Token=\"YOUR_API_KEY\""
```

**ItemId 获取**:
可通过 `/Items` 接口查询库 ID。

---

### 方法 2: 刷新特定路径

**端点**: `POST /Library/Media/Updated`

**请求**:
```http
POST /Library/Media/Updated
Authorization: MediaBrowser Token="YOUR_API_KEY"
Content-Type: application/json

{
  "Updates": [
    {
      "Path": "/media/library/Movies/Action",
      "UpdateType": "Created"
    }
  ]
}
```

**UpdateType 值**:
- `Created`: 新增
- `Modified`: 修改
- `Deleted`: 删除

**cURL 示例**:
```bash
curl -X POST "http://localhost:8096/Library/Media/Updated" \
  -H "Authorization: MediaBrowser Token=\"YOUR_API_KEY\"" \
  -H "Content-Type: application/json" \
  -d '{
    "Updates": [
      {"Path": "/media/library/Movies/Action", "UpdateType": "Created"}
    ]
  }'
```

---

### 方法 3: 触发计划任务（扫描所有库）

**端点**: `POST /ScheduledTasks/Running/{TaskId}`

**步骤**:

**1. 获取扫描任务 ID**:
```bash
curl -s -H "Authorization: MediaBrowser Token=\"YOUR_API_KEY\"" \
  "http://localhost:8096/ScheduledTasks?IsEnabled=true" | \
  jq '.[] | select(.Name=="Scan Media Library") | .Id'
```

**2. 启动扫描**:
```bash
curl -X POST -H "Authorization: MediaBrowser Token=\"YOUR_API_KEY\"" \
  "http://localhost:8096/ScheduledTasks/Running/{TaskId}"
```

**响应**: HTTP 204 No Content（成功）

---

## Go 客户端示例

```go
package jellyfin

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type JellyfinClient struct {
    BaseURL string
    APIKey  string
    client  *http.Client
}

func NewJellyfinClient(baseURL, apiKey string) *JellyfinClient {
    return &JellyfinClient{
        BaseURL: baseURL,
        APIKey:  apiKey,
        client:  &http.Client{Timeout: 10 * time.Second},
    }
}

// RefreshPath 刷新特定路径
func (c *JellyfinClient) RefreshPath(path string) error {
    url := fmt.Sprintf("%s/Library/Media/Updated", c.BaseURL)

    body := map[string]interface{}{
        "Updates": []map[string]string{
            {
                "Path":       path,
                "UpdateType": "Created",
            },
        },
    }

    jsonData, _ := json.Marshal(body)
    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("MediaBrowser Token=\"%s\"", c.APIKey))

    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 204 && resp.StatusCode != 200 {
        return fmt.Errorf("refresh failed: status %d", resp.StatusCode)
    }

    return nil
}

// RefreshItem 刷新特定库项目
func (c *JellyfinClient) RefreshItem(itemID string) error {
    url := fmt.Sprintf("%s/Items/%s/Refresh?Recursive=true&ImageRefreshMode=Default&MetadataRefreshMode=Default",
        c.BaseURL, itemID)

    req, _ := http.NewRequest("POST", url, nil)
    req.Header.Set("Authorization", fmt.Sprintf("MediaBrowser Token=\"%s\"", c.APIKey))

    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 204 && resp.StatusCode != 200 {
        return fmt.Errorf("refresh failed: status %d", resp.StatusCode)
    }

    return nil
}
```

---

## STRMSync 集成建议

### 推荐使用"路径刷新"API

**优点**:
- 只刷新变更的目录，性能更好
- 支持批量刷新多个路径
- 避免全量扫描耗时

**实现示例**:
```go
func (n *Notifier) NotifyJellyfinPathChange(path string) error {
    if n.jellyfin != nil && n.jellyfin.Enabled {
        return n.jellyfin.RefreshPath(path)
    }
    return nil
}
```

---

## 与 Emby 的对比

| 特性 | Emby | Jellyfin |
|------|------|----------|
| 默认端口 | 8096 | 8096 |
| API Key 认证 | `api_key` 参数或 `X-Emby-Token` 头 | `Authorization: MediaBrowser Token` 头 |
| 全量刷新 | `POST /emby/Library/Refresh` | `POST /ScheduledTasks/Running/{TaskId}` |
| 路径刷新 | `POST /emby/Library/Media/Updated` | `POST /Library/Media/Updated` |
| 项目刷新 | `POST /emby/Items/{ItemId}/Refresh` | `POST /Items/{ItemId}/Refresh` |
| 兼容性 | - | 兼容 Emby API（`X-Emby-Token` 头） |

---

**文档版本**: 1.0.0
**最后更新**: 2026-02-20
**作者**: STRMSync Team
