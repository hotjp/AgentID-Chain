# list_agents 示例

**调用**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_agents",
    "arguments": {
      "owner": "alice",
      "level": "test",
      "limit": 10
    }
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
      "text": "{\"agents\":[{\"agent_id\":\"...\",\"owner\":\"alice\",\"level\":\"test\",\"status\":\"active\",\"created_at\":\"2026-06-09T...\"}],\"next_cursor\":null}"
    }]
  }
}
```
