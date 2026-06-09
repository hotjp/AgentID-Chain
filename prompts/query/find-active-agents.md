---
name: find-active-agents
version: 2.0.1
role: user
description: 查找活跃 Agent
variables:
  - owner: 按 owner 过滤
tools:
  - list_agents
---

# 任务

查找指定 owner 的**所有活跃 agent**。

# 步骤

1. 确认 `owner`（如未提供，询问用户）
2. 可选过滤：`level`、`status`
3. 调用 `list_agents`：
   ```json
   {
     "owner": "{owner}",
     "status": "active",
     "limit": 200
   }
   ```
4. 如果有 `next_cursor`，继续翻页
5. 展示为表格

# 输出

```
📋 {owner} 的活跃 agent（共 N 个）：

| Agent ID | Level | Created At |
|----------|-------|------------|
| 0190... | test | 2026-06-09 |
| 0190... | prod | 2026-06-08 |
...
```

# 提示

- 默认按 `created_at DESC` 排序
- 如需按其他字段排序，请告知
