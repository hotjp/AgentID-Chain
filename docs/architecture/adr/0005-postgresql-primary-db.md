# ADR-0005: PostgreSQL 16 作为主数据库

## 状态

✅ Accepted（2024-03-09）

## 上下文

AgentID-Chain 需要一个主关系数据库，要求：

- **ACID 事务**：状态机变更必须强一致
- **JSONB 支持**：metadata 字段灵活存储
- **全文搜索**：可选，用于 owner/description 搜索
- **水平扩展**：未来可能分片
- **运维成熟度**：广泛使用，工具链完善
- **开源**：避免商业锁定

候选方案：

1. **PostgreSQL 16** — 开源、ACID、JSONB、成熟
2. **MySQL 8** — 流行、ACID
3. **SQLite** — 嵌入式、零运维
4. **CockroachDB** — 分布式 SQL、PG 协议兼容

## 决策

我们采用 **PostgreSQL 16** 作为主数据库（ent ORM 底层驱动使用 `pgx/v5`）。

理由：

- ✅ **ACID 强一致**：状态机事务必需
- ✅ **JSONB**：metadata 字段可索引、可查询
- ✅ **部分索引 / GIN**：优化冷热数据
- ✅ **丰富类型**：ULID、UUIDv7、enum、range
- ✅ **工具链成熟**：pg_dump / pg_restore / pgBouncer / 各种 exporter
- ✅ **云生态**：RDS / Aurora / Cloud SQL / 自建
- ✅ **开源无锁定**：BSD/MIT 风格许可

## 后果

### 正面

- ✅ 强一致事务，状态机不变量得到保证
- ✅ JSONB metadata 字段灵活，可演进
- ✅ 性能：P99 < 50ms（本地）；P99 < 100ms（云 RDS）
- ✅ 备份恢复工具链完善

### 负面

- ❌ 单机写入扩展性有限（水平扩展需分片或换 CockroachDB）
- ❌ 运维较 MySQL 略复杂
- ❌ 部分高级特性需 DBA 知识

### 中性

- 🔄 团队需熟悉 PG 生态（psql / pg_stat / EXPLAIN）
- 🔄 版本升级需谨慎（major 版本间有 breaking change）

## 替代方案

| 方案 | 优点 | 缺点 | 否决理由 |
|------|------|------|---------|
| **PostgreSQL 16**（已选） | ACID + JSONB + 工具链 | 运维成本 | — |
| MySQL 8 | 流行 | JSON 弱、事务隔离较弱 | JSONB 性能不足 |
| SQLite | 零运维 | 无并发写入、单机 | 与"分布式"愿景冲突 |
| CockroachDB | 分布式、PG 兼容 | 运维复杂、成本高 | 当前规模过度设计 |

## 实施细节

### 配置

```yaml
# config.yaml
storage:
  database:
    dsn: "postgres://user:pass@host:5432/agentid_chain?sslmode=require"
    max_open: 25
    max_idle: 10
    max_lifetime: 5m
    ssl_mode: require
```

### 索引策略

```sql
-- 主键 + 唯一
CREATE UNIQUE INDEX idx_agent_id ON agents(id);

-- owner 查询
CREATE INDEX idx_agent_owner ON agents(owner);

-- owner + status 联合
CREATE INDEX idx_agent_owner_status ON agents(owner, status);

-- JSONB 索引
CREATE INDEX idx_agent_metadata_gin ON agents USING GIN (metadata);

-- 部分索引：仅 active
CREATE INDEX idx_agent_active ON agents(owner) WHERE status = 'active';
```

### 备份

- 全量：每日 03:00 UTC `pg_dump`
- 增量：WAL archiving → S3
- 保留：30 天

### 监控

- `pg_stat_activity`：活跃连接
- `pg_stat_user_tables`：表扫描/索引命中
- `pg_locks`：锁等待
- 自定义 exporter：`internal/storage/metrics.go`

### 故障切换

- 主从：`streaming replication` + `repmgr`
- 自动 failover：Patroni + etcd（生产推荐）
- RPO ≤ 5s，RTO ≤ 60s

## 容量规划

| 指标 | 初始 | 1 年 | 3 年 |
|------|------|------|------|
| 数据量 | 10 GB | 500 GB | 5 TB |
| QPS | 100 | 5,000 | 50,000 |
| 连接数 | 25 | 100 | 500 |
| 备份保留 | 7d | 30d | 90d |

## 参考

- [PostgreSQL 16 官方文档](https://www.postgresql.org/docs/16/)
- [pgx/v5 驱动](https://github.com/jackc/pgx)
- 项目内 ADR-0001（存储后端）
- 项目内 ADR-0004（ent ORM）
- 项目内 `docs/architecture/storage.md`
- 相关 RFC: #L1-DB-1
