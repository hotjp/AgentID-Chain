#!/usr/bin/env bash
# =============================================================================
# AgentID-Chain — k6 负载测试运行脚本 (P18.5-18.7)
# =============================================================================
# 工具：k6 (https://k6.io)
# 安装：
#   macOS:  brew install k6
#   Linux:  sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
#           echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
#           sudo apt-get update && sudo apt-get install k6
#   Docker: docker run --rm -i grafana/k6 run -
# =============================================================================

set -euo pipefail

cd "$(dirname "$0")/../.."

TESTS_DIR="tests/load"
RESULTS_DIR="$TESTS_DIR/results"
mkdir -p "$RESULTS_DIR"

BASE_URL=${BASE_URL:-http://localhost:8080}
API_KEY=${API_KEY:-dev-loadtest-key}
DURATION=${DURATION:-5m}
RPS=${RPS:-100}

# 颜色
if [ -t 1 ]; then
    GREEN="\033[0;32m"; YELLOW="\033[1;33m"; RED="\033[0;31m"; NC="\033[0m"
else
    GREEN=""; YELLOW=""; RED=""; NC=""
fi

log()  { echo -e "${GREEN}==>${NC} $*"; }
warn() { echo -e "${YELLOW}warn:${NC} $*" >&2; }
err()  { echo -e "${RED}error:${NC} $*" >&2; }

# ---------- 工具检查 ----------
ensure_k6() {
    if ! command -v k6 >/dev/null 2>&1; then
        log "安装 k6..."
        if [ "$(uname -s)" = "Darwin" ]; then
            brew install k6
        else
            warn "请手动安装 k6：https://k6.io/docs/getting-started/installation/"
            exit 1
        fi
    fi
    k6 version
}

# ---------- 健康检查 ----------
check_health() {
    log "检查服务健康：$BASE_URL/live"
    if ! curl -sf -m 5 "$BASE_URL/live" >/dev/null; then
        err "服务不可用：$BASE_URL/live"
        return 1
    fi
    log "  ✓ 服务正常"
}

# ---------- 子命令 ----------
run_register() {
    ensure_k6
    check_health
    local out="$RESULTS_DIR/register-$(date +%Y%m%d-%H%M%S).json"
    log "运行 Register 负载测试（$RPS RPS × $DURATION）..."
    BASE_URL="$BASE_URL" API_KEY="$API_KEY" k6 run \
        --out "json=$out" \
        --duration "$DURATION" \
        --vus 50 \
        "$TESTS_DIR/register.js" || warn "Register 负载测试未达标（exit $?）"
    log "结果：$out"
}

run_a2a_negotiate() {
    ensure_k6
    check_health
    local out="$RESULTS_DIR/a2a-negotiate-$(date +%Y%m%d-%H%M%S).json"
    log "运行 A2A negotiate 负载测试..."
    BASE_URL="$BASE_URL" API_KEY="$API_KEY" k6 run \
        --out "json=$out" \
        --duration "$DURATION" \
        --vus 30 \
        "$TESTS_DIR/a2a-negotiate.js" || warn "A2A 负载测试未达标（exit $?）"
    log "结果：$out"
}

run_cache() {
    ensure_k6
    check_health
    local out="$RESULTS_DIR/cache-$(date +%Y%m%d-%H%M%S).json"
    log "运行缓存命中负载测试..."
    BASE_URL="$BASE_URL" API_KEY="$API_KEY" k6 run \
        --out "json=$out" \
        --duration "$DURATION" \
        --vus 20 \
        "$TESTS_DIR/cache.js" || warn "Cache 负载测试未达标（exit $?）"
    log "结果：$out"
}

run_all() {
    run_register
    run_a2a_negotiate
    run_cache
}

show_help() {
    sed -n '2,20p' "$0" | sed 's/^# //;s/^#//'
}

# ---------- 入口 ----------
SUBCMD=${1:-help}
shift || true
case "$SUBCMD" in
    register)        run_register "$@" ;;
    a2a-negotiate)   run_a2a_negotiate "$@" ;;
    cache)           run_cache "$@" ;;
    all)             run_all "$@" ;;
    help|--help|-h)  show_help ;;
    *) err "未知子命令：$SUBCMD"; show_help; exit 1 ;;
esac
