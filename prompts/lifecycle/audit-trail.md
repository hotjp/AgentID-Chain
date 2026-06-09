---
name: audit-trail
version: 2.0.1
role: user
description: 查询 Agent 的审计轨迹
variables:
  - uuid: Agent UUID
tools:
  - get_agent
  - list_audit_logs
---

# 任务

查询 Agent 的完整操作历史（审计轨迹）。

# 步骤

1. 获取 Agent 当前状态
2. 查询 `audit_logs` 表（按时间排序）
3. 包含：
   - Register
   - Upgrade
   - Revoke
   - Failed verify attempts
   - Chain confirmations

# 输出

```
📜 Agent 审计轨迹：{uuid}

| 时间 | 操作 | 结果 | 详情 | 操作者 |
|------|------|------|------|--------|
| 2026-06-09 12:34 | register | success | level=test | alice |
| 2026-06-09 13:00 | upgrade | success | test→prod | alice |
| 2026-06-09 14:30 | verify | failure | reason=replay | unknown |

共计 N 条记录
```

# 异常

- 无审计日志 → Agent 不存在或日志被清理
- 部分记录缺失 → 提示用户
