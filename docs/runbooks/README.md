# Runbook 索引

> 故障场景的标准应对流程

## 📋 索引

| Runbook | 严重度 | 触发条件 | 响应 SLA |
|---------|--------|---------|---------|
| [高错误率](high-error-rate.md) | P0 | 5xx 比例 > 1% (5min) | 5min |
| [数据库连接池耗尽](db-connection-pool.md) | P0 | 连接池使用率 > 95% | 10min |
| [链上 RPC 失败](chain-rpc-failure.md) | P1 | chain RPC 失败 > 10% (5min) | 30min |
| [AAP 重放攻击](aap-replay-attack.md) | P0 | `aap_nonce_replays_total > 0` | 5min |
| [磁盘压力](disk-pressure.md) | P0 | 磁盘使用 > 85% | 15min |

## 🔧 通用 Runbook 模板

```markdown
# <Runbook 标题>

## 严重度
P0 / P1 / P2

## 触发告警
<告警名> — <条件>

## 症状
- <指标异常>
- <用户影响>

## 立即行动（5min 内）
1. ...

## 诊断
### 看指标
- 仪表板: ...
- PromQL: ...

### 看日志
```bash
...
```

### 看 trace
```bash
...
```

## 缓解（短期）
1. ...

## 修复（根本）
1. ...

## 验证
- [ ] <指标恢复到正常>
- [ ] <告警已恢复>

## 沟通
- 群: ...
- 升级路径: ...
```

## 🆘 紧急联系人

| 角色 | 联系方式 |
|------|---------|
| On-call Engineer | PagerDuty |
| Tech Lead | @... |
| DBA | @... |
| Security | security@... |
