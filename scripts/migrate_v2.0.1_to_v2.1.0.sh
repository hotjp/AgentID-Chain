#!/usr/bin/env bash
# scripts/migrate_v2.0.1_to_v2.1.0.sh
#
# AgentID-Chain 数据迁移脚本：v2.0.1 → v2.1.0
#
# 变更：
#   1. 添加 idempotency_key 列（NOT NULL）
#   2. 添加 chain.fallback_url 配置
#   3. 重命名 storage.chain_url → storage.chain.primary_url
#   4. 重建索引 idx_agent_metadata_gin
#
# 用法：
#   ./scripts/migrate_v2.0.1_to_v2.1.0.sh           # 实际执行
#   ./scripts/migrate_v2.0.1_to_v2.1.0.sh --dry-run  # 演练
#   ./scripts/migrate_v2.0.1_to_v2.1.0.sh --rollback # 回滚
#
# 前置条件：
#   - PostgreSQL 可达（$DATABASE_URL）
#   - Redis 可达（$REDIS_URL）
#   - 已备份数据库

set -euo pipefail

# ===== 配置 =====
DATABASE_URL="${DATABASE_URL:-postgres://devuser:devpass@localhost:5432/agentid_chain?sslmode=disable}"
REDIS_URL="${REDIS_URL:-redis://localhost:6379/0}"
BACKUP_DIR="${BACKUP_DIR:-./build/migration-backup}"
DRY_RUN=false
ROLLBACK=false

# ===== 参数解析 =====
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --rollback) ROLLBACK=true; shift ;;
    --database-url) DATABASE_URL="$2"; shift 2 ;;
    --redis-url) REDIS_URL="$2"; shift 2 ;;
    -h|--help)
      grep "^#" "$0" | head -25
      exit 0
      ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

# ===== 工具函数 =====
log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }
err() { echo "ERROR: $*" >&2; exit 1; }
run_sql() {
  local sql="$1"
  if $DRY_RUN; then
    log "[DRY-RUN] SQL: $sql"
  else
    log "SQL: $sql"
    psql "$DATABASE_URL" -c "$sql"
  fi
}

# ===== 前置检查 =====
check_prereqs() {
  command -v psql >/dev/null || err "psql not found"
  command -v redis-cli >/dev/null || err "redis-cli not found"

  log "Checking DB connectivity..."
  if ! $DRY_RUN; then
    psql "$DATABASE_URL" -c "SELECT 1" >/dev/null
  fi
  log "✓ DB OK"

  log "Checking Redis connectivity..."
  if ! $DRY_RUN; then
    redis-cli -u "$REDIS_URL" PING >/dev/null
  fi
  log "✓ Redis OK"
}

# ===== 备份 =====
backup() {
  log "Backing up database to $BACKUP_DIR"
  mkdir -p "$BACKUP_DIR"
  local ts
  ts=$(date -u +%Y%m%dT%H%M%SZ)
  if ! $DRY_RUN; then
    pg_dump "$DATABASE_URL" > "$BACKUP_DIR/agentid_chain-$ts.sql"
    log "✓ Backup saved: $BACKUP_DIR/agentid_chain-$ts.sql"
  fi
}

# ===== 升级迁移 =====
migrate_up() {
  log "==== v2.0.1 → v2.1.0 ===="

  # 1. 添加 idempotency_key 列
  log "Step 1: add idempotency_key column"
  run_sql "ALTER TABLE agents ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(64);"
  run_sql "UPDATE agents SET idempotency_key = encode(gen_random_bytes(32), 'hex') WHERE idempotency_key IS NULL;"
  run_sql "ALTER TABLE agents ALTER COLUMN idempotency_key SET NOT NULL;"
  run_sql "CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_idempotency ON agents(idempotency_key);"

  # 2. 添加新配置字段（如果有 config 表）
  log "Step 2: add config fields"
  run_sql "INSERT INTO config (key, value, updated_at) VALUES ('chain.fallback_url', '\"\"', NOW()) ON CONFLICT (key) DO NOTHING;"

  # 3. 重命名配置键
  log "Step 3: rename config keys"
  run_sql "UPDATE config SET key = 'storage.chain.primary_url' WHERE key = 'storage.chain_url';"

  # 4. 重建 GIN 索引
  log "Step 4: rebuild GIN index"
  run_sql "DROP INDEX IF EXISTS idx_agent_metadata_gin;"
  run_sql "CREATE INDEX idx_agent_metadata_gin ON agents USING GIN (metadata);"

  # 5. 更新 schema_version
  log "Step 5: update schema version"
  run_sql "UPDATE schema_version SET version = '2.1.0', migrated_at = NOW() WHERE id = 1;"

  log "✅ Migration completed"
}

# ===== 回滚 =====
migrate_down() {
  log "==== v2.1.0 → v2.0.1 (rollback) ===="

  # 反向操作
  log "Step 1: drop idempotency_key column"
  run_sql "DROP INDEX IF EXISTS idx_agent_idempotency;"
  run_sql "ALTER TABLE agents DROP COLUMN IF EXISTS idempotency_key;"

  log "Step 2: revert config keys"
  run_sql "UPDATE config SET key = 'storage.chain_url' WHERE key = 'storage.chain.primary_url';"
  run_sql "DELETE FROM config WHERE key = 'chain.fallback_url';"

  log "Step 3: revert schema version"
  run_sql "UPDATE schema_version SET version = '2.0.1' WHERE id = 1;"

  log "✅ Rollback completed"
}

# ===== 主流程 =====
main() {
  log "==== AgentID-Chain Migration ===="
  log "From: v2.0.1"
  log "To:   v2.1.0"
  log "Mode: $($DRY_RUN && echo 'DRY-RUN' || echo 'APPLY')"
  log "Action: $($ROLLBACK && echo 'ROLLBACK' || echo 'UPGRADE')"
  echo

  check_prereqs

  if $ROLLBACK; then
    migrate_down
  else
    backup
    migrate_up
  fi

  log "==== Done ===="
}

main "$@"
