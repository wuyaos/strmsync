#!/bin/bash
# 文件名: migrate_stage0_prepare.sh
# 说明: 准备工作 - 创建备份和迁移分支

set -e  # 遇到错误立即退出

echo "=========================================="
echo "阶段0: 准备工作"
echo "=========================================="

# 1. 确认在正确的目录
if [ ! -f "go.mod" ]; then
    echo "❌ 错误: 请在backend目录下执行此脚本"
    exit 1
fi

# 2. 确认工作区干净
if [ -n "$(git status --porcelain)" ]; then
    echo "❌ 错误: Git工作区不干净，请先提交或stash当前修改"
    git status
    exit 1
fi

# 3. 创建备份分支
CURRENT_BRANCH=$(git branch --show-current)
BACKUP_BRANCH="backup-before-refactor-$(date +%Y%m%d-%H%M%S)"
echo "📦 创建备份分支: $BACKUP_BRANCH"
git branch $BACKUP_BRANCH

# 4. 创建迁移分支
REFACTOR_BRANCH="refactor/architecture-v2"
echo "🌿 创建迁移分支: $REFACTOR_BRANCH"
git checkout -b $REFACTOR_BRANCH

# 5. 创建新目录结构（空目录）
echo "📁 创建新目录结构..."
mkdir -p cmd/server
mkdir -p internal/transport/http/{handler,middleware}
mkdir -p internal/app/{sync,job,taskrun,file}
mkdir -p internal/domain/{model,repository,error}
mkdir -p internal/infra/persistence/repository
mkdir -p internal/infra/filesystem/{clouddrive2/proto,openlist,local}
mkdir -p internal/infra/mediaserver/{emby,jellyfin}
mkdir -p internal/infra/writer
mkdir -p internal/{engine,queue,worker,scheduler}
mkdir -p internal/pkg/{logger,crypto,hash,path,requestid}

# 6. 创建空的.gitkeep文件（防止空目录丢失）
find internal -type d -empty -exec touch {}/.gitkeep \;
find cmd -type d -empty -exec touch {}/.gitkeep \;

# 7. 提交空结构
git add cmd/ internal/
git commit -m "chore: create new directory structure for architecture refactoring"

echo ""
echo "✅ 准备工作完成！"
echo "   备份分支: $BACKUP_BRANCH"
echo "   当前分支: $REFACTOR_BRANCH"
echo ""
echo "下一步: 执行 ./migrate_stage1_core.sh"
