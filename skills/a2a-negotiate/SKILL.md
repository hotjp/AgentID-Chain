# a2a-negotiate

> 协商 A2A Token（Agent-to-Agent 互信）

## 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `agent_id` | string | ✅ | 申请方 Agent UUID |
| `scope` | array | ✅ | 权限范围（如 `["agents:read"]`） |

## 返回

```json
{
  "token": "eyJhbGciOiJFZERTQS...",  // A2A JWT
  "expires_in": 3600,
  "public_key": "kPqJWG...",         // 用于验签的 Ed25519 公钥
  "private_key_encrypted": "..."    // 加密的 A2A 私钥（用 AAP JWT 解密）
}
```

## 错误

| 错误码 | 含义 |
|--------|------|
| 401 | AAP 鉴权失败 |
| 403 | 申请方 Agent 无权申请此 scope |
