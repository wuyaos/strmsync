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
VUE_BUILD_DIR="${BUILD_DIR}/vue"
VUE_CACHE_DIR="${VUE_BUILD_DIR}/.vite"
NPM_CACHE_DIR="${VUE_BUILD_DIR}/npm-cache"

LOG_DIR="${ROOT_DIR}/tests/logs"
FRONTEND_LOG="${LOG_DIR}/vite.log"

FRONTEND_PID=""

cleanup() {
  echo ""
  echo "==> 正在停止前端服务..."

  if [ -n "${FRONTEND_PID}" ] && kill -0 "${FRONTEND_PID}" 2>/dev/null; then
    kill "${FRONTEND_PID}" 2>/dev/null || true
    wait "${FRONTEND_PID}" 2>/dev/null || true
  fi

  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -t -iTCP:${FRONTEND_PORT} -sTCP:LISTEN 2>/dev/null | xargs -r kill -9 2>/dev/null || true
  elif command -v fuser >/dev/null 2>&1; then
    fuser -k "${FRONTEND_PORT}/tcp" 2>/dev/null || true
  elif command -v ss >/dev/null 2>&1; then
    pids="$(ss -ltnp 2>/dev/null | awk -v port=":${FRONTEND_PORT}" '$4 ~ port {print $6}' | sed -n 's/.*pid=\([0-9]\+\).*/\1/p' | sort -u)"
    if [ -n "${pids}" ]; then
      kill -9 ${pids} 2>/dev/null || true
    fi
  elif command -v netstat >/dev/null 2>&1; then
    pids="$(netstat -lntp 2>/dev/null | awk -v port=":${FRONTEND_PORT}" '$4 ~ port {print $7}' | sed -n 's#^\([0-9]\+\)/.*#\1#p' | sort -u)"
    if [ -n "${pids}" ]; then
      kill -9 ${pids} 2>/dev/null || true
    fi
  fi

  echo "✓ 前端服务已停止"
}
trap cleanup EXIT INT TERM

echo "╔════════════════════════════════════════════════════════════╗"
echo "║          STRMSync 前端开发环境 - 独立启动                  ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

echo "==> 准备前端缓存目录..."
mkdir -p "${VUE_BUILD_DIR}" "${VUE_CACHE_DIR}" "${NPM_CACHE_DIR}"
mkdir -p "${LOG_DIR}"

echo "==> 清理前端端口..."
if command -v lsof >/dev/null 2>&1; then
  lsof -nP -t -iTCP:${FRONTEND_PORT} -sTCP:LISTEN 2>/dev/null | xargs -r kill -9 2>/dev/null || true
elif command -v fuser >/dev/null 2>&1; then
  fuser -k "${FRONTEND_PORT}/tcp" 2>/dev/null || true
elif command -v ss >/dev/null 2>&1; then
  pids="$(ss -ltnp 2>/dev/null | awk -v port=":${FRONTEND_PORT}" '$4 ~ port {print $6}' | sed -n 's/.*pid=\([0-9]\+\).*/\1/p' | sort -u)"
  if [ -n "${pids}" ]; then
    kill -9 ${pids} 2>/dev/null || true
  fi
elif command -v netstat >/dev/null 2>&1; then
  pids="$(netstat -lntp 2>/dev/null | awk -v port=":${FRONTEND_PORT}" '$4 ~ port {print $7}' | sed -n 's#^\([0-9]\+\)/.*#\1#p' | sort -u)"
  if [ -n "${pids}" ]; then
    kill -9 ${pids} 2>/dev/null || true
  fi
fi

echo "==> 启动前端服务（后台）(http://localhost:${FRONTEND_PORT})..."
(
  cd "${ROOT_DIR}/frontend"
  NPM_CONFIG_CACHE="${NPM_CACHE_DIR}" \
    VITE_BACKEND_PORT="${VITE_BACKEND_PORT}" \
    npm run dev -- --port "${FRONTEND_PORT}" >"${FRONTEND_LOG}" 2>&1
) &
FRONTEND_PID=$!

sleep 2

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║  ✓ 前端服务已启动（后台运行）                              ║"
echo "╠════════════════════════════════════════════════════════════╣"
echo "║  前端: http://localhost:${FRONTEND_PORT}                              ║"
echo "║  日志: tail -f ${FRONTEND_LOG}                  ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "==> 按 Ctrl+C 停止前端服务"

tail -f "${FRONTEND_LOG}"
