# get_agent 示例

**调用**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_agent",
    "arguments": {"uuid": "0190a3b4-7c8d-7def-9abc-def012345678"}
  }
}
```

**响应**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{
      "type": "text",
      "text": "{\"agent_id\":\"0190a3b4-7c8d-7def-9abc-def012345678\",\"owner\":\"alice\",\"level\":\"test\",\"status\":\"active\",\"created_at\":\"2026-06-09T12:34:56Z\"}"
    }]
  }
}
```
