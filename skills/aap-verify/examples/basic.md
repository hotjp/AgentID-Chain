# aap_verify 示例

**调用**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "aap_verify",
    "arguments": {
      "challenge": "dG9wIHNlY3JldA==",
      "signature": "8JKj...base64...64B",
      "public_key": "kPqJWG...base64...32B"
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
      "text": "{\"aap_token\":\"eyJhbGciOiJFZERTQSIs...\",\"expires_in\":3600}"
    }]
  }
}
```
