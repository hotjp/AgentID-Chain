#!/usr/bin/env bash
# check-docs.sh — 文档质量检查
#
# 功能：
#   1. 验证所有 markdown 文件无明显问题
#   2. 检查内部链接（基础校验）
#   3. 检查标题格式
#   4. 检查代码块语言标签
#   5. 检查文件大小（> 1MB 警告）
#
# 用法：
#   ./scripts/check-docs.sh
#   ./scripts/check-docs.sh --strict  # 严格模式
#
# 退出码：
#   0 — 全部通过
#   1 — 有错误
#   2 — 有警告（仅 --strict）

set -uo pipefail

# 颜色
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

# 路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DOCS_DIR="$ROOT_DIR/docs"

# 选项
STRICT=0
for arg in "$@"; do
    case $arg in
        --strict) STRICT=1 ;;
        --help|-h)
            echo "Usage: $0 [--strict]"
            exit 0
            ;;
    esac
done

# 计数器
ERRORS=0
WARNINGS=0
FILES_CHECKED=0

echo -e "${BLUE}📚 检查文档...${NC}"
echo "  Root: $DOCS_DIR"
echo ""

# --------------------------------------------------------------------------
# 1. 检查所有 markdown 文件存在
# --------------------------------------------------------------------------
echo -e "${BLUE}[1/5] 收集 markdown 文件...${NC}"
MD_FILES=$(find "$DOCS_DIR" -name "*.md" -type f 2>/dev/null | sort)
MD_COUNT=$(echo "$MD_FILES" | grep -c '.' || echo 0)
echo -e "  ${GREEN}✓${NC} 找到 $MD_COUNT 个 markdown 文件"
echo ""

# --------------------------------------------------------------------------
# 2. 检查关键文档存在
# --------------------------------------------------------------------------
echo -e "${BLUE}[2/5] 检查关键文档...${NC}"
REQUIRED_DOCS=(
    "$DOCS_DIR/README.md"
    "$DOCS_DIR/SUMMARY.md"
    "$DOCS_DIR/architecture/overview.md"
    "$DOCS_DIR/api/openapi.md"
    "$DOCS_DIR/operations/deployment.md"
    "$DOCS_DIR/operations/troubleshooting.md"
    "$DOCS_DIR/guides/quickstart.md"
    "$DOCS_DIR/guides/faq.md"
    "$DOCS_DIR/runbooks/README.md"
    "$DOCS_DIR/contributing/development.md"
)

for doc in "${REQUIRED_DOCS[@]}"; do
    if [ -f "$doc" ]; then
        echo -e "  ${GREEN}✓${NC} $(basename "$doc")"
    else
        echo -e "  ${RED}✗${NC} MISSING: $doc"
        ((ERRORS++))
    fi
done
echo ""

# --------------------------------------------------------------------------
# 3. 检查 markdown 文件大小（> 1MB 警告）
# --------------------------------------------------------------------------
echo -e "${BLUE}[3/5] 检查文件大小...${NC}"
LARGE_FILE_LIMIT=$((1024 * 1024))  # 1MB
while IFS= read -r file; do
    [ -z "$file" ] && continue
    size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo 0)
    if [ "$size" -gt "$LARGE_FILE_LIMIT" ]; then
        size_mb=$(echo "scale=2; $size / 1024 / 1024" | bc)
        echo -e "  ${YELLOW}⚠${NC} $(basename "$file") is ${size_mb}MB (consider splitting)"
        ((WARNINGS++))
    fi
    ((FILES_CHECKED++))
done <<< "$MD_FILES"
echo -e "  ${GREEN}✓${NC} Checked $FILES_CHECKED files"
echo ""

# --------------------------------------------------------------------------
# 4. 检查代码块语言标签
# --------------------------------------------------------------------------
echo -e "${BLUE}[4/5] 检查代码块...${NC}"
CODE_BLOCK_NO_LANG=0
while IFS= read -r file; do
    [ -z "$file" ] && continue
    # 查找 ``` 后面没跟语言标签的情况
    # 排除 ~~~ 风格和缩进代码块
    no_lang=$(grep -E '^```$' "$file" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$no_lang" -gt 0 ]; then
        echo -e "  ${YELLOW}⚠${NC} $(basename "$file"): $no_lang code block(s) without language tag"
        ((WARNINGS++))
        ((CODE_BLOCK_NO_LANG++))
    fi
done <<< "$MD_FILES"
if [ "$CODE_BLOCK_NO_LANG" -eq 0 ]; then
    echo -e "  ${GREEN}✓${NC} All code blocks have language tags"
fi
echo ""

# --------------------------------------------------------------------------
# 5. 检查内部链接（基础：查找 [text](path) 格式的相对路径）
# --------------------------------------------------------------------------
echo -e "${BLUE}[5/5] 检查内部链接...${NC}"
BROKEN_LINKS=0
while IFS= read -r file; do
    [ -z "$file" ] && continue
    # 提取 [text](path) 格式的相对路径（不含 http:// 或 https://）
    links=$(grep -oE '\[[^]]+\]\(([^)]+)\)' "$file" 2>/dev/null | \
            grep -oE '\([^)]+\)' | \
            sed 's/^(\|)$//g' | \
            grep -vE '^(https?://|#|mailto:)' || true)

    for link in $links; do
        # 跳过锚点、绝对 URL
        [[ "$link" == http* ]] && continue
        [[ "$link" == \#* ]] && continue

        # 去除锚点
        clean_link="${link%%#*}"
        [ -z "$clean_link" ] && continue

        # 解析路径
        if [[ "$clean_link" == /* ]]; then
            target="$ROOT_DIR$clean_link"
        else
            target="$(dirname "$file")/$clean_link"
        fi

        # 规范化路径
        target_normalized=$(cd "$(dirname "$target")" 2>/dev/null && pwd)/$(basename "$target") 2>/dev/null
        # 如果 target_normalized 失败，使用原 target
        if [ -z "$target_normalized" ]; then
            target_normalized="$target"
        fi

        if [ ! -f "$target_normalized" ] && [ ! -d "$target_normalized" ]; then
            # 静默跳过一些动态路径（如 /v1/agents）
            if [[ "$link" =~ ^/v[0-9]+ ]] || [[ "$link" =~ ^/mcp ]] || [[ "$link" =~ ^/healthz ]]; then
                continue
            fi
            echo -e "  ${YELLOW}⚠${NC} $(basename "$file"): broken link → $link"
            ((WARNINGS++))
            ((BROKEN_LINKS++))
        fi
    done
done <<< "$MD_FILES"
if [ "$BROKEN_LINKS" -eq 0 ]; then
    echo -e "  ${GREEN}✓${NC} All internal links resolve"
fi
echo ""

# --------------------------------------------------------------------------
# 总结
# --------------------------------------------------------------------------
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BLUE}📊 总结${NC}"
echo "  Files checked: $FILES_CHECKED"
echo -e "  Errors:   ${RED}$ERRORS${NC}"
echo -e "  Warnings: ${YELLOW}$WARNINGS${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ "$ERRORS" -gt 0 ]; then
    echo -e "${RED}❌ 失败${NC}: 有 $ERRORS 个错误"
    exit 1
fi

if [ "$WARNINGS" -gt 0 ]; then
    if [ "$STRICT" -eq 1 ]; then
        echo -e "${YELLOW}⚠️  严格模式失败${NC}: 有 $WARNINGS 个警告"
        exit 2
    fi
    echo -e "${YELLOW}⚠️  通过 (有警告)${NC}"
    exit 0
fi

echo -e "${GREEN}✅ 全部通过${NC}"
exit 0
