# Incident Response

> AgentID-Chain 事故响应手册

## 🎯 事故分级

| 等级 | 定义 | 响应时间 | 沟通频率 | 升级 |
|------|------|---------|---------|------|
| **SEV0** | 全停 / 数据丢失 | 立即 | 5min | CEO / 全员 |
| **SEV1** | 主功能不可用，>10% 用户受影响 | 15min | 15min | VP Eng |
| **SEV2** | 降级 / 部分功能不可用 | 1h | 30min | Tech Lead |
| **SEV3** | 轻微问题 / 单个客户 | 1d | 2h | Oncall |

## 🚨 触发条件（自动）

Prometheus 告警：

```yaml
- alert: AgentIDHighErrorRate
  expr: |
    sum(rate(http_requests_total{status=~"5.."}[5m]))
    / sum(rate(http_requests_total[5m])) > 0.05
  for: 5m
  severity: sev1

- alert: AgentIDHighLatency
  expr: |
    histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 1
  for: 10m
  severity: sev2

- alert: AgentIDDown
  expr: up{job="agentid-chain"} == 0
  for: 2m
  severity: sev0
```

## 👥 角色

| 角色 | 职责 |
|------|------|
| **Incident Commander (IC)** | 协调 / 决策 / 沟通 |
| **Tech Lead** | 技术调查 / 修复 |
| **Comms Lead** | 客户 / 内部沟通 |
| **Scribe** | 记录时间线 / 行动 |
| **Oncall** | 第一时间响应（默认 IC） |

## 🔄 响应流程

### Phase 1: 检测与确认（< 5min）

```bash
# 1. 告警触发
# 2. Oncall 5min 内确认
# 3. 在 #incident 创建频道
/incident new "SEV1: 错误率激增"

# 4. 初步判断
- 看 Grafana: https://grafana.internal/d/agentid
- 看 trace: https://jaeger.internal/search
- 看日志: agentid logs --tail=200 -f
```

### Phase 2: 评估与升级（< 15min）

```bash
# 5. 评估影响
- 受影响用户：5% / 50% / 100%
- 受影响功能：注册 / 查询 / 链
- 持续时间：5min / 30min / 1h

# 6. 决定是否升级 SEV
- SEV0/SEV1：通知 VP Eng + 客户
- SEV2：Slack 频道
- SEV3：Jira ticket 即可
```

### Phase 3: 缓解（< 30min）

参考 [runbooks/](runbooks/)：

- [高错误率](runbooks/high-error-rate.md)
- [DB 连接池](runbooks/db-connection-pool.md)
- [链 RPC 失败](runbooks/chain-rpc-failure.md)
- [AAP 重放攻击](runbooks/aap-replay-attack.md)
- [磁盘压力](runbooks/disk-pressure.md)

**缓解优先于根因**：

1. 先恢复服务（扩容 / 回滚 / 限流）
2. 再找根因
3. 最后修复

### Phase 4: 沟通

**对内**（#incident 频道）：

```
[15:30] IC @alice: SEV1 确认
[15:32] @bob: 看 Grafana 错误率从 0.1% → 8%
[15:35] @bob: 定位到 DB 连接池耗尽
[15:40] @bob: 临时扩容 pool_size 25 → 50
[15:42] IC: 错误率回落到 0.5%
[15:50] IC: 决定回滚到 v2.0.1
[16:05] @bob: 回滚完成，错误率 0%
[16:10] IC: SEV1 关闭，转事后复盘
```

**对外**（客户）：

模板见 [templates/customer_advisory.md](../templates/customer_advisory.md)

### Phase 5: 关闭与复盘

- [ ] 错误率 / 延迟回到 SLO 内
- [ ] 监控 30 分钟无异常
- [ ] 关闭 incident 频道
- [ ] 48h 内完成 [postmortem](../templates/postmortem.md)
- [ ] 跟进 action items

## 🛠️ 应急工具

```bash
# 立即降级（非破坏）
agentid config set read_only=true
agentid config set chain.mode=mock

# 立即回滚
helm rollback agentid-chain <revision>

# 立即扩容
kubectl scale deployment/agentid-chain --replicas=10 -n agentid

# 立即限流
agentid config set rate_limit.per_ip=10

# 立即关闭
kubectl scale deployment/agentid-chain --replicas=0 -n agentid
```

## 📚 详细 Runbook

- [高错误率](runbooks/high-error-rate.md)
- [DB 连接池](runbooks/db-connection-pool.md)
- [链 RPC 失败](runbooks/chain-rpc-failure.md)
- [AAP 重放攻击](runbooks/aap-replay-attack.md)
- [磁盘压力](runbooks/disk-pressure.md)
- [ROLLBACK.md](ROLLBACK.md)
- [postmortem 模板](../templates/postmortem.md)
