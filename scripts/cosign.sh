#!/usr/bin/env bash
# =============================================================================
# AgentID-Chain — Cosign 签名与验签 (P17.6)
# =============================================================================
# 工具：cosign (https://github.com/sigstore/cosign)
# 用法：
#   ./scripts/cosign.sh keygen                      # 生成本地密钥对
#   ./scripts/cosign.sh sign-image <image>[:<tag>]  # 签名镜像（key-based）
#   ./scripts/cosign.sh sign-image-keyless <image>  # 签名镜像（OIDC keyless）
#   ./scripts/cosign.sh verify-image <image>[:<tag>]
#   ./scripts/cosign.sh sign-blob <file>            # 签名文件（key-based）
#   ./scripts/cosign.sh sign-blob-keyless <file>    # 签名文件（OIDC keyless）
#   ./scripts/cosign.sh verify-blob <file> <sig>    # 验签文件
#   ./scripts/cosign.sh sign-sbom                   # 签名所有 SBOM（keyless）
#   ./scripts/cosign.sh verify-all                  # 批量验签
#
# 环境变量：
#   COSIGN_KEY       私钥路径（默认 ./keys/cosign.key）
#   COSIGN_PUB       公钥路径（默认 ./keys/cosign.pub）
#   COSIGN_PASSWORD  私钥密码（key-based 模式）
#   COSIGN_REPOSITORY OIDC issuer（默认 sigstore）
# =============================================================================

set -euo pipefail

cd "$(dirname "$0")/.."

COSIGN_KEY=${COSIGN_KEY:-keys/cosign.key}
COSIGN_PUB=${COSIGN_PUB:-keys/cosign.pub}
DIST_DIR=${DIST_DIR:-dist}
SBOM_DIR=${SBOM_DIR:-dist/sbom}

# ---------- 颜色 ----------
if [ -t 1 ]; then
    GREEN="\033[0;32m"; YELLOW="\033[1;33m"; RED="\033[0;31m"; NC="\033[0m"
else
    GREEN=""; YELLOW=""; RED=""; NC=""
fi

log()   { echo -e "${GREEN}==>${NC} $*"; }
warn()  { echo -e "${YELLOW}warn:${NC} $*" >&2; }
err()   { echo -e "${RED}error:${NC} $*" >&2; }

# ---------- 工具检查 ----------
ensure_cosign() {
    if ! command -v cosign >/dev/null 2>&1; then
        log "安装 cosign..."
        if [ "$(uname -s)" = "Darwin" ]; then
            brew install cosign
        else
            curl -O -L "https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64"
            sudo mv cosign-linux-amd64 /usr/local/bin/cosign
            sudo chmod +x /usr/local/bin/cosign
        fi
    fi
    cosign version
}

# ---------- 子命令 ----------
cmd_keygen() {
    ensure_cosign
    if [ -f "$COSIGN_KEY" ] && [ -f "$COSIGN_PUB" ]; then
        warn "密钥对已存在：$COSIGN_KEY / $COSIGN_PUB"
        return 0
    fi
    mkdir -p "$(dirname "$COSIGN_KEY")"
    log "生成 cosign 密钥对到 $(dirname "$COSIGN_KEY")/"
    COSIGN_PASSWORD="${COSIGN_PASSWORD:-}" cosign generate-key-pair
    ls -la "$(dirname "$COSIGN_KEY")"
    warn "请将 cosign.pub 公钥提交到仓库（cosign.key 严禁提交）"
}

cmd_sign_image() {
    local image=${1:?usage: cosign.sh sign-image <image>[:<tag>]}
    ensure_cosign
    [ -f "$COSIGN_KEY" ] || { err "未找到 $COSIGN_KEY — 先执行 keygen"; exit 1; }
    log "签名镜像（key-based）：$image"
    COSIGN_PASSWORD="${COSIGN_PASSWORD:-}" cosign sign --key "$COSIGN_KEY" "$image"
}

cmd_sign_image_keyless() {
    local image=${1:?usage: cosign.sh sign-image-keyless <image>[:<tag>]}
    ensure_cosign
    log "签名镜像（keyless OIDC）：$image"
    log "  - 需要 OIDC 令牌（CI: actions:oidc-request-token）"
    log "  - 需要 COSIGN_EXPERIMENTAL=1（OIDC subject spec）"
    COSIGN_EXPERIMENTAL=1 cosign sign --yes "$image"
}

cmd_verify_image() {
    local image=${1:?usage: cosign.sh verify-image <image>[:<tag>]}
    ensure_cosign
    [ -f "$COSIGN_PUB" ] || { err "未找到 $COSIGN_PUB"; exit 1; }
    log "验证镜像签名：$image"
    cosign verify --key "$COSIGN_PUB" "$image"
}

cmd_verify_image_keyless() {
    local image=${1:?usage: cosign.sh verify-image-keyless <image>[:<tag>]}
    ensure_cosign
    log "验证镜像签名（keyless via cert-chain）：$image"
    cosign verify \
        --certificate-identity-regexp 'https://github.com/.*' \
        --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
        "$image"
}

cmd_sign_blob() {
    local file=${1:?usage: cosign.sh sign-blob <file>}
    ensure_cosign
    [ -f "$COSIGN_KEY" ] || { err "未找到 $COSIGN_KEY — 先执行 keygen"; exit 1; }
    local sig="${file}.sig"
    local cert="${file}.cert"
    log "签名文件（key-based）：$file"
    COSIGN_PASSWORD="${COSIGN_PASSWORD:-}" cosign sign-blob \
        --key "$COSIGN_KEY" \
        --signature "$sig" \
        --certificate "$cert" \
        "$file"
    log "  ✓ $sig"
    log "  ✓ $cert"
}

cmd_sign_blob_keyless() {
    local file=${1:?usage: cosign.sh sign-blob-keyless <file>}
    ensure_cosign
    local sig="${file}.sig"
    local cert="${file}.cert"
    log "签名文件（keyless OIDC）：$file"
    COSIGN_EXPERIMENTAL=1 cosign sign-blob --yes \
        --output-signature "$sig" \
        --output-certificate "$cert" \
        "$file"
    log "  ✓ $sig"
    log "  ✓ $cert"
}

cmd_verify_blob() {
    local file=${1:?usage: cosign.sh verify-blob <file> <signature>}
    local sig=${2:?usage: cosign.sh verify-blob <file> <signature>}
    ensure_cosign
    [ -f "$COSIGN_PUB" ] || { err "未找到 $COSIGN_PUB"; exit 1; }
    log "验证文件签名：$file"
    cosign verify-blob --key "$COSIGN_PUB" --signature "$sig" "$file"
}

cmd_verify_blob_keyless() {
    local file=${1:?usage: cosign.sh verify-blob-keyless <file> <signature> <cert>}
    local sig=${2:?usage: cosign.sh verify-blob-keyless <file> <signature> <cert>}
    local cert=${3:?usage: cosign.sh verify-blob-keyless <file> <signature> <cert>}
    ensure_cosign
    log "验证文件签名（keyless via cert-chain）：$file"
    cosign verify-blob \
        --certificate "$cert" \
        --signature "$sig" \
        --certificate-identity-regexp 'https://github.com/.*' \
        --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
        "$file"
}

cmd_sign_sbom() {
    ensure_cosign
    [ -d "$SBOM_DIR" ] || { err "SBOM 目录不存在：$SBOM_DIR（先运行 scripts/sbom.sh）"; exit 1; }
    log "签名所有 SBOM 文件（keyless OIDC）..."
    local count=0
    for sbom in "$SBOM_DIR"/*.json; do
        [ -f "$sbom" ] || continue
        log "  - $(basename "$sbom")"
        COSIGN_EXPERIMENTAL=1 cosign sign-blob --yes \
            --output-signature "${sbom}.sig" \
            --output-certificate "${sbom}.cert" \
            "$sbom"
        count=$((count + 1))
    done
    log "✅ 已签名 $count 个 SBOM 文件"
}

cmd_verify_all() {
    ensure_cosign
    log "=== 批量验签 ==="
    local failed=0
    # SBOM
    if [ -d "$SBOM_DIR" ]; then
        for sig in "$SBOM_DIR"/*.sig; do
            [ -f "$sig" ] || continue
            local file="${sig%.sig}"
            local cert="${sig%.sig}.cert"
            if [ -f "$cert" ]; then
                log "  - $(basename "$file") (keyless)"
                if ! cosign verify-blob \
                    --certificate "$cert" \
                    --signature "$sig" \
                    --certificate-identity-regexp 'https://github.com/.*' \
                    --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
                    "$file" >/dev/null 2>&1; then
                    err "验签失败：$file"
                    failed=$((failed + 1))
                fi
            elif [ -f "$COSIGN_PUB" ]; then
                log "  - $(basename "$file") (key-based)"
                if ! cosign verify-blob --key "$COSIGN_PUB" --signature "$sig" "$file" >/dev/null 2>&1; then
                    err "验签失败：$file"
                    failed=$((failed + 1))
                fi
            fi
        done
    fi
    if [ "$failed" -eq 0 ]; then
        log "✅ 全部验签通过"
    else
        err "❌ $failed 个文件验签失败"
        exit 1
    fi
}

cmd_help() {
    sed -n '2,40p' "$0" | sed 's/^# //;s/^#//'
}

# ---------- 入口 ----------
SUBCMD=${1:-help}
shift || true
case "$SUBCMD" in
    keygen)              cmd_keygen "$@" ;;
    sign-image)          cmd_sign_image "$@" ;;
    sign-image-keyless)  cmd_sign_image_keyless "$@" ;;
    verify-image)        cmd_verify_image "$@" ;;
    verify-image-keyless) cmd_verify_image_keyless "$@" ;;
    sign-blob)           cmd_sign_blob "$@" ;;
    sign-blob-keyless)   cmd_sign_blob_keyless "$@" ;;
    verify-blob)         cmd_verify_blob "$@" ;;
    verify-blob-keyless) cmd_verify_blob_keyless "$@" ;;
    sign-sbom)           cmd_sign_sbom "$@" ;;
    verify-all)          cmd_verify_all "$@" ;;
    help|--help|-h)      cmd_help ;;
    *) err "未知子命令：$SUBCMD"; cmd_help; exit 1 ;;
esac
