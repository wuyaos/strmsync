#!/usr/bin/env bash
# CloudDrive2 API 文档更新脚本
set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 获取项目根目录
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCS_DIR="${ROOT_DIR}/docs"
API_DOC_URL="https://www.clouddrive2.com/api/CloudDrive2_gRPC_API_Guide_zh-CN.md"
OUTPUT_FILE="${DOCS_DIR}/CloudDrive2_API.md"

echo "========================================="
echo "CloudDrive2 API 文档更新"
echo "========================================="
echo ""

# 检查 curl 或 wget
DOWNLOADER=""
if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
else
    echo -e "${RED}错误: 未找到 curl 或 wget${NC}"
    echo ""
    echo "请安装下载工具："
    echo "  Ubuntu/Debian: sudo apt-get install curl"
    echo "  macOS:         brew install curl"
    exit 1
fi

echo -e "${GREEN}✓${NC} 使用下载工具: ${DOWNLOADER}"
echo ""

# 备份现有文档
if [ -f "${OUTPUT_FILE}" ]; then
    BACKUP_FILE="${OUTPUT_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
    echo "备份现有文档: ${BACKUP_FILE}"
    cp "${OUTPUT_FILE}" "${BACKUP_FILE}"
fi

# 下载文档
echo "正在下载 CloudDrive2 API 文档..."
echo "URL: ${API_DOC_URL}"
echo ""

if [ "${DOWNLOADER}" = "curl" ]; then
    if curl -fsSL -o "${OUTPUT_FILE}" "${API_DOC_URL}"; then
        echo -e "${GREEN}✓${NC} 下载成功"
    else
        echo -e "${RED}✗${NC} 下载失败"
        exit 1
    fi
else
    if wget -q -O "${OUTPUT_FILE}" "${API_DOC_URL}"; then
        echo -e "${GREEN}✓${NC} 下载成功"
    else
        echo -e "${RED}✗${NC} 下载失败"
        exit 1
    fi
fi

# 验证文件
if [ ! -s "${OUTPUT_FILE}" ]; then
    echo -e "${RED}✗${NC} 下载的文件为空"
    exit 1
fi

FILE_SIZE=$(wc -c < "${OUTPUT_FILE}" | tr -d ' ')
echo ""
echo "文件信息："
echo "  路径: ${OUTPUT_FILE}"
echo "  大小: ${FILE_SIZE} bytes"
echo ""

echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}CloudDrive2 API 文档更新完成！${NC}"
echo -e "${GREEN}=========================================${NC}"
