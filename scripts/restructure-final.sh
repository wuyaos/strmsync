#!/usr/bin/env bash
# 后端目录重构脚本：去除internal层，扁平化结构
# 执行位置：项目根目录

set -euo pipefail

echo "=== 开始后端重构 ==="

# Step 1: 移动main.go
echo "Step 1: 移动main.go到backend根目录"
git mv backend/cmd/server/main.go backend/main.go
echo "✓ main.go已移动"

# Step 2: 创建core目录并移动文件
echo "Step 2: 创建core/目录"
mkdir -p backend/core
git mv backend/internal/config/config.go backend/core/config.go
git mv backend/internal/database/database.go backend/core/database.go
git mv backend/internal/database/models.go backend/core/models.go
echo "✓ core/目录创建完成"

# Step 3: 移动handler（去internal层）
echo "Step 3: 移动handler到backend/handler"
git mv backend/internal/handlers backend/handler
echo "✓ handler已移动"

# Step 4: 扁平化service
echo "Step 4: 扁平化service目录"
mkdir -p backend/service
git mv backend/internal/service/interfaces.go backend/service/interfaces.go
git mv backend/internal/service/types/*.go backend/service/
git mv backend/internal/service/job/service.go backend/service/job.go
git mv backend/internal/service/taskrun/service.go backend/service/taskrun.go
git mv backend/internal/service/planner/planner.go backend/service/planner.go
git mv backend/internal/service/executor/executor.go backend/service/executor.go
git mv backend/internal/service/strm/generator.go backend/service/strm.go
git mv backend/internal/service/filemonitor/monitor.go backend/service/filemonitor.go
echo "✓ service已扁平化"

# Step 5: 移动dataserver
echo "Step 5: 移动dataserver到backend/dataserver"
git mv backend/internal/service/dataserver backend/dataserver
echo "✓ dataserver已移动"

# Step 6: 合并clouddrive2到dataserver
echo "Step 6: 合并clouddrive2到dataserver"
git mv backend/internal/clients/clouddrive2/client.go backend/dataserver/clouddrive2.go
git mv backend/internal/clients/clouddrive2/proto backend/dataserver/clouddrive2_proto
git mv backend/internal/clients/clouddrive2/pb/*.go backend/dataserver/clouddrive2_proto/
echo "✓ clouddrive2已合并"

# Step 7: 移动mediaserver
echo "Step 7: 移动mediaserver到backend/mediaserver"
git mv backend/internal/service/mediaserver backend/mediaserver
echo "✓ mediaserver已移动"

# Step 8: 移动utils
echo "Step 8: 移动utils到backend/utils"
git mv backend/internal/utils backend/utils
echo "✓ utils已移动"

# Step 9: 清理空目录
echo "Step 9: 清理空目录"
find backend/internal -type d -empty -delete 2>/dev/null || true
rmdir backend/internal/clients/clouddrive2 2>/dev/null || true
rmdir backend/internal/clients 2>/dev/null || true
rmdir backend/internal/service 2>/dev/null || true
rmdir backend/internal/config 2>/dev/null || true
rmdir backend/internal/database 2>/dev/null || true
rmdir backend/internal 2>/dev/null || true
rmdir backend/cmd/server 2>/dev/null || true
rmdir backend/cmd 2>/dev/null || true
echo "✓ 空目录已清理"

echo ""
echo "=== 重构完成！当前状态： ==="
git status --short

echo ""
echo "下一步："
echo "1. 执行 update-imports.sh 更新import路径"
echo "2. 执行 go build ./backend 验证编译"
