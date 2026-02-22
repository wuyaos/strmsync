#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

set -a
if [ -f "${ROOT_DIR}/.env" ]; then
  # shellcheck disable=SC1091
  . "${ROOT_DIR}/.env"
fi
if [ -f "${ROOT_DIR}/.env.test" ]; then
  # shellcheck disable=SC1091
  . "${ROOT_DIR}/.env.test"
fi
set +a

PORT="${PORT:-6786}"

BUILD_DIR="${ROOT_DIR}/build"
GO_BUILD_DIR="${BUILD_DIR}/go"
GO_CACHE_DIR="${GO_BUILD_DIR}/cache"
GO_MOD_CACHE_DIR="${GO_BUILD_DIR}/mod"
GO_TMP_DIR="${GO_BUILD_DIR}/tmp"
AIR_TMP_DIR="${BUILD_DIR}/air"
BACKEND_TMP_DIR="${ROOT_DIR}/backend/tmp"

LOG_DIR="${ROOT_DIR}/tests/logs"

safe_rm_dir() {
  local target="$1"
  if [ -z "${target}" ] || [ "${target}" = "/" ]; then
    return 1
  fi
  rm -rf "${target}" 2>/dev/null || true
  if [ -d "${target}" ]; then
    find "${target}" -mindepth 1 -exec rm -rf {} + 2>/dev/null || true
    rmdir "${target}" 2>/dev/null || true
  fi
}

echo "╔════════════════════════════════════════════════════════════╗"
echo "║          STRMSync 后端开发环境 - 独立启动                  ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

echo "==> 清理后端临时文件（保留编译缓存）..."
safe_rm_dir "${GO_TMP_DIR}"
safe_rm_dir "${AIR_TMP_DIR}"
safe_rm_dir "${BACKEND_TMP_DIR}"

mkdir -p "${GO_CACHE_DIR}" "${GO_MOD_CACHE_DIR}" "${GO_TMP_DIR}"
mkdir -p "${AIR_TMP_DIR}"
mkdir -p "${ROOT_DIR}/tests/.cache"
mkdir -p "${LOG_DIR}"

echo "==> 检查依赖..."
if ! command -v air >/dev/null 2>&1; then
  echo "✗ 未找到 air，请先安装：go install github.com/air-verse/air@latest"
  exit 1
fi

echo "==> 清理后端端口..."
if command -v lsof > /dev/null; then
  lsof -nP -t -iTCP:${PORT} -sTCP:LISTEN 2>/dev/null | xargs -r kill -9 2>/dev/null || true
fi

echo "==> 启动后端服务（前台）(http://localhost:${PORT})..."
echo "==> 按 Ctrl+C 停止后端服务"
echo ""

cd "${ROOT_DIR}"
GOMODCACHE="${GO_MOD_CACHE_DIR}" \
  GOCACHE="${GO_CACHE_DIR}" \
  GOTMPDIR="${GO_TMP_DIR}" \
  CGO_ENABLED=1 \
  GOMAXPROCS="$(nproc)" \
  air -c ".air.toml"
