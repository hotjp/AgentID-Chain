# On-Call 轮值

> AgentID-Chain 7×24 值班制度

## 👥 轮值表

按周轮换（一组 2 人：主 + 副）：

| 周 | 主 Oncall | 副 Oncall |
|----|----------|----------|
| W26 (2026-06-22) | @alice | @bob |
| W27 (2026-06-29) | @carol | @dave |
| W28 (2026-07-06) | @eve | @frank |
| ... | ... | ... |

**轮值节奏**：周一 09:00 UTC 交接。

## 🎯 责任

主 Oncall：

- 7×24 响应告警（5min 内）
- 初步诊断 + 缓解
- 必要时升级 SEV1+ 给 Tech Lead
- 记录 incident 时间线

副 Oncall：

- 备份主 Oncall
- 主 Oncall 不可达时接管
- 复杂事故中协作

## 🔔 通知渠道

| 渠道 | 用途 |
|------|------|
| **PagerDuty** | 告警（SEV1 自动 page） |
| **Slack #oncall** | 日常讨论 |
| **Slack #incident** | 事故时 |
| **电话** | SEV0（直接呼叫） |

## ⏱️ 响应时间

| 严重度 | 响应 | 缓解 |
|--------|------|------|
| SEV0 | 立即 | 30min |
| SEV1 | 5min | 1h |
| SEV2 | 15min | 4h |
| SEV3 | 1h | 24h |

## 💰 补偿

- 工作日：调休 1 天
- 周末/节假日：调休 2 天
- 实际 incident 处理：额外奖金（按事故等级）

## 📋 交接清单

### 上周 Oncall 交接给本周

- [ ] 任何进行中的 incident / 问题
- [ ] 已知告警 / 抖动
- [ ] 计划中的维护窗口
- [ ] 本周 release / 部署计划
- [ ] 任何待办 postmortem

### 工具检查

- [ ] PagerDuty 通知正常（演练）
- [ ] Slack / 电话可达
- [ ] VPN / kubectl / helm 可用
- [ ] 跑通 [runbooks/](runbooks/) 中至少 1 个

## 🚨 升级路径

```
Oncall
  ↓ (15min 无进展 或 SEV1+)
Tech Lead
  ↓ (SEV0/SEV1 持续 30min)
VP Eng
  ↓ (业务影响严重)
CEO
```

## 📚 工具与文档

- 事故响应：[INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md)
- Runbook：[runbooks/](runbooks/)
- 监控：Grafana dashboard
- 告警：Prometheus alerts
- 日志：kubectl logs / CloudWatch
- 链：https://polygonscan.com / https://bscscan.com

## 🎓 培训

新人 Oncall 必过：

1. 完整读 [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md) + 5 个 runbook
2. 跟随 1 次 oncall（影子）
3. 完成 1 次模拟 incident 演练
4. 单独值 1 周（可申请延长辅导）
