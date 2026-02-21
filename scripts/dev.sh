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
FRONTEND_PORT="${FRONTEND_PORT:-7786}"
VITE_BACKEND_PORT="${VITE_BACKEND_PORT:-${PORT}}"

BUILD_DIR="${ROOT_DIR}/build"
GO_BUILD_DIR="${BUILD_DIR}/go"
GO_CACHE_DIR="${GO_BUILD_DIR}/cache"
GO_MOD_CACHE_DIR="${GO_BUILD_DIR}/mod"
GO_TMP_DIR="${GO_BUILD_DIR}/tmp"
AIR_TMP_DIR="${BUILD_DIR}/air"
VUE_BUILD_DIR="${BUILD_DIR}/vue"
VUE_CACHE_DIR="${VUE_BUILD_DIR}/.vite"
NPM_CACHE_DIR="${VUE_BUILD_DIR}/npm-cache"

BACKEND_TMP_DIR="${ROOT_DIR}/backend/tmp"
LOG_DIR="${ROOT_DIR}/tests/logs"
FRONTEND_LOG="${LOG_DIR}/vite.log"

FRONTEND_PID=""

cleanup() {
  echo ""
  echo "==> 正在停止前端服务..."

  # 停止前端
  if [ -n "${FRONTEND_PID}" ] && kill -0 "${FRONTEND_PID}" 2>/dev/null; then
    kill "${FRONTEND_PID}" 2>/dev/null || true
    wait "${FRONTEND_PID}" 2>/dev/null || true
  fi

  # 确保前端端口被释放
  if command -v lsof > /dev/null; then
    lsof -nP -t -iTCP:${FRONTEND_PORT} -sTCP:LISTEN 2>/dev/null | xargs -r kill -9 2>/dev/null || true
  fi

  echo "✓ 前端服务已停止"
}
trap cleanup EXIT INT TERM

echo "╔════════════════════════════════════════════════════════════╗"
echo "║          STRMSync 开发环境 - 一键启动                      ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

echo "==> 清理临时文件（保留编译缓存以加速重启）..."
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

# 只清理临时文件，保留编译缓存
safe_rm_dir "${GO_TMP_DIR}"
safe_rm_dir "${AIR_TMP_DIR}"
safe_rm_dir "${BACKEND_TMP_DIR}"

# 保留编译缓存以加速重启（重要！）
# safe_rm_dir "${GO_CACHE_DIR}"    # 保留 Go 编译缓存（包括 sqlite3）
# safe_rm_dir "${VUE_CACHE_DIR}"   # 保留 Vite 缓存

mkdir -p "${VUE_BUILD_DIR}" "${VUE_CACHE_DIR}" "${NPM_CACHE_DIR}"
mkdir -p "${GO_CACHE_DIR}" "${GO_MOD_CACHE_DIR}" "${GO_TMP_DIR}"
mkdir -p "${AIR_TMP_DIR}"
mkdir -p "${ROOT_DIR}/tests/.cache"
mkdir -p "${LOG_DIR}"

echo "==> 检查依赖..."
if ! command -v air >/dev/null 2>&1; then
  echo "✗ 未找到 air，请先安装：go install github.com/air-verse/air@latest"
  exit 1
fi

echo "==> 清理前端进程和端口..."
if command -v lsof > /dev/null; then
  # 强制清理前端端口
  lsof -nP -t -iTCP:${FRONTEND_PORT} -sTCP:LISTEN 2>/dev/null | xargs -r kill -9 2>/dev/null || true
  # 清理后端端口
  lsof -nP -t -iTCP:${PORT} -sTCP:LISTEN 2>/dev/null | xargs -r kill -9 2>/dev/null || true
fi

echo "==> 启动前端服务（后台）(http://localhost:${FRONTEND_PORT})..."
(
  cd "${ROOT_DIR}/frontend"
  NPM_CONFIG_CACHE="${NPM_CACHE_DIR}" \
    VITE_BACKEND_PORT="${VITE_BACKEND_PORT}" \
    npm run dev -- --port "${FRONTEND_PORT}" >"${FRONTEND_LOG}" 2>&1
) &
FRONTEND_PID=$!

# 等待前端启动
sleep 3

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║  ✓ 前端服务已启动（后台运行）                              ║"
echo "╠════════════════════════════════════════════════════════════╣"
echo "║  前端: http://localhost:${FRONTEND_PORT}                              ║"
echo "║  日志: tail -f ${FRONTEND_LOG}                  ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "==> 启动后端服务（前台）(http://localhost:${PORT})..."
echo "==> 按 Ctrl+C 停止所有服务"
echo ""

cd "${ROOT_DIR}"
GOMODCACHE="${GO_MOD_CACHE_DIR}" \
  GOCACHE="${GO_CACHE_DIR}" \
  GOTMPDIR="${GO_TMP_DIR}" \
  CGO_ENABLED=1 \
  GOMAXPROCS=$(nproc) \
  air -c ".air.toml"
