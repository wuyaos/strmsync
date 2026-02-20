#!/usr/bin/env bash
# 生产环境启动脚本（合并部署）
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BIN_CANDIDATES=(
  "${SCRIPT_DIR}/strmsync"
  "${SCRIPT_DIR}/../dist/strmsync"
)

for bin in "${BIN_CANDIDATES[@]}"; do
  if [ -x "$bin" ]; then
    BIN_PATH="$bin"
    break
  fi
done

if [ -z "${BIN_PATH:-}" ]; then
  echo "未找到可执行文件: strmsync"
  exit 1
fi

RUN_DIR="$(cd "$(dirname "$BIN_PATH")" && pwd)"

if [ ! -f "${RUN_DIR}/.env" ]; then
  echo "未检测到 ${RUN_DIR}/.env，请先配置 ENCRYPTION_KEY 等参数"
fi

cd "$RUN_DIR"
echo "==> 运行 strmsync..."
exec "./$(basename "$BIN_PATH")"
