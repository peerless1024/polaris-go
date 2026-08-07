#!/bin/bash
# =============================================================================
# 清理 cloud 物料产物(dist/、zip 包、临时二进制)
#
# 使用方法:
#   ./cloud-clean.sh            # 默认: 展示后确认再清理
#   ./cloud-clean.sh -f         # 强制直接清理
#   ./cloud-clean.sh --dry-run  # 仅展示，不执行
# =============================================================================

set -euo pipefail

FORCE=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        -f|--force)  FORCE=true;   shift ;;
        --dry-run)   DRY_RUN=true; shift ;;
        -h|--help)
            echo "用法: $0 [-f|--force] [--dry-run]"
            echo "  -f, --force   强制清理，不交互确认"
            echo "  --dry-run     仅展示待清理项，不执行"
            exit 0
            ;;
        *)
            echo "未知参数: $1"; exit 1 ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${SCRIPT_DIR}/dist"
TMP_BIN="${SCRIPT_DIR}/x86-bin"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo ""
echo "待清理项:"
echo "  dist 目录:    ${DIST_DIR} ($([[ -d "$DIST_DIR" ]] && echo 存在 || echo 不存在))"
echo "  临时二进制:   ${TMP_BIN} ($([[ -f "$TMP_BIN" ]] && echo 存在 || echo 不存在))"
if [[ -d "$DIST_DIR" ]]; then
    echo "  dist 内容:"
    ls -1 "$DIST_DIR" 2>/dev/null | sed 's/^/    /'
fi
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
    echo -e "${YELLOW}[dry-run] 仅展示，未清理${NC}"
    exit 0
fi

if [[ "$FORCE" != "true" ]]; then
    read -r -p "确认清理以上产物? [y/N] " ans
    [[ "$ans" =~ ^[Yy]$ ]] || { echo "已取消"; exit 0; }
fi

rm -rf "$DIST_DIR" "$TMP_BIN"
echo -e "${GREEN}已清理 cloud 物料产物${NC}"
echo "清理完成。"
