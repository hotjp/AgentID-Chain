# A2A 协议 (Agent-to-Agent)

> 代理间互信协议 — EdDSA JWT + Redis 撤销

## 🎯 目标

让 Agent A 调 Agent B 的服务时：
- B 知道是 A 在调用（**身份**）
- 调用是不可抵赖的（**签名**）
- A 的权限可以被撤销（**撤销**）
- 不需要共享密钥（**非对称**）

## 🔄 流程

```
Agent A                                AgentID-Chain                          Agent B
  │                                          │                                     │
  │  (已有 AAP JWT, 先用 AAP 鉴权)            │                                     │
  │                                          │                                     │
  │──── POST /v1/a2a/negotiate ────────────>│                                     │
  │     Header: Bearer <aap_token>           │                                     │
  │     Body: { agent_id, scope }            │                                     │
  │                                          │                                     │
  │                                          │ (生成 Ed25519 临时密钥对,            │
  │                                          │  公钥返回给 A)                       │
  │                                          │                                     │
  │<────── 200 OK ──────────────────────────│                                     │
  │     { token, expires_in: 3600,          │                                     │
  │       public_key, private_key_encrypted }│                                     │
  │                                          │                                     │
  │  (A 解密私钥, 之后每个请求用此私钥签名)    │                                     │
  │                                                                                 │
  │──── POST http://agent-b.example/api ─────────────────────────────────────────>│
  │     Header: X-A2A-Token: <jwt>                                                  │
  │     Header: X-A2A-Signature: <ed25519>                                          │
  │     Header: X-Agent-Id: <uuid>                                                  │
  │     Body: { ... }                                                               │
  │                                                                                 │
  │                                          │   (B 验证签名,                       │
  │                                          │    检查 token 是否在 Redis 撤销集)   │
  │                                                                                 │
  │<────── 200 OK ────────────────────────────────────────────────────────────────│
  │     { result }                                                                   │
```

## 📦 Token 结构

### Header

```json
{
  "alg": "EdDSA",
  "typ": "A2A",
  "kid": "a2a-2026-06-09T12:34:56Z"
}
```

### Payload

```json
{
  "sub": "0190a3b4-7c8d-7def-9abc-def012345678",
  "owner": "alice",
  "scope": ["agents:read", "agents:write"],
  "iss": "agentid-chain",
  "aud": "agent-b",
  "iat": 1749462896,
  "exp": 1749466496,
  "jti": "uuid-v7-unique"
}
```

### Signature

Ed25519 签名（base64 编码）覆盖 `header.payload`。

## 🔐 私钥分发

**问题**：A 需要拿到 A2A 私钥才能签名。

**方案**：使用 AES-256-GCM 加密私钥。
- 加密密钥：AAP 颁发的 JWT（从 AAP verify 时使用 HKDF 派生）
- A 解密 JWT → 派生 KEK → 解密 A2A 私钥 → 缓存到内存

```go
// 派生 KEK
kek := hkdf.New(sha256.New, aapToken, []byte("a2a-kek"), []byte("v1"))
// 解密 A2A 私钥
privKey, _ := gcm.Open(nil, nonce, ciphertext, kek)
```

## 🚫 撤销机制

### 写入

```bash
POST /v1/a2a/revoke
Header: Bearer <aap_token>
Body: { "jti": "<token-id>" }
```

### 存储

Redis Set: `a2a:revoked` 包含所有被撤销的 `jti`，TTL = token 剩余有效期。

### 验证

B 在验证签名时：
1. 检查 `jti` 是否在 `a2a:revoked`（O(1) SISMEMBER）
2. 检查 `exp` 是否过期
3. 验签

## 📊 监控指标

| 指标 | 描述 |
|------|------|
| `a2a_token_issued_total` | Token 颁发次数 |
| `a2a_token_revoked_total` | Token 撤销次数 |
| `a2a_token_active` | 当前活跃 Token（估算） |
| `a2a_token_verify_total{result}` | 验证结果 |
| `a2a_negotiate_duration_seconds` | 协商延迟 |
| `a2a_session_duration_seconds` | 会话使用时长分布 |

## 🛠️ 客户端示例

```go
// 1. 协商
resp, _ := http.Post("http://localhost:8080/v1/a2a/negotiate",
    "application/json",
    bytes.NewBufferString(`{"agent_id":"...","scope":["read"]}`))
resp.Header.Set("Authorization", "Bearer "+aapJWT)
var tok struct {
    Token string `json:"token"`
    PublicKey string `json:"public_key"`
    PrivateKeyEnc string `json:"private_key_encrypted"`
    ExpiresIn int `json:"expires_in"`
}
json.NewDecoder(resp.Body).Decode(&tok)

// 2. 签名并调用 B
priv, _ := decryptPrivKey(tok.PrivateKeyEnc, aapJWT)
req, _ := http.NewRequest("POST", "http://agent-b/api", body)
req.Header.Set("X-A2A-Token", tok.Token)
req.Header.Set("X-Agent-Id", agentID)

body := req.Body
sig := ed25519.Sign(priv, mustReadAll(body))
req.Header.Set("X-A2A-Signature", base64.StdEncoding.EncodeToString(sig))
http.DefaultClient.Do(req)
```

## 📚 相关

- [AAP 协议](aap.md) — 准入协议（A2A 依赖）
- [MCP 协议](mcp.md) — LLM 工具调用
- [架构: 协议概览](../architecture/protocols.md)
