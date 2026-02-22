#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v git >/dev/null; then
  echo "未找到 git，请先安装 git。"
  exit 1
fi

if [ ! -d "${ROOT_DIR}/.githooks" ]; then
  echo "未找到 ${ROOT_DIR}/.githooks，请确认仓库完整。"
  exit 1
fi

git -C "${ROOT_DIR}" config core.hooksPath ".githooks"

if [ -f "${ROOT_DIR}/.githooks/pre-commit" ]; then
  chmod +x "${ROOT_DIR}/.githooks/pre-commit"
fi

echo "[OK] 已配置 Git Hooks: core.hooksPath=.githooks"
