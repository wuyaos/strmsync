# STRMSync

![STRMSync Logo](frontend/src/assets/icons/logo.svg)

[![Version](https://img.shields.io/badge/Version-1.0.0-fb7299?style=flat-square)](VERSION) [![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org) [![Node Version](https://img.shields.io/badge/Node-18+-339933?style=flat-square&logo=node.js&logoColor=white)](https://nodejs.org) [![Vue Version](https://img.shields.io/badge/Vue-3.x-42b883?style=flat-square&logo=vue.js&logoColor=white)](https://vuejs.org) [![License](https://img.shields.io/badge/License-MIT-yellow?style=flat-square&logo=opensourceinitiative&logoColor=white)](LICENSE) [![Last Commit](https://img.shields.io/github/last-commit/wuyaos/strmsync?style=flat-square)](https://github.com/wuyaos/strmsync)

STRMSync 是一款面向媒体库的自动化同步工具，支持从本地或云盘数据源生成 STRM 文件与元数据，统一管理任务、调度与运行状态。

## ✨ 功能介绍

- 支持多数据源（Local / CloudDrive2 / OpenList）
- 支持多媒体服务器（Emby / Jellyfin / Plex）
- STRM 生成与元数据同步
- 任务配置、定时调度、运行记录与告警
- 统一的可视化管理界面

## 🚀 快速开始

### 环境准备
- Go 1.24+
- Node.js 18+

### 一键开发（推荐）
```bash
make dev
```

### 构建与运行
```bash
make build
./dist/prod-start.sh
```

### 快速使用流程
1. 添加数据服务器与媒体服务器
2. 创建任务并启用调度
3. 查看运行记录与同步结果

如需完整开发说明，请查看：`DEVELOPMENT.md`

## 📦 安装部署

- 推荐使用部署文档：`docs/DEPLOYMENT.md`
- 生产环境请使用稳定的数据库与持久化存储
- 建议开启定时任务与日志归档

## ✅ TODO List

- [ ] OpenList / CloudDrive2 未完全适配
- [ ] Plex 适配
- [ ] Docker 部署
- [ ] GitHub Actions 自动编译发布
- [ ] 更详细的使用说明

## 🧩 依赖

- Element Plus
- Vite
- Vue.js
- ECharts
- Day.js

## 📣 声明

本项目由 Codex + Claude Code 协同开发与维护。
