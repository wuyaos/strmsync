# OpenList REST API 文档

## 基础信息

- **协议**: HTTP REST
- **数据格式**: JSON
- **默认地址**: http://localhost:5244
- **官方文档**: https://fox.oplist.org/
- **Apifox 文档**: https://openlist.apifox.cn/
- **项目主页**: https://doc.oplist.org/

---

## 认证方式

OpenList 使用 **JWT Bearer Token** 进行认证。

### 登录获取 Token

#### POST /api/auth/login

**描述**: 通过用户名和密码进行身份验证，返回 JWT Token。Token 默认 48 小时过期。

**请求**:
```http
POST /api/auth/login HTTP/1.1
Content-Type: application/json

{
  "username": "admin",
  "password": "your_password",
  "otp_code": ""  // 可选，双因素认证码
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

**错误响应** (HTTP 400):
```json
{
  "code": 400,
  "message": "Invalid request parameters",
  "data": null
}
```

---

### 使用 Token

在所有需要认证的请求中添加 Header：

```http
Authorization: <token>
```

**注意**: OpenList **不需要** "Bearer" 前缀。

---

## 核心 API

### 1. 文件系统操作

#### 1.1 列出文件目录

**POST /api/fs/list**

**请求 Header**:
```http
Authorization: <token>
Content-Type: application/json
```

**请求 Body**:
```json
{
  "path": "/Movies",         // 目录路径
  "password": "",            // 目录密码（如果设置）
  "page": 1,                 // 页码
  "per_page": 100,           // 每页数量，0 表示不分页
  "refresh": false           // 是否强制刷新缓存
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "content": [
      {
        "name": "Movie Name (2023).mkv",
        "size": 12500000000,              // 字节
        "is_dir": false,
        "modified": "2024-05-17T13:47:55.4174917+08:00",
        "created": "2024-05-17T13:47:47.5725906+08:00",
        "sign": "",                       // 签名
        "thumb": "",                      // 缩略图 URL
        "type": 4,                        // 文件类型
        "hashinfo": "null",
        "hash_info": null
      }
    ],
    "total": 85320,                       // 总文件数
    "readme": "",                         // README 内容
    "header": "",                         // 头部信息
    "write": true,                        // 是否可写
    "provider": "Local"                   // 存储提供商
  }
}
```

---

#### 1.2 获取文件/目录信息

**POST /api/fs/get**

**请求 Body**:
```json
{
  "path": "/Movies/Action/Movie.mkv",
  "password": "",
  "page": 1,
  "per_page": 0,
  "refresh": false
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "name": "Movie.mkv",
    "size": 12500000000,
    "is_dir": false,
    "modified": "2024-05-17T16:05:36.4651534+08:00",
    "created": "2024-05-17T16:05:29.2001008+08:00",
    "sign": "",
    "thumb": "",
    "type": 4,
    "raw_url": "http://localhost:5244/d/Movies/Action/Movie.mkv",  // 下载 URL
    "readme": "",
    "provider": "Local",
    "related": []
  }
}
```

**关键字段**:
- `raw_url`: 文件的直接下载链接（**用于生成 STRM 文件**）

---

#### 1.3 搜索文件或文件夹

**POST /api/fs/search**

**请求 Body**:
```json
{
  "parent": "/Movies",       // 搜索目录
  "keywords": "Action",      // 关键词
  "scope": 0,                // 搜索类型：0-全部，1-文件夹，2-文件
  "page": 1,
  "per_page": 100,
  "password": ""
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "content": [
      {
        "parent": "/Movies",
        "name": "Action",
        "is_dir": true,
        "size": 0,
        "type": 1
      }
    ],
    "total": 1
  }
}
```

---

#### 1.4 新建文件夹

**POST /api/fs/mkdir**

**请求 Body**:
```json
{
  "path": "/Movies/NewFolder"
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": null
}
```

---

#### 1.5 重命名文件

**POST /api/fs/rename**

**请求 Body**:
```json
{
  "path": "/Movies/OldName.mkv",
  "name": "NewName.mkv"       // 新文件名（不含路径）
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": null
}
```

---

#### 1.6 批量重命名

**POST /api/fs/batch_rename**

**请求 Body**:
```json
{
  "src_dir": "/Movies",
  "rename_objects": [
    {
      "src_name": "Movie1.mkv",
      "new_name": "Movie1_Renamed.mkv"
    },
    {
      "src_name": "Movie2.mkv",
      "new_name": "Movie2_Renamed.mkv"
    }
  ]
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": null
}
```

---

#### 1.7 移动文件

**POST /api/fs/move**

**请求 Body**:
```json
{
  "src_dir": "/Movies/Source",
  "dst_dir": "/Movies/Destination",
  "names": ["Movie1.mkv", "Movie2.mkv"]
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": null
}
```

---

#### 1.8 复制文件

**POST /api/fs/copy**

**请求 Body**:
```json
{
  "src_dir": "/Movies/Source",
  "dst_dir": "/Movies/Destination",
  "names": ["Movie1.mkv", "Movie2.mkv"]
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": null
}
```

---

#### 1.9 删除文件或文件夹

**POST /api/fs/remove**

**请求 Body**:
```json
{
  "dir": "/Movies",
  "names": ["Movie1.mkv", "OldFolder"]
}
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": null
}
```

---

### 2. 文件上传

#### 2.1 表单上传

**PUT /api/fs/form**

**请求 Header**:
```http
Authorization: <token>
Content-Type: multipart/form-data; boundary=...
Content-Length: 12500000
File-Path: %2FMovies%2FMovie.mkv  // URL 编码的完整路径
As-Task: true                      // 可选，是否作为后台任务
```

**请求 Body**:
```
multipart/form-data 格式的文件数据
```

**响应** (HTTP 200):
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "task": {
      "id": "sdH2LbjyWRk",
      "name": "upload Movie.mkv to [/Movies]",
      "state": 0,
      "status": "uploading",
      "progress": 0,
      "error": ""
    }
  }
}
```

---

#### 2.2 流式上传

**PUT /api/fs/put**

**请求 Header**:
```http
Authorization: <token>
Content-Type: application/octet-stream
Content-Length: 12500000
File-Path: %2FMovies%2FMovie.mkv  // URL 编码的完整路径
As-Task: true                      // 可选
```

**请求 Body**:
```
二进制文件数据流
```

**响应**: 同表单上传

**任务状态**:
- `state`: 0-排队，1-运行中，2-完成，3-失败，4-取消
- `status`: "pending", "uploading", "completed", "failed"
- `progress`: 0-100

---

### 3. 文件下载

#### 3.1 直接下载

**GET /d/{路径}**

**示例**:
```
GET /d/Movies/Action/Movie.mkv HTTP/1.1
```

**响应**: 文件二进制数据流（HTTP 200）

**说明**:
- 无需 Token 认证（取决于目录权限设置）
- 支持 Range 请求（断点续传）
- **STRM 文件内容就是这个 URL**

---

#### 3.2 获取下载链接

使用 `/api/fs/get` 接口获取文件信息，响应中的 `raw_url` 字段即为下载链接。

**示例**:
```json
{
  "data": {
    "name": "Movie.mkv",
    "raw_url": "http://localhost:5244/d/Movies/Movie.mkv"
  }
}
```

---

## 错误处理

### 通用错误格式

```json
{
  "code": 400,
  "message": "错误描述",
  "data": null
}
```

### 常见错误码

| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未授权（Token 无效或过期） |
| 403 | 权限不足 |
| 404 | 文件或目录不存在 |
| 500 | 服务器内部错误 |

### Go 客户端错误处理

```go
type APIResponse struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
}

func CheckError(resp *APIResponse) error {
    if resp.Code != 200 {
        return fmt.Errorf("API error: %s (code: %d)", resp.Message, resp.Code)
    }
    return nil
}
```

---

## 限流和性能

### 建议

1. **批量操作**: 使用批量接口（如 `batch_rename`）而非循环调用单个接口
2. **缓存策略**:
   - 首次扫描: `refresh=false`
   - 定时刷新: 每 5-10 分钟 `refresh=true`
3. **并发控制**: 建议限制并发请求数 ≤ 10
4. **分页策略**:
   - 小目录（< 1000 文件）: `per_page=0`
   - 大目录（> 1000 文件）: `per_page=100`

---

## Go 客户端示例

### 1. 初始化客户端

```go
package openlist

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
)

type Client struct {
    BaseURL string
    Token   string
    client  *http.Client
}

func NewClient(baseURL, username, password string) (*Client, error) {
    c := &Client{
        BaseURL: baseURL,
        client:  &http.Client{Timeout: 30 * time.Second},
    }

    // 登录获取 Token
    token, err := c.Login(username, password)
    if err != nil {
        return nil, err
    }

    c.Token = token
    return c, nil
}

func (c *Client) Login(username, password string) (string, error) {
    data := map[string]string{
        "username": username,
        "password": password,
    }

    resp, err := c.request("POST", "/api/auth/login", data, false)
    if err != nil {
        return "", err
    }

    var result struct {
        Code    int    `json:"code"`
        Message string `json:"message"`
        Data    struct {
            Token string `json:"token"`
        } `json:"data"`
    }

    if err := json.Unmarshal(resp, &result); err != nil {
        return "", err
    }

    if result.Code != 200 {
        return "", fmt.Errorf("login failed: %s", result.Message)
    }

    return result.Data.Token, nil
}

func (c *Client) request(method, path string, body interface{}, auth bool) ([]byte, error) {
    var reqBody io.Reader
    if body != nil {
        jsonData, _ := json.Marshal(body)
        reqBody = bytes.NewBuffer(jsonData)
    }

    req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Content-Type", "application/json")
    if auth && c.Token != "" {
        req.Header.Set("Authorization", c.Token)
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

---

### 2. 列出文件

```go
type FileInfo struct {
    Name     string `json:"name"`
    Size     int64  `json:"size"`
    IsDir    bool   `json:"is_dir"`
    Modified string `json:"modified"`
    Created  string `json:"created"`
}

type ListResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    struct {
        Content  []FileInfo `json:"content"`
        Total    int        `json:"total"`
        Provider string     `json:"provider"`
    } `json:"data"`
}

func (c *Client) ListFiles(path string, page, perPage int, refresh bool) ([]FileInfo, error) {
    data := map[string]interface{}{
        "path":     path,
        "password": "",
        "page":     page,
        "per_page": perPage,
        "refresh":  refresh,
    }

    resp, err := c.request("POST", "/api/fs/list", data, true)
    if err != nil {
        return nil, err
    }

    var result ListResponse
    if err := json.Unmarshal(resp, &result); err != nil {
        return nil, err
    }

    if result.Code != 200 {
        return nil, fmt.Errorf("list failed: %s", result.Message)
    }

    return result.Data.Content, nil
}
```

---

### 3. 获取文件信息

```go
type FileDetail struct {
    Name     string `json:"name"`
    Size     int64  `json:"size"`
    IsDir    bool   `json:"is_dir"`
    Modified string `json:"modified"`
    RawURL   string `json:"raw_url"`  // 下载 URL
    Provider string `json:"provider"`
}

func (c *Client) GetFileInfo(path string) (*FileDetail, error) {
    data := map[string]interface{}{
        "path":     path,
        "password": "",
    }

    resp, err := c.request("POST", "/api/fs/get", data, true)
    if err != nil {
        return nil, err
    }

    var result struct {
        Code    int        `json:"code"`
        Message string     `json:"message"`
        Data    FileDetail `json:"data"`
    }

    if err := json.Unmarshal(resp, &result); err != nil {
        return nil, err
    }

    if result.Code != 200 {
        return nil, fmt.Errorf("get file info failed: %s", result.Message)
    }

    return &result.Data, nil
}
```

---

### 4. 搜索文件

```go
func (c *Client) SearchFiles(parent, keywords string, scope int) ([]FileInfo, error) {
    data := map[string]interface{}{
        "parent":   parent,
        "keywords": keywords,
        "scope":    scope,  // 0-全部，1-文件夹，2-文件
        "page":     1,
        "per_page": 100,
        "password": "",
    }

    resp, err := c.request("POST", "/api/fs/search", data, true)
    if err != nil {
        return nil, err
    }

    var result struct {
        Code    int    `json:"code"`
        Message string `json:"message"`
        Data    struct {
            Content []FileInfo `json:"content"`
            Total   int        `json:"total"`
        } `json:"data"`
    }

    if err := json.Unmarshal(resp, &result); err != nil {
        return nil, err
    }

    if result.Code != 200 {
        return nil, fmt.Errorf("search failed: %s", result.Message)
    }

    return result.Data.Content, nil
}
```

---

## STRMSync 集成方案

### 配置示例

```yaml
sources:
  - name: "OpenList 电影"
    type: openlist
    enabled: true
    config:
      api_url: http://localhost:5244
      username: admin
      password: your_password
      base_path: /Movies
    mapping:
      source_prefix: /Movies
      target_prefix: /media/library/Movies
```

### STRM 文件生成

```go
func GenerateSTRM(openlistURL, filePath string) string {
    // OpenList 下载 URL 格式: http://host/d/{路径}
    downloadURL := fmt.Sprintf("%s/d%s", openlistURL, filePath)
    return downloadURL
}

// 示例
// 输入: http://localhost:5244, /Movies/Action/Movie.mkv
// 输出: http://localhost:5244/d/Movies/Action/Movie.mkv
```

### 扫描流程

```go
func ScanOpenListDirectory(client *openlist.Client, path string) error {
    files, err := client.ListFiles(path, 1, 0, false)
    if err != nil {
        return err
    }

    for _, file := range files {
        if file.IsDir {
            // 递归扫描子目录
            subPath := filepath.Join(path, file.Name)
            ScanOpenListDirectory(client, subPath)
        } else if isVideoFile(file.Name) {
            // 处理视频文件
            strmPath := generateSTRMPath(path, file.Name)
            strmContent := generateSTRMContent(path, file.Name)
            writeSTRMFile(strmPath, strmContent)
        }
    }

    return nil
}

func isVideoFile(name string) bool {
    exts := []string{".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".m4v"}
    ext := strings.ToLower(filepath.Ext(name))
    for _, e := range exts {
        if ext == e {
            return true
        }
    }
    return false
}
```

---

## 参考资源

- **官方文档**: https://doc.oplist.org/
- **API 文档**: https://fox.oplist.org/
- **Apifox 平台**: https://openlist.apifox.cn/
- **GitHub**: https://github.com/OpenListTeam/OpenList
- **Go 客户端**: https://github.com/littleboss01/openlistClient

---

**文档版本**: 1.0.0
**最后更新**: 2024-02-16
**作者**: STRMSync Team
