# STRMSync

STRMSync 是一款面向媒体库的自动化同步工具，支持从本地或云盘数据源生成 STRM 文件与元数据，统一管理任务、调度与运行状态。

## 功能介绍

- 支持多数据源（Local / CloudDrive2 / OpenList）
- 支持多媒体服务器（Emby / Jellyfin / Plex）
- STRM 生成与元数据同步
- 任务配置、定时调度、运行记录与告警
- 统一的可视化管理界面

## 快速开始

1. 启动后端（确保数据库与配置已就绪）
2. 启动前端并访问界面
3. 添加数据服务器与媒体服务器
4. 创建任务并运行

如需完整开发说明，请查看：`DEVELOPMENT.md`

## 安装部署

- 推荐使用部署文档：`docs/DEPLOYMENT.md`
- 生产环境请使用稳定的数据库与持久化存储
- 建议开启定时任务与日志归档

## TODO List

- [ ] OpenList / CloudDrive2 未完全适配
- [ ] Plex 适配
- [ ] Docker 部署
- [ ] GitHub Actions 自动编译发布
- [ ] 更详细的使用说明

## 依赖

- Element Plus
- Vite
- Vue.js
- ECharts
- Day.js

## 开发说明

本项目由 Codex + Claude Code 协同开发与维护。
