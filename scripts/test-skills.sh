#!/usr/bin/env bash
# test-skills.sh — 验证所有 skills 可加载
#
# 检查项：
#   1. JSON schema 合法
#   2. 必填字段存在
#   3. SKILL.md 存在
#   4. examples 目录有至少 1 个示例

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_DIR="$ROOT_DIR/skills"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

PASSED=0
FAILED=0
SKIPPED=0

echo "Testing AgentID-Chain Skills..."
echo ""

for skill_dir in "$SKILLS_DIR"/*/; do
    [ ! -d "$skill_dir" ] && continue

    skill_name=$(basename "$skill_dir")
    test_script="$skill_dir/tests/test_schema.sh"

    if [ -f "$test_script" ]; then
        if "$test_script" > /dev/null 2>&1; then
            echo -e "  ${GREEN}✓${NC} $skill_name"
            ((PASSED++))
        else
            echo -e "  ${RED}✗${NC} $skill_name"
            "$test_script" 2>&1 | sed 's/^/      /'
            ((FAILED++))
        fi
    else
        echo -e "  ⚠ $skill_name (no test script)"
        ((SKIPPED++))
    fi
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "Passed:  ${GREEN}$PASSED${NC}"
echo -e "Failed:  ${RED}$FAILED${NC}"
echo "Skipped: $SKIPPED"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

[ "$FAILED" -eq 0 ] && exit 0 || exit 1
