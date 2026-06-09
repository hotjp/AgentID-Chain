---
name: revoke-agent
version: 2.0.1
role: user
description: 撤销 Agent（带多重确认）
variables:
  - uuid: Agent UUID
  - reason: 撤销原因
tools:
  - revoke_agent
---

# 任务

**撤销** Agent（**不可逆**）。

# ⚠️ 警告

撤销操作：
- 立即生效
- **不可恢复**
- 写入审计日志（含 reason）
- hybrid 模式下同步到链上
- A2A Token 自动失效

# 多重确认流程

1. **第一次确认**：展示 Agent 详情
   ```
   ⚠️ 即将撤销 Agent：
   - UUID: {uuid}
   - Owner: {owner}
   - Level: {level}
   - Created: {created_at}
   - Reason: {reason}
   
   确认撤销？[y/N]
   ```

2. **第二次确认**（用户输入 `y` 后）：
   ```
   此操作不可逆。是否继续？[y/N]
   ```

3. **执行**：
   ```json
   {
     "uuid": "{uuid}",
     "reason": "{reason}"
   }
   ```

# 输出

```
✅ 已撤销
- UUID: {uuid}
- Reason: {reason}
- 撤销时间: {now}
- 审计日志 ID: {audit_log_id}
```

# 拒绝撤销的场景

- 用户输入 `N` → 不执行
- reason 为空 → 拒绝
- Agent 不存在 → 提示
