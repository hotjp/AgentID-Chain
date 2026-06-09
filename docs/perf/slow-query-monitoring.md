# 慢查询监控 (P18.12)

> 目标：发现并告警所有 > 200ms 的 SQL 查询
> 实现：`internal/storage/slow_query.go`

## 1. 设计

### 1.1 拦截点

| 层 | 拦截方式 | 适用 |
|----|---------|------|
| **ent ORM** | `ent.Client` interceptor / driver wrapper | 主要路径 |
| **pgx 直连** | `pgx.Tracer` 接口 | 性能敏感场景 |
| **database/sql** | 自定义 `driver.Conn` | 兜底 |

本项目使用 ent ORM，因此采用 `ent.Driver` 包装方式。

### 1.2 核心结构

```go
type SlowQueryMonitor struct {
    cfg       SlowQueryConfig
    threshold time.Duration       // 默认 200ms
    histogram *DurationHistogram  // 全量分位数
    counter   atomic.Int64        // 慢查询计数
}

type SlowQueryConfig struct {
    Threshold  time.Duration
    Logger     *slog.Logger
    SampleRate float64
    OnSlowQuery func(SlowQueryInfo)  // 自定义回调
    Enabled    bool
}
```

## 2. 集成步骤

### 2.1 初始化（main.go）

```go
slowMonitor := storage.NewSlowQueryMonitor(storage.SlowQueryConfig{
    Threshold:  200 * time.Millisecond,
    Logger:     logger,
    OnSlowQuery: func(info storage.SlowQueryInfo) {
        metrics.SlowQueriesTotal.WithLabelValues(template).Inc()
        alerting.Send("slow_query", info)  // 可选
    },
})
```

### 2.2 ent Driver 包装

```go
// 在 L4 service 层包装
client := ent.NewClient(ent.Driver(
    storage.NewSlowQueryDriver(db.Driver(), slowMonitor),
))
```

## 3. 监控指标

### 3.1 Prometheus 指标（建议暴露）

```promql
# 慢查询计数（按 query 模板）
agentid_slow_queries_total{template="SELECT * FROM users WHERE id = $1"} 42

# 全量查询 P99
agentid_query_duration_seconds_p99 0.234
```

### 3.2 告警规则

```yaml
- alert: HighSlowQueryRate
  expr: rate(agentid_slow_queries_total[5m]) > 10
  for: 2m
  annotations:
    summary: "5 分钟内慢查询 > 10 次/秒"

- alert: SlowQuerySpike
  expr: agentid_query_duration_seconds_p99 > 1.0
  for: 1m
  annotations:
    summary: "P99 延迟 > 1 秒"
```

## 4. SQL 模板归一化

为避免高基数（high cardinality），需要将 SQL 模板归一化：

```sql
-- 原始
SELECT * FROM users WHERE id = 42 AND email = 'alice@example.com'

-- 归一化后
SELECT * FROM users WHERE id = $1 AND email = $2
```

归一化方法：
1. 正则替换字符串字面量为 `$N`
2. 替换数字为 `$N`
3. 保留关键字

## 5. 优化方向（基于慢查询）

| 现象 | 可能原因 | 优化 |
|------|---------|------|
| 单条 SQL > 1s | 全表扫描 | 加索引 |
| 频繁出现 | N+1 查询 | 批量加载 |
| 偶发 5-10s | 锁等待 | 优化事务 |
| 突发 spike | 缓存失效 | 预热 |

## 6. 与 APM 集成

```go
OnSlowQuery: func(info SlowQueryInfo) {
    apm.Transaction().
        SetTag("db.statement", info.SQL).
        SetTag("db.duration", info.Duration).
        SetCustom("slow_query", true).
        Notice()
}
```

## 7. 引用

- pgx Tracer 接口：`internal/storage/postgres.go`
- ent Driver 包装：`docs/orm-patterns.md`
- SQL 模板归一化：https://github.com/jaegertracing/jaeger/blob/master/plugin/storage/cassandra/schema/query_template.go
