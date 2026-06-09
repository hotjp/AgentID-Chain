# Runbook: 数据库连接池耗尽

## 严重度
**P0** — 10min 内 ack

## 触发告警
- `ConnectionPoolExhausted` — 连接池使用率 > 90% (5min)
- 应用层出现 "connection refused" 或 "timeout acquiring connection"

## 症状
- 请求失败（"context deadline exceeded"）
- 慢查询激增
- 服务响应时间飙升

## 立即行动（5min 内）

1. **确认连接池状态**
   ```bash
   # 指标
   curl -s http://localhost:9090/metrics | grep "^backend_pool_size"

   # PG 端
   psql "$POSTGRES_DSN" -c "SELECT count(*), state FROM pg_stat_activity GROUP BY state"
   ```

2. **看活跃查询**
   ```bash
   psql "$POSTGRES_DSN" -c "SELECT pid, query, state, now() - query_start as duration FROM pg_stat_activity WHERE state != 'idle' ORDER BY duration DESC LIMIT 20"
   ```

3. **必要时重启服务**（释放所有连接）
   ```bash
   kubectl rollout restart deployment/api-gateway
   ```

## 诊断

### 1. 哪些查询占着连接？

```sql
SELECT
  pid,
  usename,
  application_name,
  client_addr,
  state,
  wait_event_type,
  wait_event,
  now() - query_start AS duration,
  LEFT(query, 200) AS query_preview
FROM pg_stat_activity
WHERE state != 'idle' AND pid != pg_backend_pid()
ORDER BY duration DESC
LIMIT 30;
```

### 2. 是否慢查询？

```sql
-- pg_stat_statements 需提前开启
SELECT
  substring(query, 1, 80) AS query,
  calls,
  round(mean_exec_time::numeric, 2) AS mean_ms,
  round((100 * total_exec_time / sum(total_exec_time) OVER ())::numeric, 2) AS pct_total
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 20;
```

### 3. 是否有锁等待？

```sql
SELECT
  blocked_locks.pid AS blocked_pid,
  blocked_activity.usename AS blocked_user,
  blocking_locks.pid AS blocking_pid,
  blocking_activity.usename AS blocking_user,
  blocked_activity.query AS blocked_query,
  blocking_activity.query AS blocking_query
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity
  ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks
  ON blocking_locks.locktype = blocked_locks.locktype
  AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
  AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
  AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
  AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
  AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
  AND blocking_locks.pid != blocked_locks.pid
  AND blocking_locks.granted
JOIN pg_catalog.pg_stat_activity blocking_activity
  ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;
```

## 缓解（短期）

1. **杀掉长查询**
   ```sql
   -- 杀掉执行超过 60s 的非系统查询
   SELECT pg_terminate_backend(pid)
   FROM pg_stat_activity
   WHERE state = 'active'
     AND now() - query_start > interval '60 seconds'
     AND query NOT LIKE '%pg_stat_activity%';
   ```

2. **临时增加连接池上限**（修改配置 + 重启）

3. **限流上游**（减少新请求）

## 修复（根本）

### 增加连接池容量

```yaml
storage:
  local:
    max_open: 50   # 原 25
    max_idle: 20   # 原 10
```

⚠️ 注意：每个连接占用 PG 后端内存（默认 ~10MB）。需评估 PG `max_connections`。

### 优化慢查询

1. 添加索引：
   ```sql
   CREATE INDEX CONCURRENTLY idx_agents_owner_status
     ON agents (owner, status) WHERE status = 'active';
   ```

2. 重写查询（避免 N+1、避免 `SELECT *`）

3. 分页（避免 OFFSET 100000）

### 引入连接池中间件

考虑 [PgBouncer](https://www.pgbouncer.org/) 在应用和 PG 之间做连接池：
- transaction 模式：每个事务获取连接，事务结束立即释放
- 可支撑 10000+ 应用客户端，PG 端只需 100 个连接

## 验证

- [ ] `backend_pool_size{state="active"} < 80%`
- [ ] 慢查询比例 < 5%
- [ ] P99 延迟恢复到基线
- [ ] 告警已恢复

## 配置调优指南

| 指标 | 推荐值 |
|------|--------|
| `max_open` (应用) | 25-50 (per instance) |
| `max_idle` (应用) | 10 (≈ max_open / 2.5) |
| `max_lifetime` | 5m |
| `max_idle_time` | 10m |
| PG `max_connections` | 200 (per instance) |

## 📚 相关

- [perf/connection-pool-tuning.md](../perf/connection-pool-tuning.md)
- [perf/slow-query-monitoring.md](../perf/slow-query-monitoring.md)
- [operations/troubleshooting.md](../operations/troubleshooting.md)
