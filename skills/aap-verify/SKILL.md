# aap-verify

> 验证 AAP Challenge 签名

## 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `challenge` | string | ✅ | 服务端颁发的 challenge (base64) |
| `signature` | string | ✅ | 客户端签名 (base64, 64B) |
| `public_key` | string | ✅ | 客户端公钥 (base64, 32B) |

## 返回

```json
{
  "aap_token": "eyJhbGciOiJFZERTQSIs...",  // JWT, 1h 有效
  "expires_in": 3600
}
```

## 错误

| 错误码 | 含义 |
|--------|------|
| 401 | 签名错误 |
| 410 | challenge 过期（> 60s） |
| 409 | 重放（challenge 已被使用） |
