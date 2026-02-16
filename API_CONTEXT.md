# API 对接文档总览

本文档提供 STRMSync 项目所需的所有外部 API 集成概览。

## 📚 详细 API 文档

- **[CloudDrive2 完整 API 文档](./docs/CloudDrive2_API.md)** - gRPC 接口详细说明
- **[OpenList 完整 API 文档](./docs/OpenList_API.md)** - REST API 接口详细说明

---

## 1. CloudDrive2 API（快速参考）

### 基础信息
- **协议**: gRPC/HTTP2
- **认证**: JWT Bearer Token
- **版本**: v0.9.24
- **默认地址**: http://localhost:19798
- **详细文档**: [CloudDrive2_API.md](./docs/CloudDrive2_API.md)

### 核心接口
| 接口 | 方法 | 说明 |
|------|------|------|
| GetToken | RPC | 获取 JWT Token |
| GetSystemInfo | RPC | 获取系统信息（公开接口） |
| List | RPC | 列出目录文件 |
| GetFileInfo | RPC | 获取文件信息 |
| GetMountPoints | RPC | 获取挂载点列表 |

### 使用模式
1. **本地挂载模式**（推荐）: 直接访问挂载路径 `/mnt/clouddrive`
2. **API 模式**（备选）: 通过 gRPC 接口操作

---

## 2. OpenList API（快速参考）

### 基础信息
- **协议**: HTTP REST
- **认证**: JWT Token（通过 /api/auth/login 获取）
- **默认地址**: http://localhost:5244
- **详细文档**: [OpenList_API.md](./docs/OpenList_API.md)

### 核心接口
| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 登录 | POST | /api/auth/login | 获取 Token |
| 文件列表 | POST | /api/fs/list | 列出目录 |
| 文件信息 | POST | /api/fs/get | 获取文件详情（含下载链接） |
| 搜索 | POST | /api/fs/search | 搜索文件 |
| 下载 | GET | /d/{路径} | 直接下载文件 |

### STRM 生成规则
```
源文件: /Movies/Action/Movie.mkv
STRM 内容: http://localhost:5244/d/Movies/Action/Movie.mkv
```

---

## 3. 媒体库 API

### Emby
- **地址**: http://localhost:8096
- **认证**: X-Emby-Token Header
- **刷新接口**: `POST /Library/Refresh`

### Plex
- **地址**: http://localhost:32400
- **认证**: X-Plex-Token Header
- **刷新接口**: `GET /library/sections/{section_id}/refresh`

### Jellyfin
- **地址**: http://localhost:8096
- **认证**: Authorization Header（MediaBrowser Token）
- **刷新接口**: `POST /Library/Refresh`

---

## 路径映射规则

### 本地文件系统
```
源端: /volume1/Media/Movies/Action/Movie.mkv
目标: /media/library/Movies/Action/Movie.strm
```

### CloudDrive2 挂载模式
```
源端: /mnt/clouddrive/Movies/Movie.mkv
目标: /media/library/Movies/Movie.strm
STRM 内容: /mnt/clouddrive/Movies/Movie.mkv
```

### CloudDrive2 API 模式
```
源端: cloudfs://115/Movies/Movie.mkv
目标: /media/library/Movies/Movie.strm
STRM 内容: http://localhost:19798/api/v1/download?path=/115/Movies/Movie.mkv&token=xxx
```

### OpenList API 模式
```
源端: openlist://Storage/Movies/Movie.mkv
目标: /media/library/Movies/Movie.strm
STRM 内容: http://localhost:5244/d/Storage/Movies/Movie.mkv
```

---

## 数据规模和性能要求

```yaml
data_scale:
  current_files: 30000           # 当前文件数
  max_files: 100000              # 预计最大文件数
  max_dir_files: 5000            # 单目录最大文件数
  total_storage: 50TB            # 总存储容量
  daily_new_files: 50            # 每天新增文件数
  batch_import: true             # 支持批量导入

network:
  bandwidth: 1Gbps               # 网络带宽
  latency: < 10ms                # 延迟
  clouddrive_mounted: true       # CloudDrive2 是否挂载

performance:
  scan_speed: > 3000 files/sec   # 扫描速度
  hash_speed: > 1000 files/sec   # 哈希计算速度
  strm_gen_speed: > 5000 files/sec # STRM 生成速度
  watch_delay: < 5 sec           # 文件变更检测延迟
```

---

## 排除规则

```yaml
exclude_patterns:
  directories:
    - .tmp
    - .@__thumb
    - @eaDir
    - .Trash-*
    - lost+found

  files:
    - Thumbs.db
    - .DS_Store
    - desktop.ini
    - "*.partial"
    - "*.!qB"
    - "*.crdownload"

  extensions:
    - .nfo
    - .jpg
    - .png
    - .txt
    - .srt
    - .ass
    - .ssa
```
