# STRMSync Web 界面完整测试指南

## 测试准备

### 1. 确保已编译后端

```bash
cd /mnt/c/Users/wff19/Desktop/strm/backend
go build -o ../build/strmsync ./cmd/server
```

### 2. 安装前端依赖

```bash
cd /mnt/c/Users/wff19/Desktop/strm/frontend
npm install
```

## 启动服务

### 方法一：分别启动（推荐用于开发测试）

#### 终端 1：启动后端

```bash
cd /mnt/c/Users/wff19/Desktop/strm

# 设置环境变量
export PORT=3000
export HOST=0.0.0.0
export DB_PATH=data/test.db
export LOG_LEVEL=debug
export LOG_PATH=logs
export ENCRYPTION_KEY=test_encryption_key_12345678
export SCANNER_CONCURRENCY=10
export SCANNER_BATCH_SIZE=100
export NOTIFIER_ENABLED=true
export NOTIFIER_PROVIDER=emby
export NOTIFIER_BASE_URL=http://192.168.123.179:8096
export NOTIFIER_TOKEN=fa58c9680ed34ffeb31f19f3e19f4ca3
export NOTIFIER_TIMEOUT=10
export NOTIFIER_RETRY_MAX=3
export NOTIFIER_RETRY_BASE_MS=1000
export NOTIFIER_DEBOUNCE=5
export NOTIFIER_SCOPE=global

# 创建必要目录
mkdir -p data logs test/output

# 启动后端服务
./build/strmsync
```

后端服务将在 http://localhost:3000 运行。

#### 终端 2：启动前端

```bash
cd /mnt/c/Users/wff19/Desktop/strm/frontend
npm run dev
```

前端服务将在 http://localhost:5173 运行。

### 方法二：使用脚本启动

#### 终端 1：启动后端

```bash
cd /mnt/c/Users/wff19/Desktop/strm
./scripts/test-start.sh
```

#### 终端 2：启动前端

```bash
cd /mnt/c/Users/wff19/Desktop/strm/frontend
npm run dev
```

## Web 界面测试流程

### 1. 访问界面

打开浏览器（推荐 Chrome）访问：

```
http://localhost:5173
```

你应该看到 STRMSync 的主界面，包括：
- 左侧侧边栏导航
- 顶部状态栏
- 仪表盘页面内容

### 2. 测试仪表盘

**期望看到：**
- ✅ 4个统计卡片（数据源、文件数、运行任务、失败任务）
- ✅ 数据源状态列表（当前应为空）
- ✅ 最近任务列表（当前应为空）
- ✅ 页面刷新按钮

**操作：**
1. 点击右上角刷新按钮
2. 验证页面数据重新加载
3. 测试暗色模式切换（点击月亮/太阳图标）

### 3. 测试数据源管理

点击左侧菜单 "数据源管理"。

#### 3.1 创建本地数据源

**操作：**
1. 点击右上角 "添加数据源" 按钮
2. 填写表单：
   - 数据源名称：`本地测试媒体库`
   - 类型：选择 `Local`
   - 启用：勾选
   - 源路径前缀：`/mnt/c/Users/wff19/Desktop/strm/test/media`
   - 目标路径前缀：`/mnt/c/Users/wff19/Desktop/strm/test/output`
3. 点击 "保存" 按钮

**期望结果：**
- ✅ 弹出成功提示
- ✅ 抽屉关闭
- ✅ 数据源列表中出现新创建的数据源
- ✅ 状态显示为 "空闲"

#### 3.2 测试扫描功能

**操作：**
1. 在数据源卡片中点击 "扫描" 按钮
2. 观察状态变化

**期望结果：**
- ✅ 弹出"扫描任务已提交"提示
- ✅ 数据源状态变为 "扫描中"（蓝色）
- ✅ 出现进度条（如果扫描时间较长）
- ✅ 扫描完成后状态变为 "空闲"（灰色）
- ✅ 文件数更新
- ✅ "最后扫描"时间更新

**验证：**
```bash
# 在另一个终端检查生成的 STRM 文件
ls -R /mnt/c/Users/wff19/Desktop/strm/test/output
```

#### 3.3 测试文件监控

**操作：**
1. 点击数据源卡片右侧的 "更多" 按钮（三个点）
2. 选择 "启动监控"

**期望结果：**
- ✅ 弹出成功提示
- ✅ 数据源状态变为 "监控中"（绿色）

**验证实时监控：**
```bash
# 在 test/media 目录中创建一个测试文件
cd /mnt/c/Users/wff19/Desktop/strm/test/media
echo "test video" > test.mkv

# 等待几秒后检查 output 目录
ls -la /mnt/c/Users/wff19/Desktop/strm/test/output
```

#### 3.4 测试元数据同步

**操作：**
1. 点击 "更多" 按钮
2. 选择 "同步元数据"

**期望结果：**
- ✅ 弹出"元数据同步任务已提交"提示
- ✅ 后台开始同步 NFO、海报、字幕等文件

#### 3.5 测试媒体库通知

**操作：**
1. 点击 "更多" 按钮
2. 选择 "触发通知"

**期望结果：**
- ✅ 弹出"通知已触发"提示
- ✅ Emby/Jellyfin 媒体库收到刷新通知

#### 3.6 测试编辑功能

**操作：**
1. 点击数据源卡片的 "设置" 按钮
2. 修改数据源名称
3. 点击 "保存"

**期望结果：**
- ✅ 弹出成功提示
- ✅ 数据源名称更新

#### 3.7 测试视图切换

**操作：**
1. 点击工具栏右侧的视图切换按钮（网格/列表）
2. 切换到列表视图
3. 再切换回卡片视图

**期望结果：**
- ✅ 视图平滑切换
- ✅ 数据完整显示

#### 3.8 测试搜索和过滤

**操作：**
1. 创建多个不同类型的数据源（Local、CloudDrive2、OpenList）
2. 在搜索框输入关键字
3. 使用类型和状态下拉框过滤

**期望结果：**
- ✅ 搜索实时过滤数据源
- ✅ 过滤器正确工作
- ✅ 清除按钮清空搜索

#### 3.9 测试删除功能

**操作：**
1. 点击数据源卡片的删除按钮（列表视图）或 "更多" → "删除"
2. 在确认对话框点击 "确定"

**期望结果：**
- ✅ 弹出确认对话框
- ✅ 确认后显示成功提示
- ✅ 数据源从列表中消失

### 4. 测试创建 CloudDrive2 数据源

**操作：**
1. 点击 "添加数据源"
2. 填写表单：
   - 名称：`CloudDrive2 测试`
   - 类型：`CloudDrive2`
   - 源路径前缀：`/115open/FL/AV/日本/已刮削/other`
   - 目标路径前缀：`/mnt/c/Users/wff19/Desktop/strm/test/output/cd2`
   - 主机地址：`192.168.123.179`
   - 端口：`19798`
   - 认证密钥：`68cc50f8-e946-49c4-8e65-0be228e45df8`
3. 点击 "保存"

**期望结果：**
- ✅ 创建成功
- ✅ 可以正常扫描

### 5. 测试创建 OpenList 数据源

**操作：**
1. 点击 "添加数据源"
2. 填写表单：
   - 名称：`OpenList 测试`
   - 类型：`OpenList`
   - 源路径前缀：`/115open/FL/AV/日本/已刮削/other`
   - 目标路径前缀：`/mnt/c/Users/wff19/Desktop/strm/test/output/openlist`
   - 主机地址：`192.168.123.179`
   - 端口：`5244`
   - 认证密钥：`openlist-469275dc-37d6-4f4a-9525-1ae36209cb0bOZV39A8rxtg0htFCNPCWPbdqoHH6aEYT6YBfWKNzARjn5NNfqMrDgFGdWG1DUmfs`
3. 点击 "保存"

**期望结果：**
- ✅ 创建成功
- ✅ 可以正常扫描

### 6. 测试其他页面导航

**操作：**
依次点击左侧菜单的各个项目：
- 文件浏览器
- 任务管理
- 媒体库通知
- 系统设置

**期望结果：**
- ✅ 页面切换流畅
- ✅ 占位页面正确显示
- ✅ 左侧菜单高亮当前页面

### 7. 测试响应式设计

**操作：**
1. 调整浏览器窗口大小
2. 测试不同宽度下的显示效果

**期望结果：**
- ✅ 宽屏：卡片3列显示
- ✅ 中屏：卡片2列显示
- ✅ 窄屏：卡片1列显示
- ✅ 侧边栏自动收起/展开

### 8. 测试暗色模式

**操作：**
1. 点击顶部状态栏右侧的月亮图标
2. 切换到暗色模式
3. 再次点击切换回亮色模式

**期望结果：**
- ✅ 主题平滑切换
- ✅ 所有组件颜色正确
- ✅ 偏好设置保存（刷新后保持）

### 9. 测试实时更新

**操作：**
1. 在仪表盘或数据源管理页面停留
2. 等待自动刷新（30秒或10秒）
3. 观察数据更新

**期望结果：**
- ✅ 数据自动刷新
- ✅ 无需手动刷新页面
- ✅ 统计数据实时更新

### 10. 测试错误处理

**操作：**
1. 关闭后端服务
2. 在前端执行任何操作

**期望结果：**
- ✅ 显示"网络连接失败"错误提示
- ✅ 页面不崩溃
- ✅ 重新启动后端后功能恢复

## 性能测试

### 1. 大量数据源测试

**操作：**
创建 20+ 个数据源，测试：
- ✅ 列表渲染性能
- ✅ 搜索过滤性能
- ✅ 视图切换性能

### 2. 大文件扫描测试

**操作：**
扫描包含大量文件的目录，观察：
- ✅ 扫描进度更新
- ✅ 页面响应性
- ✅ 内存使用情况

## 常见问题排查

### 问题 1：前端无法连接后端

**检查：**
```bash
# 确认后端正在运行
curl http://localhost:3000/api/health

# 检查端口占用
netstat -an | grep 3000
```

**解决：**
- 确保后端服务已启动
- 检查环境变量 PORT 设置
- 查看后端日志

### 问题 2：数据源创建失败

**检查：**
- 路径是否存在且可访问
- CloudDrive2/OpenList 连接信息是否正确
- 查看后端日志中的错误信息

### 问题 3：扫描无响应

**检查：**
```bash
# 查看后端日志
tail -f logs/*.log

# 检查数据库
sqlite3 data/test.db "SELECT * FROM sources;"
sqlite3 data/test.db "SELECT * FROM tasks;"
```

### 问题 4：前端样式异常

**解决：**
```bash
# 重新安装依赖
cd frontend
rm -rf node_modules package-lock.json
npm install

# 清除缓存
npm run dev -- --force
```

## 测试清单

- [ ] 后端服务正常启动
- [ ] 前端服务正常启动
- [ ] 可以访问 Web 界面
- [ ] 仪表盘数据正确显示
- [ ] 创建本地数据源成功
- [ ] 扫描功能正常工作
- [ ] STRM 文件正确生成
- [ ] 文件监控正常工作
- [ ] 实时文件变更检测
- [ ] 元数据同步功能
- [ ] 媒体库通知功能
- [ ] 编辑数据源功能
- [ ] 删除数据源功能
- [ ] 搜索和过滤功能
- [ ] 视图切换功能
- [ ] 暗色模式切换
- [ ] 响应式设计
- [ ] 实时数据更新
- [ ] 错误处理机制
- [ ] CloudDrive2 数据源
- [ ] OpenList 数据源

## 测试报告

完成测试后，请记录：

1. **测试环境**
   - 操作系统
   - 浏览器版本
   - Go 版本
   - Node.js 版本

2. **测试结果**
   - 通过的功能
   - 发现的问题
   - 性能表现

3. **改进建议**
   - 功能建议
   - 性能优化
   - 用户体验

## 下一步

测试完成后，可以：

1. 完善其他前端页面（文件浏览器、任务管理等）
2. 实现 WebSocket 实时推送
3. 添加用户认证功能
4. Docker 容器化部署
5. 编写自动化测试脚本
6. 性能优化和监控

---

**祝测试顺利！** 🎉
