#!/usr/bin/env bash
# =============================================================================
# AgentID-Chain — SBOM 生成 (P17.5)
# =============================================================================
# 工具：syft (https://github.com/anchore/syft)
# 输出：CycloneDX JSON（推荐）+ SPDX JSON
# 用法：
#   ./scripts/sbom.sh                 # 全部
#   ./scripts/sbom.sh gateway         # 单个 binary
#   FORMAT=spdx ./scripts/sbom.sh     # SPDX 格式
# =============================================================================

set -euo pipefail

cd "$(dirname "$0")/.."

FORMAT=${FORMAT:-cyclonedx-json}
OUTPUT_DIR=${OUTPUT_DIR:-dist/sbom}
BIN_DIR=${BIN_DIR:-bin}

mkdir -p "$OUTPUT_DIR"

BINARIES=("agentid" "migration-tool" "mock-chain")
if [ "$#" -gt 0 ]; then
    BINARIES=("$@")
fi

# 检查 syft
if ! command -v syft >/dev/null 2>&1; then
    echo "==> syft 未安装，正在安装..."
    curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
fi

for bin in "${BINARIES[@]}"; do
    BIN_PATH="$BIN_DIR/$bin"
    if [ ! -f "$BIN_PATH" ]; then
        echo "==> 跳过（不存在）：$BIN_PATH"
        continue
    fi

    OUT="$OUTPUT_DIR/${bin}.cdx.json"
    echo "==> 生成 SBOM: $bin → $OUT"
    syft "file:$BIN_PATH" -o "$FORMAT" > "$OUT"

    SIZE=$(du -h "$OUT" | cut -f1)
    echo "    大小: $SIZE"
done

# 同时生成整体 SBOM（从 go.mod）
echo "==> 生成 module-level SBOM"
syft "dir:." -o "$FORMAT" > "$OUTPUT_DIR/agentid-chain.cdx.json" || true

# 创建 index
cat > "$OUTPUT_DIR/INDEX.md" <<EOF
# AgentID-Chain SBOM Index

生成时间: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
工具: syft $(syft version 2>/dev/null | head -1)
格式: $FORMAT

| Binary | SBOM | Size |
|--------|------|------|
EOF
for bin in "${BINARIES[@]}"; do
    if [ -f "$OUTPUT_DIR/${bin}.cdx.json" ]; then
        SIZE=$(du -h "$OUTPUT_DIR/${bin}.cdx.json" | cut -f1)
        echo "| \`$bin\` | [\`${bin}.cdx.json\`](${bin}.cdx.json) | $SIZE |" >> "$OUTPUT_DIR/INDEX.md"
    fi
done

echo ""
echo "✅ SBOM 生成完成：$OUTPUT_DIR/"
ls -la "$OUTPUT_DIR/"
