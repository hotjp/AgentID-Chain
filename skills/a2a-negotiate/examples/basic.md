# a2a_negotiate 示例

**调用**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "a2a_negotiate",
    "arguments": {
      "agent_id": "0190a3b4-7c8d-7def-9abc-def012345678",
      "scope": ["agents:read", "agents:write"]
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
      "text": "{\"token\":\"eyJhbGciOiJFZERTQS...\",\"expires_in\":3600,\"public_key\":\"kPqJWG...\",\"private_key_encrypted\":\"...\"}"
    }]
  }
}
```
