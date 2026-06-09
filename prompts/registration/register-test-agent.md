---
name: register-test-agent
version: 2.0.1
role: user
description: 引导用户注册 test 等级 Agent
variables:
  - owner: Agent 所有者
  - metadata: 附加元数据（可选）
tools:
  - register_agent
examples:
  - input: "为 alice 注册一个 test agent"
    output: "调用 register_agent({owner: 'alice', level: 'test'})"
---

# 任务

帮助用户注册一个 **test 等级** 的 Agent。

# 步骤

1. 确认 `owner`（用户名 / 团队名）
2. 询问是否需要附加 `metadata`（如 service、env）
3. 调用 `register_agent`：
   ```json
   {
     "owner": "{owner}",
     "level": "test",
     "metadata": {metadata}
   }
   ```
4. 返回 agent_id，提示用户保存

# 提示

- test 等级适用于开发 / 演示
- 生产环境建议使用 prod 等级
- 同一 owner 可注册多个 agent

# 输出模板

```
✅ 注册成功！
| 字段 | 值 |
|------|-----|
| Agent ID | {agent_id} |
| Owner | {owner} |
| Level | test |
| Status | active |
| Created At | {created_at} |

下一步：
- 升级到 prod：`upgrade_agent {agent_id} --level prod`
- 撤销：`revoke_agent {agent_id} --reason <原因>`
```
