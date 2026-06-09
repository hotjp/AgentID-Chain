# batch-then-verify

> 批量注册后逐一验证状态

## 📋 场景

为多团队注册一批 agent，验证全部成功。

## 🔄 工作流

```
1. 收集团队列表
2. batch_register 调用
3. 解析响应（results + success_count + failure_count）
4. 对失败的项，调用 get_agent 进一步诊断
5. 输出汇总报告
```

## 📝 完整对话

```
USER: 为 team-a, team-b, team-c 各注册 3 个 agent（test 等级）

LLM:
  I'll register 9 agents across 3 teams and verify the results.

TOOL_CALL: batch_register({
  "agents": [
    {"owner": "team-a", "level": "test"},
    {"owner": "team-a", "level": "test"},
    {"owner": "team-a", "level": "test"},
    {"owner": "team-b", "level": "test"},
    {"owner": "team-b", "level": "test"},
    {"owner": "team-b", "level": "test"},
    {"owner": "team-c", "level": "test"},
    {"owner": "team-c", "level": "test"},
    {"owner": "team-c", "level": "test"}
  ]
})

TOOL_RESPONSE:
  {
    "results": [...],
    "success_count": 9,
    "failure_count": 0
  }

ASSISTANT:
  ✅ All 9 agents registered successfully!

  | Team | Count | Agent IDs |
  |------|-------|-----------|
  | team-a | 3 | 0190..., 0190..., 0190... |
  | team-b | 3 | 0190..., 0190..., 0190... |
  | team-c | 3 | 0190..., 0190..., 0190... |

  All agents are active. Use `list_agents` to view them anytime.
```

## 失败处理

如果有失败项：

```
ASSISTANT:
  ⚠️ 8 of 9 agents registered successfully. 1 failed.

  | Agent | Status | Error |
  |-------|--------|-------|
  | team-a-1 | ✅ success | - |
  | team-a-2 | ❌ failure | invalid level |
  | team-a-3 | ✅ success | - |

  Investigating team-a-2...

TOOL_CALL: get_agent({uuid: "..."})  // UUID 不存在 → 确认失败

ASSISTANT:
  team-a-2 failed with "invalid level". This shouldn't happen since
  we used "test" for all. Would you like to:
  1. Retry with level="prod"
  2. Skip this one
  3. Cancel
```

## 🛠️ 实现

```python
def batch_then_verify(agents: list) -> dict:
    result = call_tool("batch_register", {"agents": agents})

    summary = {
        "total": len(agents),
        "success": result["success_count"],
        "failure": result["failure_count"],
        "details": []
    }

    for r in result["results"]:
        if "error" in r:
            # 进一步诊断
            diagnostic = call_tool("get_agent", {"uuid": r.get("agent_id", "")})
            r["diagnostic"] = diagnostic
        summary["details"].append(r)

    return summary
```
