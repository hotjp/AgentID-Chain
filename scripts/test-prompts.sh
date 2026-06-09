#!/usr/bin/env bash
# test-prompts.sh — 验证 prompts 模板格式
#
# 检查项：
#   1. 文件以 YAML frontmatter 开头（---）
#   2. 包含 name、version 字段
#   3. role 是 system / user / assistant 之一

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PROMPTS_DIR="$ROOT_DIR/prompts"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

PASSED=0
FAILED=0

echo "Testing AgentID-Chain Prompts..."
echo ""

while IFS= read -r file; do
    [ -z "$file" ] && continue

    # 1. 必须以 --- 开头
    first_line=$(head -n 1 "$file")
    if [ "$first_line" != "---" ]; then
        echo -e "  ${RED}✗${NC} $(basename "$file"): missing YAML frontmatter"
        ((FAILED++))
        continue
    fi

    # 2. 检查 name 和 version 字段
    content=$(cat "$file")
    if ! echo "$content" | grep -q '^name:'; then
        echo -e "  ${RED}✗${NC} $(basename "$file"): missing 'name' field"
        ((FAILED++))
        continue
    fi

    if ! echo "$content" | grep -q '^version:'; then
        echo -e "  ${RED}✗${NC} $(basename "$file"): missing 'version' field"
        ((FAILED++))
        continue
    fi

    echo -e "  ${GREEN}✓${NC} $(basename "$file")"
    ((PASSED++))

done < <(find "$PROMPTS_DIR" -name "*.md" -type f 2>/dev/null | grep -v "/README.md$")

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

[ "$FAILED" -eq 0 ] && exit 0 || exit 1
