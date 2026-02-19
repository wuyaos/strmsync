# STRMSync 部署文档

> 生产环境部署指南

## 系统要求

| 组件 | 版本要求 | 说明 |
|------|----------|------|
| Go | 1.24+ | 后端编译和运行 |
| Node.js | 18+ | 前端构建（Vite 5要求）|
| SQLite | 3.x | 数据库（自动创建）|
| 操作系统 | Linux / macOS / Windows | 任意平台 |

---

## 环境变量

所有配置通过环境变量或 `.env` 文件指定。参考 `.env.example` 创建 `.env` 文件：

```bash
cp .env.example .env
```

### 配置项说明

| 变量 | 默认值 | 必填 | 说明 |
|------|--------|------|------|
| `PORT` | `6754` | 否 | HTTP服务端口 |
| `DB_PATH` | `app/data.db` | 否 | SQLite数据库文件路径 |
| `ENCRYPTION_KEY` | — | **是** | API密钥加密密钥（32字符随机字符串）|
| `TZ` | `Asia/Shanghai` | 否 | 时区（影响日志时间和Cron调度）|

> ⚠️ **安全警告**: 生产环境必须将 `ENCRYPTION_KEY` 修改为随机字符串，不得使用默认值。

### 生成安全的加密密钥

```bash
# Linux/macOS
openssl rand -hex 16

# 或使用Go
go run -e 'import "crypto/rand"; import "encoding/hex"; b := make([]byte, 16); rand.Read(b); fmt.Println(hex.EncodeToString(b))'
```

---

## 数据库

### 初始化

数据库在首次启动时**自动创建和迁移**，无需手动操作：

```bash
./server  # 首次运行自动创建 app/data.db
```

### 备份

```bash
# 备份SQLite数据库文件
cp app/data.db app/data.db.backup.$(date +%Y%m%d)
```

### 迁移

当前版本使用 GORM AutoMigrate，每次启动自动处理数据库结构变更（向前兼容）。

---

## 快速部署

### 方案一：直接运行（推荐开发/测试）

```bash
# 1. 克隆代码
git clone <repository_url>
cd strm

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 文件，修改 ENCRYPTION_KEY

# 3. 编译前端
cd frontend
npm install
npm run build
cd ..

# 4. 编译后端（前端构建产物会被嵌入）
cd backend
go build -o server ./cmd/server
cd ..

# 5. 启动服务
cd backend
./server
```

服务启动后访问: `http://localhost:6754`

### 方案二：使用启动脚本

```bash
# 启动（后台运行）
./scripts/start.sh

# 停止
./scripts/stop.sh

# 查看进程
ps aux | grep strmsync
```

### 方案三：Docker（待完善）

```bash
# 构建镜像
docker build -t strmsync .

# 运行容器
docker run -d \
  -p 6754:6754 \
  -v $(pwd)/data:/app/data \
  -e ENCRYPTION_KEY=your_random_key_here \
  -e TZ=Asia/Shanghai \
  strmsync
```

> 注意：Docker支持仍在完善中。

---

## 前端静态资源

前端构建产物通过Go的 `embed` 机制嵌入到二进制文件中，无需单独部署。

**构建流程**:
```bash
# 1. 构建前端（产物输出到 frontend/dist/）
cd frontend && npm run build

# 2. 编译后端（自动嵌入 frontend/dist/）
cd ../backend && go build -o server ./cmd/server

# 3. 单个可执行文件包含完整应用
./server  # 前后端均在 http://localhost:6754
```

---

## 反向代理配置

### Nginx

```nginx
server {
    listen 80;
    server_name your-domain.com;

    # 重定向到HTTPS（推荐生产环境）
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /path/to/certificate.crt;
    ssl_certificate_key /path/to/private.key;

    # 代理到STRMSync
    location / {
        proxy_pass http://127.0.0.1:6754;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket支持（如有需要）
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### Caddy

```caddy
your-domain.com {
    reverse_proxy localhost:6754
}
```

---

## 进程管理

### Systemd（Linux推荐）

创建 `/etc/systemd/system/strmsync.service`:

```ini
[Unit]
Description=STRMSync Media Sync Service
After=network.target

[Service]
Type=simple
User=strmsync
WorkingDirectory=/opt/strmsync
ExecStart=/opt/strmsync/server
Restart=on-failure
RestartSec=5s
EnvironmentFile=/opt/strmsync/.env

# 日志
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

启用并启动服务：

```bash
# 重新加载配置
sudo systemctl daemon-reload

# 启用开机自启
sudo systemctl enable strmsync

# 启动服务
sudo systemctl start strmsync

# 查看状态
sudo systemctl status strmsync

# 查看日志
sudo journalctl -u strmsync -f
```

---

## 监控和日志

### 健康检查

```bash
curl http://localhost:6754/api/health
# 期望响应: {"status":"healthy","timestamp":"..."}
```

### 日志级别

通过 `LOG_LEVEL` 环境变量控制（未在.env.example中显示，为向后兼容保留）：

```bash
LOG_LEVEL=debug ./server    # 调试信息
LOG_LEVEL=info ./server     # 一般信息（默认）
LOG_LEVEL=warn ./server     # 仅警告
LOG_LEVEL=error ./server    # 仅错误
```

### 日志格式

后端使用结构化日志（JSON格式），适合日志聚合系统：

```json
{"level":"info","ts":"2026-02-19T10:00:00.000+0800","msg":"任务执行完成","job_id":1,"run_id":123,"duration":330}
```

---

## 安全建议

1. **加密密钥**: 必须使用随机生成的强密钥，且不同环境使用不同密钥
2. **反向代理**: 生产环境建议通过Nginx/Caddy进行反向代理并启用HTTPS
3. **网络隔离**: 后端端口6754建议仅监听内网地址或localhost
4. **防火墙**: 通过防火墙规则限制访问来源
5. **定期备份**: 定期备份SQLite数据库文件

---

## 故障排查

### 后端无法启动

```bash
# 检查端口是否被占用
lsof -i :6754

# 检查环境变量
cat .env

# 以debug日志级别运行
LOG_LEVEL=debug ./server
```

### 前端无法访问

```bash
# 确认服务器已启动
curl http://localhost:6754/api/health

# 检查前端构建产物是否存在
ls frontend/dist/
```

### 数据库问题

```bash
# 检查数据库文件权限
ls -la app/data.db

# SQLite命令行查看数据
sqlite3 app/data.db ".tables"
sqlite3 app/data.db "SELECT count(*) FROM jobs;"
```

---

## 版本升级

```bash
# 1. 停止服务
./scripts/stop.sh

# 2. 备份数据
cp app/data.db app/data.db.backup.$(date +%Y%m%d)

# 3. 更新代码
git pull origin main

# 4. 重新构建
cd frontend && npm install && npm run build && cd ..
cd backend && go build -o server ./cmd/server && cd ..

# 5. 启动服务
./scripts/start.sh
```

数据库迁移将在启动时自动执行。
