# batch_register 示例

**调用**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "batch_register",
    "arguments": {
      "agents": [
        {"owner": "alice", "level": "test"},
        {"owner": "bob", "level": "prod"},
        {"owner": "carol", "level": "test", "metadata": {"team": "infra"}}
      ]
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
      "text": "{\"results\":[{\"agent_id\":\"...\",\"owner\":\"alice\",\"level\":\"test\",\"status\":\"active\"},{\"agent_id\":\"...\",\"owner\":\"bob\",\"level\":\"prod\",\"status\":\"active\"},{\"agent_id\":\"...\",\"owner\":\"carol\",\"level\":\"test\",\"status\":\"active\"}],\"success_count\":3,\"failure_count\":0}"
    }]
  }
}
```
