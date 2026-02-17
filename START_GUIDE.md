# STRMSync 启动指南

## 快速启动

```bash
# 添加执行权限
chmod +x scripts/*.sh

# 启动服务
./scripts/start.sh

# 停止服务  
./scripts/stop.sh

# 清理项目
./scripts/clean.sh
```

## 访问地址
- API: http://localhost:6754
- Web: http://localhost:5676

## 架构说明

新架构 (三层):
1. 配置管理: /api/servers/data, /api/servers/media
2. 任务配置: /api/jobs
3. 执行记录: /api/runs
