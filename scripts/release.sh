#!/usr/bin/env bash
# release.sh — 语义化版本发布脚本
#
# 用法：
#   ./scripts/release.sh patch        # 0.0.x
#   ./scripts/release.sh minor        # 0.x.0
#   ./scripts/release.sh major        # x.0.0
#   ./scripts/release.sh v2.0.2       # 指定版本
#   ./scripts/release.sh --dry-run    # 演练
#
# 流程：
#   1. 验证：分支 / 状态 / 权限
#   2. 自检：test / lint / build
#   3. 版本号 bump
#   4. 更新 CHANGELOG
#   5. 提交 + tag
#   6. 构建 + 签名
#   7. 推送（可选）

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 选项
DRY_RUN=0
LEVEL=""
EXPLICIT_VERSION=""
SKIP_TESTS=0
SKIP_PUSH=0

for arg in "$@"; do
    case $arg in
        --dry-run) DRY_RUN=1 ;;
        --skip-tests) SKIP_TESTS=1 ;;
        --skip-push) SKIP_PUSH=1 ;;
        patch|minor|major) LEVEL="$arg" ;;
        v[0-9]*.[0-9]*.[0-9]*) EXPLICIT_VERSION="$arg" ;;
        --help|-h)
            echo "Usage: $0 [patch|minor|major|vX.Y.Z] [--dry-run] [--skip-tests] [--skip-push]"
            exit 0
            ;;
        *) echo "Unknown arg: $arg"; exit 1 ;;
    esac
done

# --------------------------------------------------------------------------
# 工具
# --------------------------------------------------------------------------
log_info()  { echo -e "${BLUE}ℹ${NC}  $*"; }
log_ok()    { echo -e "${GREEN}✓${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}⚠${NC}  $*"; }
log_error() { echo -e "${RED}✗${NC}  $*"; }

# --------------------------------------------------------------------------
# 1. 验证
# --------------------------------------------------------------------------
echo -e "${BLUE}━━━ 步骤 1/7: 验证环境 ━━━${NC}"

# 在 git 仓库
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    log_error "Not in a git repository"
    exit 1
fi

cd "$ROOT_DIR"

# 主分支
BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || git rev-parse --short HEAD)
if [ "$BRANCH" != "main" ] && [ "$DRY_RUN" -eq 0 ]; then
    log_warn "Not on main branch (current: $BRANCH)"
    read -p "Continue? [y/N] " -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]] || exit 1
fi

# 工作区干净
if [ -n "$(git status --porcelain)" ] && [ "$DRY_RUN" -eq 0 ]; then
    log_error "Working tree not clean. Commit or stash first."
    git status --short
    exit 1
fi

log_ok "环境验证通过"

# --------------------------------------------------------------------------
# 2. 解析版本号
# --------------------------------------------------------------------------
echo -e "${BLUE}━━━ 步骤 2/7: 解析版本 ━━━${NC}"

# 从 git tag 拿最近版本
LATEST_TAG=$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1)

if [ -z "$LATEST_TAG" ]; then
    LATEST_VERSION="0.0.0"
else
    LATEST_VERSION="${LATEST_TAG#v}"
fi

log_info "当前版本: $LATEST_VERSION"

if [ -n "$EXPLICIT_VERSION" ]; then
    NEW_VERSION="${EXPLICIT_VERSION#v}"
elif [ -n "$LEVEL" ]; then
    IFS='.' read -r MAJOR MINOR PATCH <<< "$LATEST_VERSION"
    case "$LEVEL" in
        major) NEW_VERSION="$((MAJOR + 1)).0.0" ;;
        minor) NEW_VERSION="$MAJOR.$((MINOR + 1)).0" ;;
        patch) NEW_VERSION="$MAJOR.$MINOR.$((PATCH + 1))" ;;
    esac
else
    log_error "需要指定 patch / minor / major / vX.Y.Z"
    exit 1
fi

NEW_TAG="v$NEW_VERSION"
log_ok "新版本: $NEW_VERSION (tag: $NEW_TAG)"

if [ "$DRY_RUN" -eq 1 ]; then
    log_warn "DRY RUN: 不会做任何修改"
fi

# --------------------------------------------------------------------------
# 3. 自检
# --------------------------------------------------------------------------
echo -e "${BLUE}━━━ 步骤 3/7: 自检 ━━━${NC}"

if [ "$SKIP_TESTS" -eq 0 ]; then
    log_info "运行单元测试..."
    if [ "$DRY_RUN" -eq 0 ]; then
        go test ./... -short || { log_error "测试失败"; exit 1; }
    fi
    log_ok "测试通过"

    log_info "运行 lint..."
    if [ "$DRY_RUN" -eq 0 ] && command -v golangci-lint >/dev/null 2>&1; then
        golangci-lint run ./... || { log_error "Lint 失败"; exit 1; }
    fi
    log_ok "Lint 通过"
else
    log_warn "跳过测试（--skip-tests）"
fi

# --------------------------------------------------------------------------
# 4. 更新 CHANGELOG
# --------------------------------------------------------------------------
echo -e "${BLUE}━━━ 步骤 4/7: 更新 CHANGELOG ━━━${NC}"

if [ "$DRY_RUN" -eq 0 ]; then
    TODAY=$(date +%Y-%m-%d)
    # 在 [Unreleased] 段落后插入新版本
    if grep -q "## \[Unreleased\]" CHANGELOG.md; then
        # 用 sed 在 [Unreleased] 段后插入新版本头
        awk -v tag="$NEW_TAG" -v date="$TODAY" '
            /^## \[Unreleased\]/ {
                print
                print ""
                print "## [" substr(tag, 2) "] - " date
                print ""
                next
            }
            { print }
        ' CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
    fi
fi
log_ok "CHANGELOG.md 更新"

# --------------------------------------------------------------------------
# 5. 提交 + tag
# --------------------------------------------------------------------------
echo -e "${BLUE}━━━ 步骤 5/7: 提交 + tag ━━━${NC}"

if [ "$DRY_RUN" -eq 0 ]; then
    git add CHANGELOG.md
    if [ -n "$(git status --porcelain)" ]; then
        git commit -m "chore(release): $NEW_TAG"
    fi

    git tag -a "$NEW_TAG" -m "Release $NEW_TAG"
    log_ok "已创建 tag: $NEW_TAG"
else
    log_warn "DRY RUN: 跳过 commit / tag"
fi

# --------------------------------------------------------------------------
# 6. 构建（distroless + 签名）
# --------------------------------------------------------------------------
echo -e "${BLUE}━━━ 步骤 6/7: 构建 + 签名 ━━━${NC}"

if [ "$DRY_RUN" -eq 0 ]; then
    # 编译
    log_info "编译二进制..."
    mkdir -p dist
    GOOS=linux GOARCH=amd64 go build -o "dist/agentid-$NEW_VERSION-linux-amd64" ./cmd/agentid
    log_ok "已构建 dist/agentid-$NEW_VERSION-linux-amd64"

    # SBOM
    if command -v syft >/dev/null 2>&1; then
        log_info "生成 SBOM..."
        syft "dist/agentid-$NEW_VERSION-linux-amd64" \
            -o spdx-json \
            --file "dist/agentid-$NEW_VERSION-linux-amd64.spdx.json"
        log_ok "SBOM: dist/agentid-$NEW_VERSION-linux-amd64.spdx.json"
    fi

    # 签名
    if command -v cosign >/dev/null 2>&1; then
        log_info "Cosign 签名..."
        cosign sign-blob --yes \
            "dist/agentid-$NEW_VERSION-linux-amd64" \
            --output-signature "dist/agentid-$NEW_VERSION-linux-amd64.sig" \
            --output-certificate "dist/agentid-$NEW_VERSION-linux-amd64.pem" || \
            log_warn "Cosign 签名失败（可能未配置 key）"
    fi
fi

# --------------------------------------------------------------------------
# 7. 推送
# --------------------------------------------------------------------------
echo -e "${BLUE}━━━ 步骤 7/7: 推送 ━━━${NC}"

if [ "$DRY_RUN" -eq 0 ] && [ "$SKIP_PUSH" -eq 0 ]; then
    log_info "推送 commit + tag..."
    git push origin HEAD
    git push origin "$NEW_TAG"
    log_ok "已推送"
else
    log_warn "跳过推送（--skip-push 或 dry-run）"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}🎉 发布完成：$NEW_TAG${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "后续步骤："
echo "  1. 等待 GitHub Actions 构建 Docker 镜像"
echo "  2. 在 GitHub 创建 Release（含 dist/ 中的 artifact）"
echo "  3. 通知团队（Slack #releases）"
echo ""
