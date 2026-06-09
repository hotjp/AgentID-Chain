#!/usr/bin/env bash
# =============================================================================
# AgentID-Chain — gomock 生成脚本 (P15.4)
# =============================================================================
# 用 mockgen 为所有 interface 生成 mock。
# 入口：internal/testutil/gomock/gen.sh
# 约定：mock 输出到 internal/testutil/gomock/ 下的同名子包。
#
# 用法：
#   ./internal/testutil/gomock/gen.sh                     # 生成全部
#   ./internal/testutil/gomock/gen.sh internal/storage     # 只生成一个包
# =============================================================================

set -euo pipefail

cd "$(dirname "$0")/../../.."

MOCK_ROOT="internal/testutil/gomock"

# 包列表（可由参数覆盖）
if [ "$#" -gt 0 ]; then
    PACKAGES=("$@")
else
    PACKAGES=(
        "internal/storage"
        "internal/authz"
        "internal/aap"
        "internal/a2a"
    )
fi

# 检查 mockgen
if ! command -v mockgen >/dev/null 2>&1; then
    echo "==> mockgen 未安装，正在安装..."
    go install go.uber.org/mock/mockgen@latest
fi

for pkg in "${PACKAGES[@]}"; do
    echo "==> 处理 $pkg"
    # 找出所有 interface
    interfaces=$(go doc -all "./$pkg" 2>/dev/null | grep -E "^type [A-Z][A-Za-z0-9_]+ interface" | awk '{print $2}' || true)
    if [ -z "$interfaces" ]; then
        echo "    (无 interface，跳过)"
        continue
    fi

    # 输出目录：保持与原始包相同的相对路径
    rel="${pkg#internal/}"
    out_dir="$MOCK_ROOT/$rel"
    mkdir -p "$out_dir"

    for iface in $interfaces; do
        out_file="$out_dir/${iface,,}_mock.go"
        echo "    生成 Mock: $iface → $out_file"
        mockgen -source="$pkg" -destination="$out_file" -package="mock_${rel//\//_}" "$iface" 2>/dev/null \
            || mockgen -destination="$out_file" -package="mock_${rel//\//_}" "$pkg" "$iface"
    done
done

echo "==> 完成 ✅"
