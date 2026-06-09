# register-and-query

> 注册新 Agent 后立即查询验证

## 📋 场景

用户想注册 agent 并立即确认其状态。

## 🔄 工作流

```
用户：帮我注册一个 alice 的 test agent

LLM（思考）：
  1. 用户想注册 agent
  2. 需要先 AAP 鉴权（但系统已处理，假设已认证）
  3. 调用 register_agent
  4. 拿到 agent_id 后自动查询
  5. 展示结果
```

## 📝 完整对话

```
USER: 帮我注册一个 alice 的 test agent

LLM:
  I'll help you register a test agent for alice and verify its status.

  Step 1: Register the agent

TOOL_CALL: register_agent({
  "owner": "alice",
  "level": "test"
})

TOOL_RESPONSE:
  {"agent_id": "0190a3b4-7c8d-7def-9abc-def012345678", "status": "active", ...}

  Step 2: Verify the registration

TOOL_CALL: get_agent({
  "uuid": "0190a3b4-7c8d-7def-9abc-def012345678"
})

TOOL_RESPONSE:
  {"agent_id": "0190a3b4-7c8d-7def-9abc-def012345678", "owner": "alice", "level": "test", "status": "active", "created_at": "2026-06-09T12:34:56Z"}

ASSISTANT:
  ✅ Agent registered and verified!

  | Field | Value |
  |-------|-------|
  | Agent ID | 0190a3b4-7c8d-7def-9abc-def012345678 |
  | Owner | alice |
  | Level | test |
  | Status | active |
  | Created | 2026-06-09 12:34:56 UTC |

  The agent is ready to use. What would you like to do next?
  - Upgrade to prod
  - Register more agents
  - Revoke this one
```

## 🛠️ 实现

```python
def register_and_query_workflow(owner: str, level: str):
    # Step 1: register
    agent = call_tool("register_agent", {"owner": owner, "level": level})

    if "error" in agent:
        return f"❌ Registration failed: {agent['error']}"

    agent_id = agent["agent_id"]

    # Step 2: query (verify)
    verified = call_tool("get_agent", {"uuid": agent_id})

    return format_agent_table(verified)
```
