#!/bin/bash
# 文件名: migrate_stage1_core.sh
# 说明: 阶段1 - 迁移domain、infra、engine、queue、worker、scheduler

set -e

echo "=========================================="
echo "阶段1: 核心层迁移"
echo "=========================================="

cd "$(git rev-parse --show-toplevel)/backend"

# ============================================
# 1. 迁移 domain 层
# ============================================
echo "📦 1/8 迁移 domain 层..."

# 提取 models 到 domain/model/
echo "  - 提取模型到 domain/model/"
cp core/models.go internal/domain/model/models.go
git add internal/domain/model/models.go

# 创建 repository 接口文件
echo "  - 创建 repository 接口"
cat > internal/domain/repository/job.go << 'EOF'
// Package repository 定义领域层的Repository接口
package repository

import (
	"context"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
)

// JobRepository Job仓储接口
type JobRepository interface {
	GetByID(ctx context.Context, id uint) (model.Job, error)
	ListEnabledJobs(ctx context.Context) ([]model.Job, error)
	UpdateStatus(ctx context.Context, id uint, status string) error
	UpdateLastRunAt(ctx context.Context, id uint, lastRunAt time.Time) error
}
EOF

cat > internal/domain/repository/data_server.go << 'EOF'
package repository

import (
	"context"

	"github.com/strmsync/strmsync/internal/domain/model"
)

// DataServerRepository DataServer仓储接口
type DataServerRepository interface {
	GetByID(ctx context.Context, id uint) (model.DataServer, error)
}
EOF

git add internal/domain/repository/

echo "✅ domain 层迁移完成"

# ============================================
# 2. 迁移 infra/persistence
# ============================================
echo "📦 2/8 迁移 infra/persistence..."

git mv core/database.go internal/infra/persistence/database.go
git mv core/config.go internal/infra/persistence/config.go
git mv core/job_repository.go internal/infra/persistence/repository/job.go
git mv core/data_server_repository.go internal/infra/persistence/repository/data_server.go

# 删除旧的core目录（如果为空）
if [ -z "$(ls -A core 2>/dev/null)" ]; then
    rmdir core
fi

echo "✅ infra/persistence 迁移完成"

# ============================================
# 3. 迁移 infra/filesystem
# ============================================
echo "📦 3/8 迁移 infra/filesystem..."

# 移动主文件
git mv filesystem/interfaces.go internal/infra/filesystem/interfaces.go
git mv filesystem/types.go internal/infra/filesystem/types.go
git mv filesystem/client.go internal/infra/filesystem/client.go
git mv filesystem/driver_adapter.go internal/infra/filesystem/driver_adapter.go
git mv filesystem/driver_adapter_test.go internal/infra/filesystem/driver_adapter_test.go

# 移动实现文件
git mv filesystem/clouddrive2.go internal/infra/filesystem/clouddrive2/client.go
git mv filesystem/openlist.go internal/infra/filesystem/openlist/client.go
git mv filesystem/local.go internal/infra/filesystem/local/client.go

# 移动proto
if [ -d "filesystem/clouddrive2_proto" ]; then
    git mv filesystem/clouddrive2_proto/* internal/infra/filesystem/clouddrive2/proto/ 2>/dev/null || true
    rmdir filesystem/clouddrive2_proto 2>/dev/null || true
fi

# 删除旧目录
if [ -d "filesystem" ]; then
    rmdir filesystem 2>/dev/null || true
fi

echo "✅ infra/filesystem 迁移完成"

# ============================================
# 4. 迁移 infra/mediaserver
# ============================================
echo "📦 4/8 迁移 infra/mediaserver..."

git mv mediaserver/interfaces.go internal/infra/mediaserver/interfaces.go
git mv mediaserver/types.go internal/infra/mediaserver/types.go
git mv mediaserver/client.go internal/infra/mediaserver/client.go
git mv mediaserver/emby.go internal/infra/mediaserver/emby/client.go
git mv mediaserver/jellyfin.go internal/infra/mediaserver/jellyfin/client.go

rmdir mediaserver 2>/dev/null || true

echo "✅ infra/mediaserver 迁移完成"

# ============================================
# 5. 迁移 infra/writer
# ============================================
echo "📦 5/8 迁移 infra/writer..."

git mv strmwriter/interfaces.go internal/infra/writer/interfaces.go
git mv strmwriter/local_writer.go internal/infra/writer/local_writer.go

rmdir strmwriter 2>/dev/null || true

echo "✅ infra/writer 迁移完成"

# ============================================
# 6. 迁移 engine、queue、worker、scheduler
# ============================================
echo "📦 6/8 迁移核心模块..."

git mv syncengine/* internal/engine/
git mv syncqueue/* internal/queue/
git mv worker/* internal/worker/
git mv scheduler/* internal/scheduler/

rmdir syncengine syncqueue worker scheduler 2>/dev/null || true

echo "✅ 核心模块迁移完成"

# ============================================
# 7. 迁移 pkg (utils)
# ============================================
echo "📦 7/8 迁移工具包..."

git mv utils/logger.go internal/pkg/logger/logger.go
git mv utils/crypto.go internal/pkg/crypto/crypto.go
git mv utils/hash.go internal/pkg/hash/hash.go
git mv utils/path.go internal/pkg/path/path.go
git mv utils/request_id.go internal/pkg/requestid/request_id.go

rmdir utils 2>/dev/null || true

echo "✅ 工具包迁移完成"

# ============================================
# 8. 更新包声明和import路径
# ============================================
echo "📦 8/8 更新包声明和import路径..."

# 更新 domain/model 中的包声明
sed -i 's|package core|package model|g' internal/domain/model/models.go

# 更新 persistence 中的包声明
find internal/infra/persistence -name "*.go" -type f -exec sed -i 's|package core|package persistence|g' {} +
find internal/infra/persistence/repository -name "*.go" -type f -exec sed -i 's|package core|package repository|g' {} +

# 批量替换 import 路径
find . -name "*.go" -type f -not -path "./internal/*" -exec sed -i \
    -e 's|github.com/strmsync/strmsync/core|github.com/strmsync/strmsync/internal/infra/persistence|g' \
    -e 's|github.com/strmsync/strmsync/filesystem|github.com/strmsync/strmsync/internal/infra/filesystem|g' \
    -e 's|github.com/strmsync/strmsync/mediaserver|github.com/strmsync/strmsync/internal/infra/mediaserver|g' \
    -e 's|github.com/strmsync/strmsync/strmwriter|github.com/strmsync/strmsync/internal/infra/writer|g' \
    -e 's|github.com/strmsync/strmsync/syncengine|github.com/strmsync/strmsync/internal/engine|g' \
    -e 's|github.com/strmsync/strmsync/syncqueue|github.com/strmsync/strmsync/internal/queue|g' \
    -e 's|github.com/strmsync/strmsync/worker|github.com/strmsync/strmsync/internal/worker|g' \
    -e 's|github.com/strmsync/strmsync/scheduler|github.com/strmsync/strmsync/internal/scheduler|g' \
    -e 's|github.com/strmsync/strmsync/utils|github.com/strmsync/strmsync/internal/pkg|g' \
    {} +

# 更新internal目录内的文件
find internal -name "*.go" -type f -exec sed -i \
    -e 's|github.com/strmsync/strmsync/core|github.com/strmsync/strmsync/internal/infra/persistence|g' \
    -e 's|github.com/strmsync/strmsync/filesystem|github.com/strmsync/strmsync/internal/infra/filesystem|g' \
    -e 's|github.com/strmsync/strmsync/mediaserver|github.com/strmsync/strmsync/internal/infra/mediaserver|g' \
    -e 's|github.com/strmsync/strmsync/strmwriter|github.com/strmsync/strmsync/internal/infra/writer|g' \
    -e 's|github.com/strmsync/strmsync/syncengine|github.com/strmsync/strmsync/internal/engine|g' \
    -e 's|github.com/strmsync/strmsync/syncqueue|github.com/strmsync/strmsync/internal/queue|g' \
    -e 's|github.com/strmsync/strmsync/worker|github.com/strmsync/strmsync/internal/worker|g' \
    -e 's|github.com/strmsync/strmsync/scheduler|github.com/strmsync/strmsync/internal/scheduler|g' \
    -e 's|github.com/strmsync/strmsync/utils|github.com/strmsync/strmsync/internal/pkg|g' \
    {} +

echo "✅ 路径更新完成"

# ============================================
# 9. 编译验证
# ============================================
echo "🔨 编译验证..."
if go build ./...; then
    echo "✅ 编译成功！"
else
    echo "❌ 编译失败，请检查错误"
    exit 1
fi

# ============================================
# 10. 提交
# ============================================
echo "💾 提交更改..."
git add .
git commit -m "refactor(stage1): migrate domain, infra, engine, queue, worker, scheduler

- Move core/ to internal/infra/persistence/
- Move filesystem/ to internal/infra/filesystem/
- Move mediaserver/ to internal/infra/mediaserver/
- Move strmwriter/ to internal/infra/writer/
- Move syncengine/ to internal/engine/
- Move syncqueue/ to internal/queue/
- Move worker/ to internal/worker/
- Move scheduler/ to internal/scheduler/
- Move utils/ to internal/pkg/
- Create domain layer with model and repository interfaces
- Update all import paths"

echo ""
echo "✅ 阶段1完成！"
echo ""
echo "下一步: 执行 ./migrate_stage2_app_transport.sh"
