# Emby API 文档

## 基础信息
- **协议**: HTTP REST
- **默认地址**: http://localhost:8096
- **官方文档**: https://dev.emby.media/doc/restapi/
- **API 版本**: v3+

---

## 认证方式

### 方式 1: API Key（推荐）

**创建 API Key**:
1. 以管理员身份登录 Emby Server
2. 打开 Server Dashboard（Web UI）
3. 左侧菜单 -> Expert -> Advanced
4. 顶部导航 -> Security
5. 点击 Add 按钮，输入 API Key 名称

**使用 API Key**:

**方法 A**: 查询参数（推荐）
```http
GET /emby/System/Info?api_key=YOUR_API_KEY
```

**方法 B**: 请求头
```http
GET /emby/System/Info
X-Emby-Token: YOUR_API_KEY
```

---

### 方式 2: 用户名密码

**登录获取 Token**:
```http
POST /emby/Users/AuthenticateByName
Content-Type: application/json

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

**使用 Token**:
```http
GET /emby/System/Info
X-Emby-Token: your-access-token
```

---

## 媒体库刷新 API

### 全量刷新所有库

**端点**: `POST /emby/Library/Refresh`

**请求**:
```http
POST /emby/Library/Refresh?api_key=YOUR_API_KEY
Content-Length: 0
```

**注意**: 必须包含 `Content-Length: 0` 或 `-d ""` （curl），否则返回 411 Length Required。

**cURL 示例**:
```bash
curl -X POST "http://localhost:8096/emby/Library/Refresh?api_key=YOUR_API_KEY" -d ""
```

或使用请求头认证：
```bash
curl -X POST "http://localhost:8096/emby/Library/Refresh" \
  -H "X-Emby-Token: YOUR_API_KEY" \
  -H "Content-Length: 0"
```

**响应**: HTTP 204 No Content（成功）

---

### 刷新特定路径

**端点**: `POST /emby/Library/Media/Updated`

**请求**:
```http
POST /emby/Library/Media/Updated?api_key=YOUR_API_KEY
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
- `Created`: 新增文件
- `Modified`: 文件修改
- `Deleted`: 文件删除

**cURL 示例**:
```bash
curl -X POST "http://localhost:8096/emby/Library/Media/Updated?api_key=YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "Updates": [
      {"Path": "/media/library/Movies/Action", "UpdateType": "Created"}
    ]
  }'
```

---

## Go 客户端示例

```go
package emby

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type EmbyClient struct {
    BaseURL string
    APIKey  string
    client  *http.Client
}

func NewEmbyClient(baseURL, apiKey string) *EmbyClient {
    return &EmbyClient{
        BaseURL: baseURL,
        APIKey:  apiKey,
        client:  &http.Client{Timeout: 10 * time.Second},
    }
}

// RefreshLibrary 刷新所有媒体库
func (c *EmbyClient) RefreshLibrary() error {
    url := fmt.Sprintf("%s/emby/Library/Refresh?api_key=%s", c.BaseURL, c.APIKey)

    req, _ := http.NewRequest("POST", url, nil)
    req.Header.Set("Content-Length", "0")

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

// RefreshPath 刷新特定路径
func (c *EmbyClient) RefreshPath(path string) error {
    url := fmt.Sprintf("%s/emby/Library/Media/Updated?api_key=%s", c.BaseURL, c.APIKey)

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
func (n *Notifier) NotifyEmbyPathChange(path string) error {
    if n.emby != nil && n.emby.Enabled {
        return n.emby.RefreshPath(path)
    }
    return nil
}
```

---

**文档版本**: 1.0.0
**最后更新**: 2026-02-20
**作者**: STRMSync Team
