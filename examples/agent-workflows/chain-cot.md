# chain-cot

> Chain-of-Thought 推理 — 显式展示思考过程

## 📋 场景

用户提出复杂需求，LLM 通过思考链逐步推理。

## 📝 示例

```
USER: 我想为生产环境做一个完整的管理流程：注册 3 个 agent，验证状态，然后备份他们的配置

LLM (思考链):

Step 1: 分析需求
  - 用户需要：
    a) 注册 3 个 agent
    b) 验证状态（全部成功）
    c) 备份配置

Step 2: 规划
  - 使用 batch_register 一次完成注册（高效）
  - 使用 get_agent 逐个验证（确认）
  - 备份：返回完整 JSON，让用户保存

Step 3: 收集信息
  - 需要知道：owner、level、metadata
  - 用户未指定 → 询问

Step 4: 选择工具
  - batch_register (3 agents)
  - get_agent (3 次)
  - 不需要备份工具（输出即可）

Step 5: 执行顺序
  1. 询问用户参数
  2. 注册
  3. 验证
  4. 输出 JSON
```

## 📝 完整对话

```
USER: 我想为生产环境做完整管理流程：注册 3 个 agent，验证状态，备份配置

LLM:
  Let me plan this carefully:

  📋 Plan:
  1. Register 3 agents in one batch call
  2. Verify each by querying
  3. Output a JSON backup of all 3

  To proceed, I need:
  - 3 owner names (or 1 owner for all 3)
  - 3 levels (test/prod)
  - Optional: metadata for each

  What would you like to use?

USER: 都是 alice，level 分别是 test、prod、prod

LLM:
  Got it. Registering now.

TOOL_CALL: batch_register({
  "agents": [
    {"owner": "alice", "level": "test"},
    {"owner": "alice", "level": "prod"},
    {"owner": "alice", "level": "prod"}
  ]
})

TOOL_RESPONSE: {success: 3, results: [...]}

LLM:
  Registered 3 agents. Verifying...

TOOL_CALL: get_agent({uuid: "..."})
TOOL_CALL: get_agent({uuid: "..."})
TOOL_CALL: get_agent({uuid: "..."})

LLM:
  ✅ All 3 agents verified.

  📦 Backup JSON (save this):
  ```json
  {
    "agents": [
      {
        "agent_id": "0190a3b4-...",
        "owner": "alice",
        "level": "test",
        "status": "active",
        "created_at": "2026-06-09T12:00:00Z"
      },
      {
        "agent_id": "0190a3b5-...",
        "owner": "alice",
        "level": "prod",
        "status": "active",
        "created_at": "2026-06-09T12:00:01Z"
      },
      {
        "agent_id": "0190a3b6-...",
        "owner": "alice",
        "level": "prod",
        "status": "active",
        "created_at": "2026-06-09T12:00:02Z"
      }
    ],
    "backup_at": "2026-06-09T12:00:03Z"
  }
  ```
```

## 🛠️ 关键要点

1. **显式思考**：让 LLM 输出 `Plan:` 部分
2. **分解需求**：将复杂任务拆为子任务
3. **选择合适工具**：避免冗余调用
4. **聚合输出**：批量结果合并展示

## 📚 提示技巧

在 system prompt 中加入：

```
When handling complex requests:
1. Output a "Plan:" section showing your reasoning
2. Break down into sub-tasks
3. Identify the right tools for each
4. Execute in optimal order
5. Aggregate results
```
