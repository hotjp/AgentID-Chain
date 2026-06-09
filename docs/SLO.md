# AgentID-Chain — SLO 制定 (P18.13)

> 范围：API 网关 + 鉴权 + 存储 全链路
> 版本：v2.0.1
> 制定日期：2026-06-09
> 评审周期：每季度

## 1. SLO 概览

| 服务 | 指标 | SLO 目标 | 错误预算（30天） | 测量窗口 |
|------|------|---------|---------------|---------|
| API Gateway | 可用性 | **99.9%** | 43.2 min/月 | 滚动 30 天 |
| API Gateway | 延迟 P99 | **< 100ms** | 1% 请求 > 100ms | 滚动 30 天 |
| API Gateway | 错误率 | **< 0.1%** | 0.1% 请求 5xx | 滚动 30 天 |
| 鉴权层 | AAP 握手 P99 | **< 5ms** | 1% 请求 > 5ms | 滚动 30 天 |
| 鉴权层 | RBAC 检查 P99 | **< 1ms** | 0.5% 请求 > 1ms | 滚动 30 天 |
| 存储层 | Register 端点 P99 | **< 50ms** | 1% 请求 > 50ms | 滚动 30 天 |
| 存储层 | Query 慢查询率 | **< 0.5%** | 0.5% 查询 > 200ms | 滚动 30 天 |
| 端到端 | 注册成功链路 P99 | **< 200ms** | 1% 请求 > 200ms | 滚动 30 天 |

## 2. SLI 定义

### 2.1 可用性 SLI

```
可用性 = 成功请求数 / 总请求数
"成功" = 状态码 < 500
```

**排除**（不算失败）：
- 4xx（客户端错误，预期）
- 客户端取消 (`< EOF>`)
- 维护窗口（通过 Header `X-Maintenance: true` 排除）

### 2.2 延迟 SLI

```
延迟 SLI = 满足 P99 目标的请求比例
P99 目标 = 服务级别目标延迟
```

**测量方式**：
- 从 `request_received_at` 到 `response_sent_at`
- 排除静态资源（CSS/JS/图片）
- 排除非业务端点（/live、/ready、/metrics）

### 2.3 错误率 SLI

```
错误率 = 5xx 状态码请求 / 总请求
```

**包含**：
- 500 Internal Server Error
- 502 Bad Gateway
- 503 Service Unavailable
- 504 Gateway Timeout

**排除**：
- 业务 4xx（认证失败、参数错误等）

## 3. 错误预算（Error Budget）

### 3.1 月度预算（30天）

可用性 99.9%：
```
30 天 × 24h × 60min = 43200 min
错误预算 = 43200 × 0.001 = 43.2 分钟
```

延迟 P99 < 100ms（1% 容差）：
```
1,000,000 请求/月 → 10,000 请求可超 100ms
```

### 3.2 预算消耗率告警

| 消耗率 | 状态 | 行动 |
|--------|------|------|
| < 50% | 🟢 健康 | 持续优化 |
| 50-80% | 🟡 警告 | 排查根因 |
| 80-95% | 🟠 风险 | 暂停非关键变更 |
| > 95% | 🔴 耗尽 | 冻结所有变更，紧急修复 |

## 4. 分服务 SLO

### 4.1 API Gateway

```yaml
slo:
  availability:
    objective: 99.9%
    sli: http_status_class{class="success"} / http_status_class
  latency:
    objective: p99 < 100ms
    sli: http_request_duration_seconds{quantile="0.99"}
  error_rate:
    objective: < 0.1%
    sli: http_requests_total{status=~"5.."} / http_requests_total
```

**关键依赖**：
- Auth Center（SLO 99.9%）
- Tag Sense（SLO 99.9%）
- PostgreSQL（SLO 99.95%）
- Redis（SLO 99.95%）

### 4.2 Authz Layer (L3)

| 端点 | P50 | P95 | P99 | 错误率 |
|------|-----|-----|-----|--------|
| `/v1/aap/handshake` | < 5ms | < 20ms | < 50ms | < 0.1% |
| `/v1/aap/verify` | < 2ms | < 5ms | < 10ms | < 0.05% |
| `/v1/aap/token` | < 1ms | < 3ms | < 5ms | < 0.05% |
| RBAC `Check` | < 0.01ms | < 0.05ms | < 0.1ms | 0% |

### 4.3 Service Layer (L4)

| 端点 | P50 | P95 | P99 | 错误率 |
|------|-----|-----|-----|--------|
| Register | < 20ms | < 35ms | < 50ms | < 0.1% |
| Upgrade | < 30ms | < 50ms | < 80ms | < 0.1% |
| Get Info | < 5ms | < 15ms | < 30ms | < 0.05% |
| Batch Register | < 200ms | < 500ms | < 1s | < 0.5% |

### 4.4 Storage Layer (L1)

| 操作 | P50 | P95 | P99 |
|------|-----|-----|-----|
| PG 单 INSERT | < 2ms | < 5ms | < 10ms |
| PG 单 SELECT (by PK) | < 1ms | < 3ms | < 5ms |
| PG 事务（5 写） | < 10ms | < 20ms | < 50ms |
| Redis GET | < 0.5ms | < 1ms | < 2ms |
| Redis Pipeline (10 ops) | < 1ms | < 2ms | < 5ms |
| Outbox publish | < 5ms | < 15ms | < 30ms |

## 5. 端到端 SLO

### 5.1 核心路径 SLO

| 路径 | 端到端 P99 |
|------|-----------|
| Register → Store | < 100ms |
| Register → Chain (onchain) | < 5s |
| Get Info (cached) | < 10ms |
| Get Info (cold) | < 30ms |
| AAP Handshake → Verify → Token | < 50ms |

### 5.2 计算公式

```
端到端 P99 ≈ Σ(各环节 P99) × 安全系数 1.2
```

示例 Register：
```
5ms (TLS) + 5ms (UA) + 5ms (RBAC) + 50ms (Service) + 10ms (DB) + 20ms (Chain) × 1.2 = ~120ms
```

实际 SLO 目标定为 100ms（保留 buffer）。

## 6. 监控与告警

### 6.1 关键 SLI 指标

```promql
# 可用性
sum(rate(http_requests_total{status!~"5.."}[5m])) /
sum(rate(http_requests_total[5m]))

# P99 延迟
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))

# 错误率
sum(rate(http_requests_total{status=~"5.."}[5m])) /
sum(rate(http_requests_total[5m]))
```

### 6.2 告警规则

```yaml
groups:
- name: slo_alerts
  rules:
  - alert: SLO_Availability_BudgetBurning
    expr: slo_availability_budget_burn_rate > 2
    for: 5m
    annotations:
      summary: "可用性 SLO 预算消耗速率 > 2x"
      runbook: "https://wiki/runbooks/slo/availability"

  - alert: SLO_Latency_P99_Breaching
    expr: histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 0.1
    for: 5m
    annotations:
      summary: "API Gateway P99 延迟 > 100ms"
      runbook: "https://wiki/runbooks/slo/latency"

  - alert: SLO_ErrorRate_Spike
    expr: sum(rate(http_requests_total{status=~"5.."}[1m])) /
          sum(rate(http_requests_total[1m])) > 0.005
    for: 2m
    annotations:
      summary: "5xx 错误率 > 0.5%"
```

### 6.3 仪表板

Grafana Dashboard：
- "AgentID-Chain / SLO Overview" —— 全局 SLO 状态
- "AgentID-Chain / Burn Rate" —— 预算消耗速率
- "AgentID-Chain / Per-Service" —— 各服务分解

## 7. 报告

### 7.1 周报

- 错误预算消耗（绝对值 + 速率）
- Top 3 慢查询
- Top 3 错误源
- 本周事件回顾

### 7.2 月报

- 月度 SLO 达成率
- 错误预算剩余
- 改进措施
- 下一季度 SLO 调整建议

## 8. 引用

- Google SRE Book: https://sre.google/sre-book/service-level-objectives/
- Prometheus SLO: https://prometheus.io/docs/practices/instrumentation/#service-level-objectives
- OpenSLO: https://openslo.com/
