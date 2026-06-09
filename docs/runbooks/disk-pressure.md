# Runbook: 磁盘压力

## 严重度
**P0** — 15min 内 ack

## 触发告警
- 磁盘使用 > 85% (10min)
- 磁盘使用 > 95% (5min) — **critical**

## 症状
- 写操作失败
- 监控指标丢失（Prometheus 自身存储满）
- PG 报错 "No space left on device"
- Redis 触发 eviction 策略

## 立即行动（5min 内）

1. **确认磁盘使用**
   ```bash
   df -h | grep -v tmpfs
   du -sh /var/lib/docker/ 2>/dev/null
   du -sh /var/lib/postgresql/ 2>/dev/null
   ```

2. **找大文件**
   ```bash
   # 系统级
   du -h / | sort -h | tail -20

   # 查找大日志
   find /var/log -type f -size +100M 2>/dev/null
   ```

3. **紧急清理（如安全）**
   ```bash
   # 清理 Docker
   docker system prune -a --volumes  # 慎用！会删未持久化数据

   # 清理旧日志
   journalctl --vacuum-size=500M
   ```

## 诊断

### 1. 哪类数据在增长？

| 来源 | 排查命令 |
|------|---------|
| **PG 数据** | `psql -c "SELECT pg_size_pretty(pg_database_size('agentid'))"` |
| **PG WAL** | `ls -la /var/lib/postgresql/data/pg_wal/` |
| **Redis** | `redis-cli INFO keyspace` + `INFO memory` |
| **链上 worker 日志** | `du -sh /var/log/agentid/` |
| **审计日志** | `psql -c "SELECT count(*), pg_size_pretty(pg_total_relation_size('audit_logs')) FROM audit_logs"` |
| **outbox 残留** | `psql -c "SELECT chain_status, count(*), pg_size_pretty(sum(pg_column_size(payload))::bigint) FROM outbox_events GROUP BY chain_status"` |

### 2. PG 大表

```sql
SELECT
  schemaname || '.' || tablename AS table,
  pg_size_pretty(pg_total_relation_size(schemaname || '.' || tablename)) AS size,
  pg_total_relation_size(schemaname || '.' || tablename) AS bytes
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY bytes DESC
LIMIT 20;
```

### 3. PG 大索引

```sql
SELECT
  schemaname || '.' || indexname AS index,
  pg_size_pretty(pg_relation_size(schemaname || '.' || indexname)) AS size
FROM pg_indexes
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY pg_relation_size(schemaname || '.' || indexname) DESC
LIMIT 20;
```

## 缓解（短期）

### 1. 清理 outbox 已完成事件

```sql
-- 安全的：删除已确认 30 天以上的事件
DELETE FROM outbox_events
WHERE chain_status = 'confirmed'
  AND confirmed_at < now() - interval '30 days';
```

### 2. 清理审计日志

```sql
-- 保留 90 天
DELETE FROM audit_logs
WHERE created_at < now() - interval '90 days';
```

### 3. VACUUM FULL（释放碎片）

```sql
-- 需要停机或表锁
VACUUM FULL agents;
```

或在线：`pg_repack -t agents -d agentid`

### 4. 扩容磁盘

**云上**：
```bash
# AWS EBS
aws ec2 modify-volume --volume-id vol-xxx --size 200

# 然后在 OS 内扩展
sudo growpart /dev/nvme0n1 1
sudo resize2fs /dev/nvme0n1p1
```

**物理机**：
- 加盘 / LVM 扩展
- 迁移到更大卷

## 修复（根本）

### 1. 完善数据保留策略

```yaml
retention:
  audit_logs: 90d
  outbox_events: 30d
  outbox_failed: 7d  # 失败事件保留供分析
  metrics_local: 7d  # Prometheus 本地存储
```

### 2. 监控和告警

```promql
# 磁盘使用告警
(node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes) < 0.15
```

### 3. 日志轮转

`/etc/logrotate.d/agentid`:
```
/var/log/agentid/*.log {
  daily
  rotate 7
  compress
  missingok
  notifempty
  create 0644 agentid agentid
  postrotate
    systemctl reload agentid
  endscript
}
```

### 4. 容量规划

| 增长率 | 提前告警（85%） | 扩容动作 |
|--------|---------------|---------|
| 1GB/day | 30 天 | 加 30GB |
| 10GB/day | 15 天 | 加 150GB + 考虑分表/分区 |

## 验证

- [ ] 磁盘使用 < 70%
- [ ] 应用恢复正常（无写入失败）
- [ ] 告警已恢复
- [ ] 数据保留策略已配置

## 预防

1. **每月容量评审**：预测下月增长
2. **告警阈值**：85% 警告，95% critical
3. **定期 VACUUM**：自动 vacuum 已开启，但 `VACUUM FULL` 需手动
4. **归档**：超过保留期的数据归档到 S3/OSS

## 📚 相关

- [operations/troubleshooting.md](../operations/troubleshooting.md)
- [perf/slow-query-monitoring.md](../perf/slow-query-monitoring.md)
