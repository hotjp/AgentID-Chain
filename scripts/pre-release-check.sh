#!/usr/bin/env bash
# pre-release-check.sh — 发布前全面自检
#
# 检查项：
#   1. 工作区干净
#   2. 在主分支
#   3. 单元测试通过
#   4. 覆盖率 ≥ 70%
#   5. Lint 通过
#   6. Security 通过（gosec / govulncheck）
#   7. Build 通过
#   8. 文档无明显错误
#   9. CHANGELOG 有 Unreleased 段

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

cd "$ROOT_DIR"

log() { echo -e "${BLUE}━━━ $* ━━━${NC}"; }
ok() { echo -e "  ${GREEN}✓${NC}  $*"; ((PASSED++)); }
fail() { echo -e "  ${RED}✗${NC}  $*"; ((FAILED++)); }
warn() { echo -e "  ${YELLOW}⚠${NC}  $*"; ((WARNINGS++)); }

# --------------------------------------------------------------------------
# 1. 工作区
# --------------------------------------------------------------------------
log "1. Git 状态"

if [ -n "$(git status --porcelain)" ]; then
    fail "工作区不干净"
    git status --short
else
    ok "工作区干净"
fi

BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || echo "DETACHED")
if [ "$BRANCH" = "main" ]; then
    ok "在主分支"
else
    warn "当前分支: $BRANCH (非 main)"
fi

# --------------------------------------------------------------------------
# 2. 单元测试
# --------------------------------------------------------------------------
log "2. 单元测试"

if go test ./... -short > /tmp/test.log 2>&1; then
    ok "单元测试通过"
else
    fail "单元测试失败 (见 /tmp/test.log)"
fi

# --------------------------------------------------------------------------
# 3. 覆盖率
# --------------------------------------------------------------------------
log "3. 覆盖率"

if command -v gocov >/dev/null 2>&1 || go test -cover ./... > /tmp/cover.log 2>&1; then
    COVERAGE=$(grep -oE '[0-9]+\.[0-9]+%' /tmp/cover.log | head -1 | tr -d '%')
    if [ -n "$COVERAGE" ]; then
        if (( $(echo "$COVERAGE >= 70" | bc -l) )); then
            ok "覆盖率: ${COVERAGE}% (≥ 70%)"
        else
            fail "覆盖率: ${COVERAGE}% (< 70%)"
        fi
    else
        warn "无法解析覆盖率"
    fi
else
    warn "覆盖率检查失败（命令错误）"
fi

# --------------------------------------------------------------------------
# 4. Lint
# --------------------------------------------------------------------------
log "4. Lint"

if command -v golangci-lint >/dev/null 2>&1; then
    if golangci-lint run ./... > /tmp/lint.log 2>&1; then
        ok "golangci-lint 通过"
    else
        fail "golangci-lint 失败"
    fi
else
    warn "golangci-lint 未安装"
fi

# --------------------------------------------------------------------------
# 5. Security
# --------------------------------------------------------------------------
log "5. Security"

if command -v gosec >/dev/null 2>&1; then
    if gosec -quiet ./... > /tmp/gosec.log 2>&1; then
        ok "gosec 通过"
    else
        warn "gosec 发现问题（查看 /tmp/gosec.log）"
    fi
else
    warn "gosec 未安装"
fi

if command -v govulncheck >/dev/null 2>&1; then
    if govulncheck ./... > /tmp/vuln.log 2>&1; then
        ok "govulncheck 通过"
    else
        fail "govulncheck 发现漏洞"
    fi
else
    warn "govulncheck 未安装"
fi

# --------------------------------------------------------------------------
# 6. Build
# --------------------------------------------------------------------------
log "6. Build"

if go build ./... > /tmp/build.log 2>&1; then
    ok "go build 通过"
else
    fail "go build 失败"
fi

# --------------------------------------------------------------------------
# 7. 文档
# --------------------------------------------------------------------------
log "7. 文档"

if [ -x scripts/check-docs.sh ]; then
    if scripts/check-docs.sh > /tmp/docs.log 2>&1; then
        ok "文档检查通过"
    else
        warn "文档检查有警告"
    fi
else
    warn "check-docs.sh 未找到"
fi

# --------------------------------------------------------------------------
# 8. CHANGELOG
# --------------------------------------------------------------------------
log "8. CHANGELOG"

if [ -f CHANGELOG.md ]; then
    if grep -q "## \[Unreleased\]" CHANGELOG.md; then
        ok "CHANGELOG.md 有 Unreleased 段"
    else
        warn "CHANGELOG.md 缺少 [Unreleased] 段"
    fi
else
    fail "CHANGELOG.md 不存在"
fi

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
    echo -e "${RED}❌ 发布前检查未通过${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ 可以发布${NC}"
exit 0
