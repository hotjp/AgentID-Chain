#!/usr/bin/env bash
# constitution-check.sh — 验证项目宪法遵守情况
#
# 检查项：
#   1. L2 Domain 零第三方依赖
#   2. 关键文件存在
#   3. ADR 格式正确
#   4. CHANGELOG.md 存在
#   5. Conventional Commits 历史
#   6. 必填文档存在

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASSED=0
FAILED=0
WARNINGS=0

log() { echo -e "${BLUE}━━━ $* ━━━${NC}"; }
ok() { echo -e "  ${GREEN}✓${NC} $*"; ((PASSED++)); }
fail() { echo -e "  ${RED}✗${NC} $*"; ((FAILED++)); }
warn() { echo -e "  ${YELLOW}⚠${NC} $*"; ((WARNINGS++)); }

cd "$ROOT_DIR"

# --------------------------------------------------------------------------
# 1. L2 Domain 零第三方依赖
# --------------------------------------------------------------------------
log "1. L2 Domain 零依赖铁律"

if [ -d "internal/domain" ]; then
    third_party=$(go list -f '{{ join .Imports "\n" }}' ./internal/domain/... 2>/dev/null | \
                  grep -E '^(github\.com|golang\.org|gopkg\.in|entgo\.io)' || true)
    if [ -z "$third_party" ]; then
        ok "L2 Domain 零第三方依赖"
    else
        fail "L2 Domain 包含第三方依赖："
        echo "$third_party" | sed 's/^/      /'
    fi
else
    warn "internal/domain/ 不存在"
fi

# --------------------------------------------------------------------------
# 2. 关键文件
# --------------------------------------------------------------------------
log "2. 关键文件"

for f in README.md CLAUDE.md CHANGELOG.md LICENSE; do
    if [ -f "$f" ]; then
        ok "$f"
    else
        fail "$f 不存在"
    fi
done

# --------------------------------------------------------------------------
# 3. ADR 格式
# --------------------------------------------------------------------------
log "3. ADR 格式"

if [ -d "docs/architecture/adr" ]; then
    adr_count=$(find docs/architecture/adr -name '[0-9][0-9][0-9][0-9]-*.md' -type f | wc -l | tr -d ' ')
    if [ "$adr_count" -gt 0 ]; then
        ok "$adr_count 个 ADR"

        # 每个 ADR 必填字段
        for adr in docs/architecture/adr/[0-9][0-9][0-9][0-9]-*.md; do
            [ ! -f "$adr" ] && continue
            for field in "## 状态" "## 上下文" "## 决策" "## 后果"; do
                if ! grep -q "^$field" "$adr"; then
                    fail "$(basename $adr) 缺少: $field"
                fi
            done
        done
    else
        warn "无 ADR"
    fi
else
    warn "docs/architecture/adr/ 不存在"
fi

# --------------------------------------------------------------------------
# 4. 文档完整性
# --------------------------------------------------------------------------
log "4. 关键文档"

REQUIRED_DOCS=(
    "docs/README.md"
    "docs/SUMMARY.md"
    "docs/architecture/overview.md"
    "docs/api/openapi.md"
    "docs/operations/deployment.md"
    "docs/operations/troubleshooting.md"
    "docs/operations/release-process.md"
    "docs/guides/quickstart.md"
    "docs/runbooks/README.md"
    "docs/contributing/development.md"
    "docs/SECURITY_AUDIT.md"
    "docs/SLO.md"
)

for doc in "${REQUIRED_DOCS[@]}"; do
    if [ -f "$doc" ]; then
        ok "$doc"
    else
        warn "$doc 不存在"
    fi
done

# --------------------------------------------------------------------------
# 5. Conventional Commits
# --------------------------------------------------------------------------
log "5. 提交规范"

if command -v git >/dev/null 2>&1; then
    recent_commits=$(git log -50 --pretty=%s 2>/dev/null)
    bad_count=$(echo "$recent_commits" | grep -vE '^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z]+\))?!?: ' | wc -l | tr -d ' ')
    total=$(echo "$recent_commits" | wc -l | tr -d ' ')
    if [ "$total" -gt 0 ]; then
        rate=$(echo "scale=0; (($total - $bad_count) * 100) / $total" | bc)
        if [ "$bad_count" -eq 0 ]; then
            ok "100% 提交符合 Conventional Commits"
        elif [ "$rate" -ge 80 ]; then
            ok "${rate}% 提交符合规范 (${bad_count}/${total} 不符合)"
        else
            warn "${rate}% 提交符合规范 (${bad_count}/${total} 不符合)"
        fi
    fi
else
    warn "git 不可用"
fi

# --------------------------------------------------------------------------
# 6. 关键脚本可执行
# --------------------------------------------------------------------------
log "6. 关键脚本"

SCRIPTS=(
    "scripts/check-docs.sh"
    "scripts/test-skills.sh"
    "scripts/test-prompts.sh"
    "scripts/release.sh"
    "scripts/pre-release-check.sh"
)

for script in "${SCRIPTS[@]}"; do
    if [ -f "$script" ]; then
        if [ -x "$script" ]; then
            ok "$script (executable)"
        else
            warn "$script 存在但不可执行"
        fi
    else
        warn "$script 不存在"
    fi
done

# --------------------------------------------------------------------------
# 7. 治理文件
# --------------------------------------------------------------------------
log "7. 治理文件"

for f in ".long-run-agent/constitution.yaml" ".long-run-agent/governance.md" ".long-run-agent/quarterly-review.md"; do
    if [ -f "$f" ]; then
        ok "$f"
    else
        warn "$f 不存在"
    fi
done

# --------------------------------------------------------------------------
# 总结
# --------------------------------------------------------------------------
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "  Passed:   ${GREEN}$PASSED${NC}"
echo -e "  Failed:   ${RED}$FAILED${NC}"
echo -e "  Warnings: ${YELLOW}$WARNINGS${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ "$FAILED" -gt 0 ]; then
    echo ""
    echo -e "${RED}❌ Constitution 检查未通过${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ Constitution 检查通过${NC}"
exit 0
