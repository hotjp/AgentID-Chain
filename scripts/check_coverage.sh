#!/usr/bin/env bash
# =============================================================================
# AgentID-Chain — 覆盖率门槛检查 (P15.10)
# =============================================================================
# 跑 go test -cover + 解析每个包的覆盖率，门槛不通过则 exit 1。
# =============================================================================

set -u

cd "$(dirname "$0")/.."

MIN=70
PKG="./..."

while [ "$#" -gt 0 ]; do
    case "$1" in
        --min) MIN="$2"; shift 2 ;;
        --pkg) PKG="$2"; shift 2 ;;
        *) echo "unknown arg: $1"; exit 2 ;;
    esac
done

echo "==> 覆盖率门槛: ${MIN}%"
echo "==> 检查包: ${PKG}"
echo ""

# 收集所有 "ok pkg X% of statements" 行
LINES=$(go test -cover "$PKG" 2>&1 | grep -E "^ok" || true)

declare -a FAILED_PKGS=()
declare -a PASSED_PKGS=()

while IFS= read -r line; do
    [ -z "$line" ] && continue
    pkg=$(echo "$line" | awk '{print $2}')
    cov=$(echo "$line" | sed -nE 's/.*coverage: ([0-9]+\.[0-9]+)%.*/\1/p')
    [ -z "$pkg" ] && continue
    [ -z "$cov" ] && continue

    pass=$(awk -v c="$cov" -v m="$MIN" 'BEGIN { print (c+0 >= m+0) ? 1 : 0 }')
    if [ "$pass" = "1" ]; then
        PASSED_PKGS+=("$pkg ${cov}%")
    else
        FAILED_PKGS+=("$pkg ${cov}%")
    fi
done <<< "$LINES"

echo "==> 通过的包（>= ${MIN}%）: ${#PASSED_PKGS[@]}"
i=0
for p in "${PASSED_PKGS[@]}"; do
    i=$((i+1))
    [ $i -gt 10 ] && break
    echo "  ✅ $p"
done
[ "${#PASSED_PKGS[@]}" -gt 10 ] && echo "  ...（共 ${#PASSED_PKGS[@]} 个）"

echo ""

if [ "${#FAILED_PKGS[@]}" -gt 0 ]; then
    echo "==> 未达标的包（< ${MIN}%): ${#FAILED_PKGS[@]}"
    for p in "${FAILED_PKGS[@]}"; do
        echo "  ❌ $p"
    done
    echo ""
    echo "❌ 覆盖率检查失败（${#FAILED_PKGS[@]} 个包未达 ${MIN}%）"
    exit 1
fi

echo "✅ 全部包覆盖率 >= ${MIN}%"
exit 0
