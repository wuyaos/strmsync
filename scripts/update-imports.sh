#!/usr/bin/env bash
# 更新import路径脚本

set -euo pipefail

echo "=== 更新import路径 ==="

# 定义sed函数（兼容GNU和BSD）
sedi() {
  if sed --version >/dev/null 2>&1; then
    sed -i "$1" "$2"
  else
    sed -i '' "$1" "$2"
  fi
}

echo "正在扫描并更新.go文件..."

find backend -type f -name "*.go" | while read -r file; do
  # 更新import路径（去掉/internal/前缀，不加/backend/前缀）
  sedi 's|github.com/strmsync/strmsync/internal/config|github.com/strmsync/strmsync/core|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/database|github.com/strmsync/strmsync/core|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/handlers|github.com/strmsync/strmsync/handler|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/types|github.com/strmsync/strmsync/service|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/job|github.com/strmsync/strmsync/service|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/taskrun|github.com/strmsync/strmsync/service|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/planner|github.com/strmsync/strmsync/service|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/executor|github.com/strmsync/strmsync/service|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/strm|github.com/strmsync/strmsync/service|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/filemonitor|github.com/strmsync/strmsync/service|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/dataserver|github.com/strmsync/strmsync/dataserver|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service/mediaserver|github.com/strmsync/strmsync/mediaserver|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/service|github.com/strmsync/strmsync/service|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/clients/clouddrive2/pb|github.com/strmsync/strmsync/dataserver/clouddrive2_proto|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/clients/clouddrive2|github.com/strmsync/strmsync/dataserver|g' "$file"
  sedi 's|github.com/strmsync/strmsync/internal/utils|github.com/strmsync/strmsync/utils|g' "$file"
done

echo "✓ import路径更新完成"

echo ""
echo "=== 验证编译 ==="
cd backend
if go build -o /tmp/strmsync-test ./main.go 2>&1; then
  echo "✓ 编译成功！"
  rm -f /tmp/strmsync-test
else
  echo "✗ 编译失败，请检查错误"
  exit 1
fi
