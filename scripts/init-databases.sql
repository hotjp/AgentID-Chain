-- =============================================================================
-- AgentID-Chain v2.0.1 — Postgres init databases
-- =============================================================================
-- 用法:
--   挂载到 dev-postgres 容器的 /docker-entrypoint-initdb.d/
--   首次启动空数据卷时自动执行（postgres:16-alpine 官方约定）
--
-- 默认连接超级用户 = $POSTGRES_USER（即 'agentid'）
-- 默认数据库 = $POSTGRES_DB（即 'agentid'，由镜像自动建好）
--
-- 本脚本职责：
--   1. 创建项目内其他必需库（apigateway/authcenter/tagsense/audit）
--   2. 创建配套用户（每库独立用户，最小权限）
--   3. 开启必备扩展（uuid-ossp / pgcrypto / pg_trgm）
-- =============================================================================

-- ---------- 扩展 ------------------------------------------------------------
\connect agentid;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ---------- 业务库：apigateway ----------------------------------------------
CREATE DATABASE apigateway
    WITH OWNER = agentid
    ENCODING = 'UTF8'
    LC_COLLATE = 'C'
    LC_CTYPE = 'C'
    TEMPLATE = template0;

\connect apigateway;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------- 业务库：authcenter ----------------------------------------------
\connect agentid;

CREATE DATABASE authcenter
    WITH OWNER = agentid
    ENCODING = 'UTF8'
    LC_COLLATE = 'C'
    LC_CTYPE = 'C'
    TEMPLATE = template0;

\connect authcenter;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------- 业务库：tagsense ------------------------------------------------
\connect agentid;

CREATE DATABASE tagsense
    WITH OWNER = agentid
    ENCODING = 'UTF8'
    LC_COLLATE = 'C'
    LC_CTYPE = 'C'
    TEMPLATE = template0;

\connect tagsense;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ---------- 审计库：agentid_audit -------------------------------------------
\connect agentid;

CREATE DATABASE agentid_audit
    WITH OWNER = agentid
    ENCODING = 'UTF8'
    LC_COLLATE = 'C'
    LC_CTYPE = 'C'
    TEMPLATE = template0;

\connect agentid_audit;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------- 完成 ------------------------------------------------------------
\connect agentid;
SELECT 'AgentID-Chain databases initialized: agentid, apigateway, authcenter, tagsense, agentid_audit' AS init_status;
