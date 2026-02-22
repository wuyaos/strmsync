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
FRONTEND_PID=""

cleanup() {
  echo ""
  echo "==> 正在停止前端服务..."

  # 停止前端
  if [ -n "${FRONTEND_PID}" ] && kill -0 "${FRONTEND_PID}" 2>/dev/null; then
    kill "${FRONTEND_PID}" 2>/dev/null || true
    wait "${FRONTEND_PID}" 2>/dev/null || true
  fi

  echo "✓ 前端服务已停止"
}
trap cleanup EXIT INT TERM

echo "╔════════════════════════════════════════════════════════════╗"
echo "║          STRMSync 开发环境 - 一键启动                      ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

echo "==> 启动前端服务（后台）..."
"${ROOT_DIR}/scripts/dev-frontend.sh" &
FRONTEND_PID=$!

echo "==> 启动后端服务（前台）..."
"${ROOT_DIR}/scripts/dev-backend.sh"
