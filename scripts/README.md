# 开发脚本说明

本目录包含用于开发环境的辅助脚本。

## 开发脚本

### dev.sh ⭐
一键启动完整开发环境（推荐）

**功能：**
- 自动清理临时文件（保留编译缓存）
- 自动释放端口
- 后台启动前端（Vite HMR）
- 前台启动后端（Air 热重载）
- 统一进程管理

**使用方法：**
```bash
# 直接运行
./scripts/dev.sh

# 或使用 Makefile
make dev
```

**特点：**
- 一条命令启动前后端
- 后端日志直接显示在终端（方便调试）
- 前端后台运行，日志保存到 `tests/logs/vite.log`
- 每次启动自动清理前端进程和端口
- Ctrl+C 同时停止所有服务
- 保留编译缓存，重启仅需 1-2 秒

**环境变量：**
- `PORT`: 后端端口（默认：6786）
- `FRONTEND_PORT`: 前端端口（默认：7786）

## 编译性能优化

当前配置已针对开发环境优化：

| 优化项 | 配置 | 效果 |
|--------|------|------|
| 并行编译 | `-p $(nproc)` | 自动使用所有 CPU 核心 |
| 禁用优化 | `-gcflags="all=-N -l"` | 编译速度提升 2-3 倍 |
| 热重载延迟 | `delay = 500ms` | 更快的代码变更响应 |
| CGO 并行 | `GOMAXPROCS=$(nproc)` | 最大化 sqlite3 编译性能 |

**性能对比：**
- 首次编译：~15-30s（16核，优化前 60-90s）
- 重启开发环境：~1-2s（保留缓存，优化前 30-60s）
- 增量编译：~0.5-1s（优化前 5-8s）
- 热重载响应：~0.5-0.8s（优化前 1.5-2s）

**缓存策略：**
- ✅ 保留 Go 编译缓存（包括 sqlite3 等 CGO 包）
- ✅ 保留 Vite 缓存
- ❌ 清理临时文件（Go/Air/Backend tmp）

## 注意事项

1. **首次启动**：首次编译 sqlite3 需要 15-30 秒（16核）
2. **重启速度**：保留编译缓存，重启仅需 1-2 秒
3. **缓存清理**：如需完全重新编译，手动删除 `build/go/cache`
4. **端口冲突**：脚本会自动清理占用的端口
5. **日志查看**：
   - 一键启动：实时显示前后端日志
   - 分别启动：各自终端实时显示
6. **热重载**：修改代码后自动重新编译（0.5-1秒）
7. **进程管理**：
   - `dev.sh`：Ctrl+C 停止所有服务
   - 分别启动：需要在各自终端停止

## 故障排查

### 后端启动失败
```bash
# 检查端口占用
lsof -i :6786

# 手动清理
./scripts/dev-stop.sh

# 重新启动
./scripts/dev-start.sh
```

### 前端启动失败
```bash
# 检查 Node.js 版本
node --version  # 需要 18+

# 检查后端是否运行
curl http://localhost:6786/api/health

# 重新安装依赖
cd frontend
rm -rf node_modules package-lock.json
npm install
```

### 编译速度慢
```bash
# 检查 Go 版本
go version  # 需要 1.24+

# 清理缓存重新编译
rm -rf build/
./scripts/dev-start.sh
```

### 热重载不工作
```bash
# 检查 Air 版本
air -v

# 重新安装 Air
go install github.com/air-verse/air@latest
```

## 生产环境构建

使用 Makefile 进行生产环境构建：

```bash
# 完整构建（前端 + 后端）
make build

# 仅构建前端
make frontend

# 仅构建后端
make backend

# 多平台发布包
make release
```

**构建优化：**
- ✅ 去除调试信息和符号表（`-s -w`）
- ✅ 去除文件路径信息（`-trimpath`）
- ✅ 并行编译（`-p $(nproc)`）
- ✅ 启用 CGO（sqlite3 原生性能）
- ✅ 自动显示二进制文件大小

**生产环境脚本：**
- `prod-start.sh` - 启动生产服务
- `gen_clouddrive2_proto.sh` - 生成 gRPC 代码
- `update_clouddrive2_api.sh` - 更新 API 文档

详见项目根目录 DEVELOPMENT.md 和 docs/DEPLOYMENT.md。
