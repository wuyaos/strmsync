# 文档目录

本目录包含项目的技术文档和API文档。

## 📂 文档列表

### 应用API文档

| 文档 | 说明 | 状态 |
|------|------|------|
| [backend/README.md](../backend/README.md) | 后端HTTP API完整文档 | ✅ 完成 |
| [DEPLOYMENT.md](DEPLOYMENT.md) | 生产环境部署指南 | ✅ 完成 |

### CloudDrive2 集成文档

| 文档 | 说明 | 状态 |
|------|------|------|
| [CloudDrive2_Integration.md](CloudDrive2_Integration.md) | CloudDrive2 gRPC 集成完整指南（整合版） | ✅ 完成 |
| [CloudDrive2_API.md](CloudDrive2_API.md) | CloudDrive2 gRPC API 官方文档 | 📚 参考 |

### 媒体服务器 API 文档

| 文档 | 说明 | 状态 |
|------|------|------|
| [Emby_API.md](Emby_API.md) | Emby REST API 文档 | 📚 参考 |
| [Jellyfin_API.md](Jellyfin_API.md) | Jellyfin REST API 文档 | 📚 参考 |

### 其他第三方 API

| 文档 | 说明 | 状态 |
|------|------|------|
| [OpenList_API.md](OpenList_API.md) | OpenList REST API 文档 | 📚 参考 |

---

## 🗂️ 文档用途

### 开发参考
- **后端 API**: 前后端开发时查看API规范和响应格式
- **DEPLOYMENT**: 生产环境部署和运维参考
- **CloudDrive2集成**: 查看Integration了解gRPC集成实现
- **第三方API**: 集成其他服务时参考对应的API文档

### 问题排查
- **DEPLOYMENT**: 故障排查和日志查看
- **CloudDrive2_Known_Issues**: CloudDrive2相关问题（如存在）
- **gRPC_Setup**: 开发环境问题参考设置指南

### 快速上手
- **HTTP_API**: 了解可用的API接口和调用方式
- **DEPLOYMENT**: 快速部署指南和环境配置

---

## 📝 文档维护

### 新增文档
新增技术文档时，请：
1. 使用清晰的文件名（PascalCase + 下划线）
2. 在本README.md中添加索引
3. 包含创建日期和最后更新日期
4. 使用Markdown格式，添加目录

### 文档更新
更新文档时：
1. 更新"最后更新"日期
2. 如果是重大变更，在顶部添加changelog
3. 保持与代码实现的一致性

---

## 🔗 相关目录

- **测试用例**: 见 [../tests/](../tests/)
- **运行时日志**: `logs/` 目录与可执行文件同级，运行时自动创建
  - 生产环境（Docker）: `/app/logs/`
  - 开发环境: `<项目根目录>/logs/`
  - 测试环境: `tests/logs/`
- **代码文档**: 见各模块代码注释

---

**维护者**: STRMSync Team
**最后更新**: 2026-02-19
