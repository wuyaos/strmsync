# STRMSync 脚本目录

## 📂 脚本列表

| 脚本 | 说明 |
|------|------|
| `gen_clouddrive2_proto.sh` | 生成 CloudDrive2 gRPC 代码（集成自动下载 proto） |
| `update_clouddrive2_api.sh` | 更新 CloudDrive2 API 文档 |
| `prod-start.sh` | 启动生产环境服务 |
| `prod-stop.sh` | 停止生产环境服务 |
| `prod-restart.sh` | 重启生产环境服务 |
| `prod-start-separate.sh` | 分离部署启动（前端 + 后端 + Nginx） |
| `prepare-separate-package.sh` | 整理分离部署产物（生成 dist 结构） |
| `start-separate.sh` | 分离部署启动模板（自动下载本地 Nginx） |
| `strmsync.conf` | Nginx 配置模板 |

## 🚀 开发环境管理

**所有开发环境操作已迁移到 Makefile，请使用以下命令：**

```bash
# 启动开发环境（Air 热重载 + Vite HMR）
make dev

# 停止开发环境
make dev-stop

# 重启开发环境
make dev-restart

# 强制清理端口占用
make kill-ports
```

## 🏭 生产环境管理

```bash
# 构建合并部署版本
make build

# 启动生产服务（工作目录为 dist）
./dist/prod-start.sh

# 停止生产服务
./scripts/prod-stop.sh

# 重启生产服务
./scripts/prod-restart.sh

# 分离部署（整理 + 启动）
./scripts/prepare-separate-package.sh
./scripts/prod-start-separate.sh

# 查看日志
tail -f logs/strmsync.log
```

**环境变量配置：**
- 在项目根目录创建 `.env` 文件
- 参考 `.env.example` 配置生产环境参数
- `make build` 会将 `.env` 复制到 `dist/.env`（若不存在则复制 `.env.example`）

## 🔧 其他命令

```bash
# 查看所有可用命令
make help

# 清理编译缓存
make clean-cache

# 完全清理（包括数据库）
make clean-all
```

## 📖 详细文档

- [HTTP API 文档](../docs/HTTP_API.md)
- [部署文档](../docs/DEPLOYMENT.md)
- [项目说明](../README.md)

---

**最后更新**: 2026-02-20
