-- 20260608000001_init.sql
--
-- AgentID-Chain 初始 schema migration
-- 由 LRA P3.6 任务创建 — ent 0.14.6 不再自带 migrate generate 子命令，
-- 改用 Atlas / 手写 SQL；本文件对应 ent/schema 下的 4 个 schema（User/Agent/AuditLog/OutboxEvent）。
--
-- 命名约定：
--   表名：snake_case 复数（users / agents / audit_logs / outbox_events）
--   主键：id uuid NOT NULL DEFAULT gen_random_uuid()
--   时间戳：created_at / updated_at timestamptz NOT NULL DEFAULT now()
--   索引：idx_<table>_<columns>，唯一约束：<table>_<columns>_key
--
-- 注意事项：
--   - gen_random_uuid() 需要 pgcrypto 扩展（已在 scripts/init-databases.sql 中 CREATE EXTENSION）
--   - 未来若切到 MySQL / SQLite，pg_trgm / jsonb 需要重新评估
--   - 此 migration 与 ent migrate.Atlas 兼容：可后续用 atlas migrate apply 校验

BEGIN;

-- 启用扩展（幂等）
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ==================== users ====================
CREATE TABLE IF NOT EXISTS "users" (
    "id"          uuid        NOT NULL DEFAULT gen_random_uuid(),
    "email"       varchar(320) NOT NULL,
    "role_did"    varchar(256) NOT NULL,
    "created_at"  timestamptz NOT NULL DEFAULT now(),
    "updated_at"  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "users_email_key" ON "users" ("email");
CREATE INDEX IF NOT EXISTS "idx_users_role_did"  ON "users" ("role_did");

COMMENT ON TABLE  "users"                   IS 'AgentID-Chain 系统用户（Agent 拥有者 / 操作者）';
COMMENT ON COLUMN "users"."id"              IS 'UUIDv7 主键；时间排序、全局唯一';
COMMENT ON COLUMN "users"."email"           IS '用户邮箱；唯一约束';
COMMENT ON COLUMN "users"."role_did"        IS '角色 DID（did:agentid:role:admin / user 等）';
COMMENT ON COLUMN "users"."created_at"      IS '创建时间';
COMMENT ON COLUMN "users"."updated_at"      IS '更新时间';

-- ==================== agents ====================
CREATE TABLE IF NOT EXISTS "agents" (
    "id"          uuid        NOT NULL DEFAULT gen_random_uuid(),
    "owner_did"   varchar(256) NOT NULL,
    "level"       smallint    NOT NULL DEFAULT 0 CHECK ("level" >= 0 AND "level" <= 7),
    "permission"  bigint      NOT NULL DEFAULT 0,
    "status"      smallint    NOT NULL DEFAULT 0 CHECK ("status" >= 0 AND "status" <= 3),
    "created_at"  timestamptz NOT NULL DEFAULT now(),
    "updated_at"  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "agents_id_key"       ON "agents" ("id");
CREATE INDEX        IF NOT EXISTS "idx_agent_owner_did" ON "agents" ("owner_did");
CREATE INDEX        IF NOT EXISTS "idx_agent_status"    ON "agents" ("status");

COMMENT ON TABLE  "agents"              IS 'Agent 实体（核心）';
COMMENT ON COLUMN "agents"."id"         IS 'UUIDv7 主键；全局唯一（链上链下统一表示）';
COMMENT ON COLUMN "agents"."owner_did"  IS '拥有者 DID';
COMMENT ON COLUMN "agents"."level"      IS '等级 0-7';
COMMENT ON COLUMN "agents"."permission" IS '权限位掩码';
COMMENT ON COLUMN "agents"."status"     IS '状态机：0=registered, 1=active, 2=banned, 3=unregistered';

-- ==================== audit_logs ====================
CREATE TABLE IF NOT EXISTS "audit_logs" (
    "id"            uuid        NOT NULL DEFAULT gen_random_uuid(),
    "action"        varchar(64)  NOT NULL,
    "reason"        varchar(1024),
    "operator_did"  varchar(256) NOT NULL,
    "occurred_at"   timestamptz NOT NULL DEFAULT now(),
    "agent_agents"  uuid        NOT NULL,
    PRIMARY KEY ("id"),
    CONSTRAINT "audit_logs_agent_agents_fk"
        FOREIGN KEY ("agent_agents")
        REFERENCES "agents" ("id")
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "idx_audit_logs_agent"     ON "audit_logs" ("agent_agents");
CREATE INDEX IF NOT EXISTS "idx_audit_logs_occurred"  ON "audit_logs" ("occurred_at" DESC);

COMMENT ON TABLE  "audit_logs"                IS 'Agent 生命周期审计日志';
COMMENT ON COLUMN "audit_logs"."action"       IS '动作类型：register/upgrade/ban/unban/unregister';
COMMENT ON COLUMN "audit_logs"."reason"       IS '业务原因（可空）';
COMMENT ON COLUMN "audit_logs"."operator_did" IS '操作者 DID';
COMMENT ON COLUMN "audit_logs"."agent_agents" IS '归属 Agent ID（ent edge 反向字段命名）';

-- ==================== outbox_events ====================
CREATE TABLE IF NOT EXISTS "outbox_events" (
    "id"              uuid         NOT NULL DEFAULT gen_random_uuid(),
    "aggregate_type"  varchar(64)  NOT NULL,
    "aggregate_id"    varchar(64)  NOT NULL,
    "event_type"      varchar(128) NOT NULL,
    "payload"         jsonb        NOT NULL,
    "occurred_at"     timestamptz  NOT NULL DEFAULT now(),
    "idempotency_key" varchar(128) NOT NULL,
    "status"          smallint     NOT NULL DEFAULT 0 CHECK ("status" >= 0 AND "status" <= 3),
    "retry_count"     integer      NOT NULL DEFAULT 0,
    "last_error"      varchar(1024),
    "next_retry_at"   timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "outbox_events_idempotency_key_key" ON "outbox_events" ("idempotency_key");
CREATE INDEX        IF NOT EXISTS "idx_outbox_pending"               ON "outbox_events" ("status", "next_retry_at");
CREATE INDEX        IF NOT EXISTS "idx_outbox_aggregate"             ON "outbox_events" ("aggregate_type", "aggregate_id");

COMMENT ON TABLE  "outbox_events"                 IS '事务性发件箱表';
COMMENT ON COLUMN "outbox_events"."aggregate_type" IS '聚合根类型';
COMMENT ON COLUMN "outbox_events"."aggregate_id"   IS '聚合根 ID';
COMMENT ON COLUMN "outbox_events"."event_type"     IS '事件类型';
COMMENT ON COLUMN "outbox_events"."payload"        IS 'JSON payload';
COMMENT ON COLUMN "outbox_events"."idempotency_key" IS '幂等键（unique）';
COMMENT ON COLUMN "outbox_events"."status"         IS '0=pending, 1=published, 2=failed, 3=dead';
COMMENT ON COLUMN "outbox_events"."retry_count"    IS '已重试次数';
COMMENT ON COLUMN "outbox_events"."next_retry_at"  IS '下次可重试时间（轮询扫描用）';

COMMIT;
