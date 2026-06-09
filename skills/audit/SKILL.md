# audit-agent

> 查询 Agent 操作审计日志

## 📋 描述

查询指定 agent 在系统中的所有状态变更、操作记录、签名验证等审计事件。

**适用场景**：

- 排查安全事件
- 合规审计
- 行为追溯

## 🔧 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `agent_id` | string | ✅ | Agent UUID |
| `since` | string | | ISO8601 起始时间 |
| `until` | string | | ISO8601 截止时间 |
| `event_type` | string | | 过滤：register / upgrade / revoke / aap_verify |
| `limit` | int | | 最多返回条数（默认 100） |

## 📤 返回

```json
{
  "agent_id": "0190a3b4-7c8d-7def-9abc-def012345678",
  "events": [
    {
      "timestamp": "2026-06-09T12:34:56Z",
      "event_type": "register",
      "actor": "alice",
      "details": {"level": "test"},
      "trace_id": "abc123"
    },
    {
      "timestamp": "2026-06-09T13:00:00Z",
      "event_type": "upgrade",
      "actor": "alice",
      "details": {"from": "test", "to": "prod"},
      "trace_id": "def456"
    }
  ],
  "total": 2
}
```

## 🛠️ 实现

```python
def audit_agent(agent_id, since=None, until=None, event_type=None, limit=100):
    args = {"agent_id": agent_id, "limit": limit}
    if since: args["since"] = since
    if until: args["until"] = until
    if event_type: args["event_type"] = event_type
    return call_tool("audit_agent", args)
```

## 📚 提示技巧

- 配合 `since` / `until` 缩小范围，避免一次性返回太多
- 安全事件追溯时，先查 `aap_verify` 失败记录
- 长期审计建议导出到 SIEM 系统
