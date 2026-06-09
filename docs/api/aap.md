# AAP 协议 (Agent Admission Protocol)

> Challenge-Response 准入协议 — 防止重放与伪造

## 🎯 目标

- 验证 Agent 持有有效私钥（防止冒名）
- 防止重放攻击（每请求使用一次性 challenge）
- 颁发短时 JWT（默认 1h）供后续 API 调用

## 🔄 流程

```
Client                                AgentID-Chain
  │                                         │
  │──── POST /v1/aap/challenge ────────────>│
  │     { public_key }                      │
  │                                         │
  │                                         │ (生成 32B 随机数,
  │                                         │  存 Redis: nonce:<key> = challenge,
  │                                         │  TTL = 60s)
  │                                         │
  │<────── 200 OK ──────────────────────────│
  │     { challenge, expires_in: 60 }       │
  │                                         │
  │  (客户端用私钥对 challenge 签名)         │
  │                                         │
  │──── POST /v1/aap/verify ───────────────>│
  │     { challenge, signature, public_key }│
  │                                         │
  │                                         │ (验证 EdDSA,
  │                                         │  删除 nonce (一次性),
  │                                         │  颁发 JWT)
  │                                         │
  │<────── 200 OK ──────────────────────────│
  │     { aap_token (JWT, 1h), expires_in } │
  │                                         │
  │──── POST /v1/agents ───────────────────>│
  │     Header: Authorization: Bearer <jwt> │
  │     Body: { owner, level }              │
  │                                         │
  │<────── 201 Created ─────────────────────│
  │     { agent_id, ... }                   │
```

## 🔐 密码学

- **算法**：EdDSA Ed25519（见 [ADR-0002](../architecture/adr/0002-aap-eddsa.md)）
- **Challenge**：32 字节加密随机数
- **公钥**：32 字节
- **签名**：64 字节
- **编码**：全部使用 base64（标准编码，with padding）

## 📡 协议消息

### Challenge Request

```http
POST /v1/aap/challenge
Content-Type: application/json

{
  "public_key": "kPqJWG...base64...32B"
}
```

### Challenge Response

```json
{
  "challenge": "dG9wIHNlY3JldA==",
  "expires_in": 60
}
```

### Verify Request

```json
{
  "challenge": "dG9wIHNlY3JldA==",
  "signature": "8JKj...base64...64B",
  "public_key": "kPqJWG...base64...32B"
}
```

### Verify Response

```json
{
  "aap_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_in": 3600
}
```

## 🛡️ 安全保证

| 威胁 | 缓解 |
|------|------|
| **重放攻击** | challenge 一次性（Redis SETNX + DEL after verify） |
| **中间人** | HTTPS（生产强制）+ HSTS |
| **签名伪造** | EdDSA 128-bit 安全性 |
| **公开 challenge 泄露** | 60s TTL + 一次性 |
| **私钥泄露** | 颁发 JWT 时可绑定 `agent_id`；泄露后撤销 |

## 📊 性能

- **Challenge 生成**：~1ms（Redis SETNX）
- **Verify**：~53μs（Ed25519 Verify）
- **JWT 颁发**：~100μs（HS256 签名）

见 [aap-benchmark.md](../perf/aap-benchmark.md)

## 🛠️ 客户端示例

### Go

```go
import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/base64"
    "net/http"
    "bytes"
    "encoding/json"
)

// 1. 生成密钥对
pub, priv, _ := ed25519.GenerateKey(rand.Reader)
pubB64 := base64.StdEncoding.EncodeToString(pub)

// 2. 获取 challenge
resp, _ := http.Post("http://localhost:8080/v1/aap/challenge",
    "application/json",
    bytes.NewBufferString(fmt.Sprintf(`{"public_key":"%s"}`, pubB64)))
var ch struct{ Challenge string `json:"challenge"`; ExpiresIn int `json:"expires_in"` }
json.NewDecoder(resp.Body).Decode(&ch)

// 3. 签名
chBytes, _ := base64.StdEncoding.DecodeString(ch.Challenge)
sig := ed25519.Sign(priv, chBytes)
sigB64 := base64.StdEncoding.EncodeToString(sig)

// 4. Verify
verifyBody := fmt.Sprintf(`{"challenge":"%s","signature":"%s","public_key":"%s"}`,
    ch.Challenge, sigB64, pubB64)
resp2, _ := http.Post("http://localhost:8080/v1/aap/verify",
    "application/json",
    bytes.NewBufferString(verifyBody))
```

## 📈 监控指标

| 指标 | 标签 | 描述 |
|------|------|------|
| `aap_challenge_total` | result=success/failure | Challenge 颁发次数 |
| `aap_verify_total` | result=success/failure, reason | Verify 次数 |
| `aap_verify_duration_seconds` | - | Verify 延迟 |
| `aap_nonce_replays_total` | - | 重放尝试次数（告警） |
| `aap_active_sessions` | - | 当前活跃 JWT 数量（估算） |

## 📚 相关

- [A2A 协议](a2a.md) — Agent 间互信协议
- [MCP 协议](mcp.md) — LLM 工具调用
- [ADR-0002: AAP 使用 EdDSA](../architecture/adr/0002-aap-eddsa.md)
