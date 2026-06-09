---
name: postmortem
version: 2.0.1
role: system
description: 事故复盘 (postmortem) 模板与流程
---

# Postmortem 模板

事故发生后 48 小时内完成此文档。无指责文化，专注系统改进。

## 📋 文档结构

```markdown
# Postmortem: <简短标题>

**事故 ID**: PM-YYYY-NNN
**日期**: YYYY-MM-DD
**作者**: @name
**评审人**: @name1, @name2
**状态**: Draft | Review | Final
**严重度**: SEV1 (停服) | SEV2 (降级) | SEV3 (局部)

---

## 1. 摘要 (TL;DR)

用 3-5 句话讲清楚：
- 发生了什么
- 影响范围（多少用户/请求/数据）
- 何时发现 / 何时恢复
- 根因（一句话）

## 2. 时间线 (Timeline)

按 UTC 时间顺序记录关键事件：

| 时间 (UTC) | 事件 | 操作人 | 链接 |
|-----------|------|--------|------|
| 14:23 | 部署 v2.0.1 (commit abc) | @alice | [PR#123](#) |
| 14:25 | 错误率从 0.1% 升至 8% | (监控) | [Grafana](#) |
| 14:27 | 告警触发 (#alert-456) | Prometheus | - |
| 14:30 | @bob oncall 介入 | @bob | [Slack](#) |
| 14:35 | 定位到 DB 连接池耗尽 | @bob | - |
| 14:40 | 临时扩容 pool_size: 50 | @bob | [config diff](#) |
| 14:42 | 错误率回落到 0.5% | (监控) | - |
| 14:50 | 决定回滚到 v2.0.0 | @alice | - |
| 15:05 | 回滚完成，错误率 0% | (监控) | - |

## 3. 影响 (Impact)

- **用户影响**：约 5% 用户在 25 分钟内遇到 5xx
- **请求量**：约 8000 次失败请求
- **业务影响**：3 个客户报告问题，其中 1 个要求 SLA 报告
- **SLO 影响**：月错误率预算消耗 12%（burn rate 高峰 18×）

## 4. 根因 (Root Cause)

### 4.1 直接原因

DB 连接池默认 max_open=25，新部署引入了 N+1 查询，单请求占用连接数 ×3。

### 4.2 深层原因

- 配置默认值未考虑业务高峰
- 缺乏连接池耗尽的快速诊断
- 上线前未做负载测试

## 5. 触发因素 (Trigger)

PR #123 引入 `batch_register` 接口，未优化 N+1 查询。

## 6. 为什么没发现 (Why not caught)

- 单测覆盖率足够，但未跑压测
- staging 环境与生产流量差异大
- 缺乏 connection pool 使用率告警

## 7. 检测与响应 (Detection & Response)

- **检测延迟**：2 分钟（监控告警）
- **响应延迟**：7 分钟（oncall 介入）
- **缓解延迟**：17 分钟（扩容）
- **恢复延迟**：42 分钟（回滚）

**做得好的**：
- 监控告警及时
- oncall 5 分钟内响应

**可改进的**：
- runbook 缺少"连接池耗尽"条目
- 回滚决策花了 15 分钟（应 < 5 分钟）

## 8. 改进项 (Action Items)

按优先级排：

| # | 行动 | 优先级 | 负责人 | 截止日 |
|---|------|--------|--------|--------|
| 1 | 默认 max_open 提升到 100 + 文档说明 | P0 | @alice | 2026-06-15 |
| 2 | 添加 connection pool 使用率告警 (>80%) | P0 | @bob | 2026-06-12 |
| 3 | batch_register N+1 优化（ent Eager Loading）| P1 | @carol | 2026-06-20 |
| 4 | CI 增加 k6 压测 step | P1 | @dave | 2026-06-25 |
| 5 | runbook 增补"连接池耗尽"条目 | P2 | @bob | 2026-06-30 |

## 9. 经验教训 (Lessons Learned)

- **技术**：连接池默认值需根据真实负载设定
- **流程**：staging 环境与生产流量模型差异需量化
- **沟通**：客户 SLA 报告应在 24h 内发出
- **工具**：k6 压测门槛低，应纳入 CI

## 10. 参考

- 相关 PR: #123
- 相关告警: #alert-456
- 监控面板: [Grafana Dashboard]
- 关联事故: PM-2026-005
- 上游 issue: #L1-DB-42
```

## 📝 评审流程

1. **作者**：24h 内完成初稿
2. **团队评审**：48h 内收集反馈（异步）
3. **评审会议**：1h 同步会议
4. **最终发布**：标记为 Final，公开（敏感信息脱敏）
5. **追踪**：action items 进入 sprint backlog

## 🤝 文化

- ✅ **Blameless**：不追责个人，专注系统改进
- ✅ **Bugs are welcome**：复现越快修越快
- ✅ **5 Whys**：问 5 个为什么找到根因
- ❌ **No finger-pointing**：禁止"谁写的代码"
- ❌ **No rushing**：48h 截止不打折

## 📚 参考

- [Google SRE Book - Postmortem Culture](https://sre.google/sre-book/postmortem-culture/)
- [Etsy Debriefing Facilitation](https://extfiles.etsy.com/DebriefingFacilitationGuide.pdf)
- [Atlassian Incident Handbook](https://www.atlassian.com/incident-management/handbook/post-mortems)
