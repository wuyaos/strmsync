# 单容器部署架构说明

## 架构设计

STRMSync 采用**单容器部署**方案，前后端在一个容器内运行。

```
┌─────────────────────────────────────────────────────────────┐
│                     Docker Container                         │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              Go Binary (strmsync)                       │ │
│  │                                                         │ │
│  │  ┌──────────────────┐  ┌──────────────────────────┐   │ │
│  │  │  Gin HTTP Server │  │  Embedded Frontend       │   │ │
│  │  │                  │  │  (Vue 3 dist/)           │   │ │
│  │  │  /api/*  ────────┼──┼──> API Handlers         │   │ │
│  │  │  /*      ────────┼──┼──> Static File Server   │   │ │
│  │  └──────────────────┘  └──────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Port 3000                                                   │
└─────────────────────────────────────────────────────────────┘
```

---

## 实现方式

### 1. 前端嵌入

使用 Go 1.16+ 的 `embed` 特性将前端静态文件嵌入到二进制文件中。

```go
// backend/cmd/server/main.go
package main

import (
    "embed"
    "github.com/gin-gonic/gin"
    "io/fs"
    "net/http"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
    r := gin.Default()

    // API 路由
    api := r.Group("/api")
    {
        api.GET("/health", healthHandler)
        api.GET("/sources", sourcesHandler)
        // ... 其他 API 路由
    }

    // 静态文件服务（前端）
    staticFS, _ := fs.Sub(staticFiles, "static")
    r.StaticFS("/assets", http.FS(staticFS))
    r.NoRoute(func(c *gin.Context) {
        data, _ := staticFiles.ReadFile("static/index.html")
        c.Data(http.StatusOK, "text/html; charset=utf-8", data)
    })

    r.Run(":3000")
}
```

---

### 2. 多阶段构建

**Dockerfile 构建流程**:

```
阶段 1: Node.js 构建前端
  - 安装 npm 依赖
  - 运行 npm run build
  - 生成 dist/ 目录

阶段 2: Go 构建后端
  - 复制前端 dist/ 到 backend/static/
  - 编译 Go 二进制（启用 embed）
  - 前端文件被嵌入到二进制中

阶段 3: 最小运行环境
  - 仅包含编译后的二进制文件
  - 镜像大小 ~30MB
```

---

### 3. 路由规则

| 路径 | 处理方式 | 说明 |
|------|----------|------|
| `/api/*` | Go API Handlers | 后端 API 接口 |
| `/assets/*` | 嵌入的静态文件 | JS/CSS/图片 |
| `/*` | index.html（SPA） | Vue Router（History 模式） |

**前端路由配置**（Vue Router）:
```javascript
// frontend/src/router/index.js
const router = createRouter({
  history: createWebHistory('/'),  // History 模式
  routes: [
    { path: '/', component: Dashboard },
    { path: '/sources', component: Sources },
    // ...
  ]
})
```

**后端路由配置**（Gin）:
```go
// 所有非 API 路径返回 index.html（SPA）
r.NoRoute(func(c *gin.Context) {
    if strings.HasPrefix(c.Request.URL.Path, "/api") {
        c.JSON(404, gin.H{"error": "API not found"})
        return
    }
    data, _ := staticFiles.ReadFile("static/index.html")
    c.Data(200, "text/html; charset=utf-8", data)
})
```

---

## 优势

### 1. 简化部署
- ✅ 单个 Docker 镜像
- ✅ 单个容器运行
- ✅ 无需 Nginx 或其他反向代理
- ✅ 端口映射简单（只需映射一个端口）

### 2. 性能优异
- ✅ Go embed 性能接近原生文件系统
- ✅ 无额外网络开销（无需前后端跨容器通信）
- ✅ 静态文件直接从内存提供

### 3. 管理简便
- ✅ 统一的日志输出
- ✅ 统一的健康检查
- ✅ 统一的环境变量配置
- ✅ 便于调试和监控

### 4. 镜像精简
- ✅ 镜像大小 ~30MB（多阶段构建）
- ✅ 仅包含必要的运行时依赖
- ✅ 启动速度快

---

## 部署示例

### docker-compose.yml

```yaml
version: '3.8'

services:
  strmsync:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: strmsync
    ports:
      - "3000:3000"    # 单个端口，同时提供前端和 API
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
    environment:
      - PORT=3000
      - TZ=Asia/Shanghai
    restart: unless-stopped
```

### 访问方式

- **Web 界面**: http://localhost:3000/
- **API 接口**: http://localhost:3000/api/health
- **静态资源**: http://localhost:3000/assets/logo.png

---

## 开发模式

开发时前后端仍然分离运行：

**后端**（监听 3001 端口）:
```bash
cd backend
go run cmd/server/main.go
```

**前端**（开发服务器 + 代理）:
```bash
cd frontend
npm run dev
```

**前端配置代理**（vite.config.js）:
```javascript
export default {
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:3001',
        changeOrigin: true
      }
    }
  }
}
```

---

## 构建和部署

```bash
# 构建镜像
docker-compose build

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 访问
open http://localhost:3000
```

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 前端构建 | Vue 3 + Vite + Element Plus |
| 前端嵌入 | Go embed (Go 1.16+) |
| 后端框架 | Gin |
| HTTP 服务 | Gin（同时提供 API 和静态文件） |
| 容器化 | Docker 多阶段构建 |

---

## 对比传统方案

| 特性 | 单容器（本方案） | 双容器（Nginx + Go） |
|------|-----------------|---------------------|
| 容器数量 | 1 | 2 |
| 镜像大小 | ~30MB | ~150MB+ |
| 端口数量 | 1 | 2 |
| 反向代理 | 无需 | Nginx |
| 配置复杂度 | 低 | 中 |
| 网络延迟 | 最低 | 稍高（跨容器） |
| 推荐场景 | ✅ 中小型应用 | 大型应用 |

---

**文档版本**: 1.0.0
**最后更新**: 2024-02-16
**作者**: STRMSync Team
