---
name: check-agent-status
version: 2.0.1
role: user
description: 检查指定 Agent 的状态
variables:
  - uuid: Agent UUID
tools:
  - get_agent
---

# 任务

查询指定 Agent 的当前状态。

# 步骤

1. 验证 `uuid` 格式（标准 UUID v7）
2. 调用 `get_agent`：
   ```json
   {
     "uuid": "{uuid}"
   }
   ```
3. 展示状态详情

# 输出

```
🔍 Agent 详情：

| 字段 | 值 |
|------|-----|
| Agent ID | {agent_id} |
| Owner | {owner} |
| Level | {level} |
| Status | {status} |
| Created At | {created_at} |
| Chain TX | {chain_tx_hash} |

{如果 status=active，可执行操作}
{如果 status=revoked，说明撤销时间和原因（如果可查）}
```

# 异常处理

- Agent 不存在：提示用户检查 UUID
- AAP 鉴权失败：提示用户先完成鉴权
