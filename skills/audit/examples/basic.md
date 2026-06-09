# audit-agent 示例

## 查询最近 24 小时事件

```python
from datetime import datetime, timedelta

since = (datetime.utcnow() - timedelta(days=1)).isoformat() + "Z"
result = call_tool("audit_agent", {
    "agent_id": "0190a3b4-7c8d-7def-9abc-def012345678",
    "since": since
})
print(f"Total events: {result['total']}")
for e in result["events"]:
    print(f"{e['timestamp']} {e['event_type']} by {e['actor']}")
```

## 只查失败的 AAP 验证

```python
result = call_tool("audit_agent", {
    "agent_id": "0190a3b4-7c8d-7def-9abc-def012345678",
    "event_type": "aap_verify"
})
```
