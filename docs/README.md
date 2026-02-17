# 文档目录

本目录包含项目的技术文档和API文档。

## 📂 文档列表

### CloudDrive2相关文档（已完成集成）

| 文档 | 说明 | 状态 |
|------|------|------|
| [CloudDrive2_Integration.md](CloudDrive2_Integration.md) | CloudDrive2 gRPC集成完整文档 | ✅ 完成 |
| [CloudDrive2_gRPC_Setup.md](CloudDrive2_gRPC_Setup.md) | gRPC开发环境设置指南 | ✅ 完成 |
| [CloudDrive2_API.md](CloudDrive2_API.md) | CloudDrive2 gRPC API完整文档 | 📚 参考 |
| [clouddrive.proto](clouddrive.proto) | Proto定义文件（v0.9.24） | 📚 参考 |

### 第三方API文档（待集成）

| 文档 | 说明 | 状态 |
|------|------|------|
| [Emby_Jellyfin_API.md](Emby_Jellyfin_API.md) | Emby/Jellyfin API文档 | ⏳ 待实现 |
| [OpenList_API.md](OpenList_API.md) | OpenList REST API文档 | ⏳ 待实现 |

### 架构和设计文档

| 文档 | 说明 | 状态 |
|------|------|------|
| [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) | 详细实施方案（STRM规则、数据库、服务架构） | 📚 设计参考 |

---

## 🗂️ 文档用途

### 开发参考
- **CloudDrive2集成**: 查看Integration和Test Report了解当前实现
- **API文档**: 集成其他服务时参考对应的API文档
- **架构文档**: 了解系统设计和部署方案

### 问题排查
- **Known_Issues**: 遇到CloudDrive2相关问题先查看已知问题
- **gRPC_Setup**: 开发环境问题参考设置指南

### 未来开发
- **Emby/Jellyfin**: MediaServer集成时参考
- **OpenList**: 第二数据源集成时参考

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

### 废弃文档
当文档过时时：
1. 移动到`docs/archive/`目录（如需保留）
2. 或直接删除（不再需要）
3. 从本README.md中移除索引

---

## 🔗 相关目录

- **项目总结**: 见 [../.claude/summaries/](../.claude/summaries/)
- **测试用例**: 见 [../tests/](../tests/)
- **运行时日志**: `logs/` 目录与可执行文件同级，运行时自动创建
  - 生产环境（Docker）: `/app/logs/`
  - 开发环境: `<项目根目录>/logs/`
  - 测试环境: `tests/logs/`
- **代码文档**: 见各模块代码注释

---

**维护者**: STRMSync Team
**最后更新**: 2026-02-18
