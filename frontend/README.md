# STRMSync Web 前端

基于 Vue 3 + Element Plus 的现代化管理界面。

## 技术栈

- **核心框架**: Vue 3 (Composition API)
- **构建工具**: Vite 5
- **UI组件库**: Element Plus 2
- **路由**: Vue Router 4
- **状态管理**: Pinia 2
- **HTTP客户端**: Axios
- **日期处理**: Day.js

## 快速开始

### 安装依赖

```bash
npm install
```

### 开发模式

```bash
npm run dev
```

访问 http://localhost:5173

### 生产构建

```bash
npm run build
```

构建产物位于 `dist/` 目录。

### 预览生产构建

```bash
npm run preview
```

## 项目结构

```
frontend/
├── public/               # 静态资源
├── src/
│   ├── api/             # API 接口封装
│   ├── assets/          # 资源文件
│   │   └── styles/      # 全局样式
│   ├── layouts/         # 布局组件
│   ├── router/          # 路由配置
│   ├── views/           # 页面组件
│   ├── App.vue          # 根组件
│   └── main.js          # 入口文件
├── index.html           # HTML 模板
├── vite.config.js       # Vite 配置
└── package.json         # 项目配置
```

## 功能模块

### 已实现

- [x] 仪表盘 - 系统概览和关键指标
- [x] 数据源管理 - CRUD 操作、扫描、监控
- [x] 主布局 - 侧边栏导航、顶部状态栏
- [x] 暗色模式支持
- [x] 响应式设计

### 待实现

- [ ] 文件浏览器 - 文件列表、搜索、详情
- [ ] 任务管理 - 任务列表、进度监控
- [ ] 媒体库通知 - 通知器配置、历史记录
- [ ] 系统设置 - 配置管理

## API 代理

开发模式下，所有 `/api` 请求会被代理到 `http://localhost:3000`。

如需修改后端地址，请编辑 `vite.config.js` 中的 proxy 配置。

## 浏览器支持

- Chrome >= 87
- Firefox >= 78
- Safari >= 14
- Edge >= 88

## 开发规范

### 代码风格

- 使用 Composition API
- 使用 `<script setup>` 语法
- 组件文件使用 PascalCase 命名
- 样式使用 SCSS

### 组件规范

- 页面组件放在 `views/` 目录
- 公共组件放在 `components/` 目录
- 布局组件放在 `layouts/` 目录

## 许可证

MIT
