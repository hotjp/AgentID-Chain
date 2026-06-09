# 客户公告模板

> 用于产品变更通知

---

**主题**：[AgentID-Chain] YYYY-MM-DD 计划维护 / 紧急通知 / 新功能上线

**发送时间**：YYYY-MM-DD HH:MM UTC

**优先级**：🔴 高（影响服务） / 🟡 中（重要变更） / 🟢 低（一般通知）

---

## 摘要

1-2 句话讲清楚要通知什么。

**示例**：
> 计划于 2026-07-01 02:00-04:00 UTC 进行版本升级，预计服务中断 30 分钟。

---

## 影响范围

- **受影响服务**：[所有 / 注册 / 查询 / 链]
- **影响程度**：[完全中断 / 性能降级 / 短暂延迟 / 无影响]
- **预计时间窗口**：YYYY-MM-DD HH:MM-HH:MM UTC
- **影响客户**：[全部 / 付费用户 / 指定 region]

## 变更内容

### 新功能

- 列出本次新增能力
- 附文档链接

### 修复

- 列出主要 bug 修复
- 关联 CVE（如有）

### 破坏性变更

> ⚠️ **重要**：以下变更需要客户配合

- 列出 API 变更
- 提供迁移代码示例
- 给出 EOL 时间

## 行动项（客户必做）

按优先级排：

1. **必须**：[如有]
2. **建议**：[如有]
3. **可选**：[如有]

## 时间表

| 时间 (UTC) | 事件 |
|-----------|------|
| YYYY-MM-DD HH:MM | 通知发出 |
| YYYY-MM-DD HH:MM | staging 升级 |
| YYYY-MM-DD HH:MM | 灰度 10% |
| YYYY-MM-DD HH:MM | 灰度 50% |
| YYYY-MM-DD HH:MM | 全量 |
| YYYY-MM-DD HH:MM | 升级完成 |

## 风险与回滚

- 已知风险：
- 回滚计划：
- 联系支持：

## 支持渠道

- 邮件：support@agentid-chain.example
- Slack：[invite link]
- 状态页：https://status.agentid-chain.example
- 文档：https://docs.agentid-chain.example

## 致谢

感谢您的支持！

— AgentID-Chain 团队
