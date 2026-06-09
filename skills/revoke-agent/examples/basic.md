# revoke_agent 示例

**调用**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "revoke_agent",
    "arguments": {
      "uuid": "0190a3b4-7c8d-7def-9abc-def012345678",
      "reason": "compromised key detected"
    }
  }
}
```

**响应**：204 No Content
