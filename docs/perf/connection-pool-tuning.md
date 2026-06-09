# 连接池参数调优 (P18.11)

> 范围：PostgreSQL + Redis 连接池
> 依据：P18.1-18.6 压测数据 + PostgreSQL/Redis 最佳实践

## 1. 调参原则

### 1.1 PostgreSQL (pgxpool)

| 参数 | 当前 | 含义 | 调参依据 |
|------|------|------|---------|
| `max_open` | 25 | 最大连接数 | 服务节点数 × 25 ≤ DB `max_connections` |
| `max_idle` | 10 | 最大空闲连接 | 减少握手开销 |
| `max_lifetime` | 5m | 连接最大寿命 | 防 stale / DNS 切换失效 |
| `max_idle_time` | 2m | 空闲连接最大存活 | 释放资源 |
| `conn_timeout` | 5s | 建立连接超时 | 防雪崩 |
| `query_timeout` | 10s | 单次查询超时 | 防慢查询拖累 |

**黄金公式**：

```
max_open × 服务副本数 ≤ PG.max_connections - 预留连接(管理员、监控、迁移)
```

示例：PG `max_connections=200`，3 副本：

```
服务 max_open × 3 + 50 (预留) ≤ 200
max_open ≤ 50
```

保守设 25（10x 余量给其他服务）。

### 1.2 Redis (go-redis)

| 参数 | 当前 | 含义 |
|------|------|------|
| `pool_size` | 20 | 总连接数 |
| `min_idle_conns` | 10 | 预热空闲连接 |
| `max_idle_conns` | 15 | 空闲连接上限 |
| `conn_max_idle_time` | 5m | 空闲连接寿命 |
| `pool_timeout` | 3s | 等待连接超时 |
| `read_timeout` | 2s | 读超时 |
| `write_timeout` | 2s | 写超时 |

**原则**：

- `pool_size` ≥ **并发 RPS × P99 延迟秒数**（Little's Law）
- 例：1000 RPS × 5ms = 5 个连接（最坏情况下）
- 实际：考虑突发 + 安全余量，设为 20

## 2. 监控指标

### 2.1 PG（pgxpool）

```promql
# 连接池使用率
pgxpool_acquired_conns / pgxpool_max_conns

# 等待中的连接数（> 0 表示瓶颈）
pgxpool_wait_count > 0

# 取消的获取请求
rate(pgxpool_canceled_acquire_count[5m]) > 0
```

### 2.2 Redis

```promql
# 连接池使用率
redis_pool_active_conns / redis_pool_max_conns

# 等待时间
histogram_quantile(0.99, redis_pool_wait_duration_seconds) > 0.05
```

## 3. 压测调参流程

### 3.1 准备

```bash
# 启动 1 个节点
docker-compose -f docker-compose.dev.yml up -d

# 启动 100 RPS 负载（5 分钟）
k6 run --duration 5m --vus 50 tests/load/register.js
```

### 3.2 观察指标

```bash
# PG 端
watch -n 1 "psql -c 'SELECT count(*), state FROM pg_stat_activity GROUP BY state;'"

# Redis 端
redis-cli INFO clients
redis-cli INFO stats | grep instantaneous
```

### 3.3 调整方向

| 现象 | 调整 |
|------|------|
| PG 等待连接 | ↑ `max_open`（前提是 DB 端有余量）|
| PG CPU 100% | ↓ `max_open`、加副本 |
| Redis 等待 | ↑ `pool_size` |
| Redis 内存 | ↓ `pool_size`、检查 `maxmemory` |
| 连接频繁创建 | ↑ `min_idle_conns` |

### 3.4 验证

```bash
# 压测时实时观察
watch -n 1 'curl -s http://localhost:9090/metrics | grep -E "pool|conn" | head -20'
```

## 4. 配置项

参见 `configs/app.yaml` 的 `storage.db` 和 `storage.redis` 段。

## 5. 容量规划表

| 服务副本 | DB max_connections | 单副本 max_open | 单副本 max_idle |
|---------|-------------------|---------------|---------------|
| 1 | 200 | 50 | 25 |
| 3 | 200 | 50 | 25 |
| 5 | 500 | 80 | 30 |
| 10 | 1000 | 80 | 30 |

## 6. 引用

- pgx 文档：https://github.com/jackc/pgx
- go-redis 文档：https://redis.uptrace.dev/
- Little's Law：https://en.wikipedia.org/wiki/Little%27s_law
