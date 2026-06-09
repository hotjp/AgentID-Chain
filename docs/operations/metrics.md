# 指标与监控

> Prometheus 指标、可视化仪表板、告警规则

## 📊 指标分类

| 分类 | 指标前缀 | 用途 |
|------|---------|------|
| HTTP | `http_*` | 网关 / 业务路由 |
| AAP | `aap_*` | 准入协议 |
| A2A | `a2a_*` | Agent 间通信 |
| 后端 | `backend_*` | PG / Redis / Chain |
| 缓存 | `cache_*` | 命中率、延迟 |
| 资源 | `go_*` / `process_*` | 运行时 |

## 🔧 指标端点

| 服务 | 路径 |
|------|------|
| API Gateway | `http://localhost:9090/metrics` |
| Auth Center | `http://localhost:9091/metrics` |
| Tag Sense | `http://localhost:9092/metrics` |

## 📋 关键指标

### HTTP

```
http_requests_total{service_name, route, method, status_code}  # Counter
http_request_duration_seconds_bucket{service_name, route, method, le}  # Histogram
http_requests_in_flight{service_name, route}  # Gauge
http_request_bytes{service_name, route}  # Histogram
http_response_bytes{service_name, route}  # Histogram
```

### AAP

```
aap_challenge_total{result}  # success / failure
aap_verify_total{result, reason}  # success / failure / expired / replay
aap_challenge_duration_seconds  # Histogram
aap_verify_duration_seconds  # Histogram
aap_active_sessions  # Gauge
aap_nonce_replays_total  # Counter
```

### A2A

```
a2a_token_issued_total  # Counter
a2a_token_revoked_total  # Counter
a2a_token_active  # Gauge
a2a_token_verify_total{result}  # success / failure / revoked / expired
a2a_negotiate_duration_seconds  # Histogram
```

### 后端

```
backend_requests_total{type, op, status}  # Counter
backend_request_duration_seconds_bucket{type, op, le}  # Histogram
backend_pool_size{type, state}  # Gauge (state=active/idle/total)
backend_pool_wait_seconds_bucket{type, le}  # Histogram
```

### 缓存

```
cache_operations_total{backend, result}  # hit / miss / error
cache_operation_duration_seconds_bucket{backend, op, le}  # Histogram
cache_bytes{backend}  # Gauge
cache_keys{backend}  # Gauge
cache_hit_ratio{backend}  # Gauge
```

### 资源

```
go_goroutines  # Gauge
go_memstats_heap_inuse_bytes  # Gauge
go_memstats_heap_alloc_bytes  # Gauge
go_gc_duration_seconds_sum  # Counter
process_cpu_seconds_total  # Counter
process_open_fds  # Gauge
```

## 📈 Grafana 仪表板

完整仪表板定义：[observability/grafana-dashboard.json](../observability/grafana-dashboard.json)

### 面板列表

1. **HTTP 延迟 P50 / P95 / P99**
2. **HTTP 吞吐量 (RPS)**
3. **错误率 (5xx)**
4. **AAP 验证成功率**
5. **活跃 AAP 会话**
6. **后端延迟 P99 (PG / Redis)**
7. **缓存命中率**
8. **A2A Token 颁发 / 撤销**
9. **Goroutine / Heap**

### 导入方式

```bash
# Grafana UI
Dashboards → Import → Upload JSON
# 上传 docs/observability/grafana-dashboard.json

# Prometheus 数据源: 选择你的 Prometheus 实例
```

## 🚨 告警规则

完整告警：[observability/prometheus-alerts.yaml](../observability/prometheus-alerts.yaml)

### 关键告警

| 告警 | 严重度 | 条件 |
|------|--------|------|
| `HighErrorRate` | critical | 5xx 比例 > 1% (5min) |
| `ServiceDown` | critical | `up{} == 0` (1min) |
| `SLOBurnRateFast` | critical | 可用性 < 99% (2min) |
| `HighP99Latency` | warning | P99 > 100ms (5min) |
| `AAPVerifyFailureSpike` | warning | failure > 5/s (5min) |
| `AAPNonceReplayDetected` | critical | 重放 > 0 (5min) |
| `LowCacheHitRate` | warning | 命中率 < 50% (10min) |
| `HighGoroutineCount` | warning | goroutine > 10000 (5min) |
| `HighHeapUsage` | warning | heap > 1GB (5min) |
| `ChainHighFailureRate` | warning | 链 RPC 失败 > 10% (5min) |

## 🛠️ 常用 PromQL

### 错误率

```promql
sum(rate(http_requests_total{status_code=~"5.."}[5m])) by (service_name, route) /
sum(rate(http_requests_total[5m])) by (service_name, route)
```

### P99 延迟

```promql
histogram_quantile(0.99,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service_name, route)
)
```

### AAP 成功率

```promql
sum(rate(aap_verify_total{result="success"}[5m])) /
sum(rate(aap_verify_total[5m]))
```

### 缓存命中率

```promql
sum(rate(cache_operations_total{result="hit"}[5m])) by (backend) /
sum(rate(cache_operations_total{result=~"hit|miss"}[5m])) by (backend)
```

### 慢查询比例

```promql
sum(rate(backend_request_duration_seconds_bucket{type="postgres",op="query",le="0.2"}[5m])) /
sum(rate(backend_request_duration_seconds_bucket{type="postgres",op="query"}[5m]))
```

## 📚 相关

- [SLO 定义](../SLO.md)
- [Grafana 仪表板](../observability/grafana-dashboard.json)
- [Prometheus 告警](../observability/prometheus-alerts.yaml)
- [故障 Runbook](../runbooks/)
