---
name: aap-handshake
version: 2.0.1
role: system
description: AAP (Agent-to-Agent Protocol) 完整握手流程
---

# AAP 协议握手

当需要调用 AgentID-Chain 写接口（register/upgrade/revoke）时，引导用户完成 AAP 鉴权。

## 协议流程

```
┌──────────┐                              ┌──────────────┐
│  Client  │                              │  AgentID-Chain │
└────┬─────┘                              └──────┬───────┘
     │ 1. POST /v1/aap/challenge                  │
     │ ──────────────────────────────────────────> │
     │ <──────────────────────────  {challenge, ts, nonce} │
     │ 2. 用 ed25519 私钥签名 challenge            │
     │ 3. POST /v1/aap/verify                     │
     │ ──────────────────────────────────────────> │
     │ <──────────────────────────  {token, ttl}   │
     │ 4. 后续请求带 AAP-Token header              │
     │    Authorization: AAP <token>              │
```

## 步骤详解

### Step 1: 获取 challenge

```http
POST /v1/aap/challenge
Content-Type: application/json

{
  "public_key": "<base64 ed25519 公钥>"
}
```

响应：
```json
{
  "challenge": "0190a3b4-7c8d-7def-9abc-def012345678",
  "timestamp": 1718000000,
  "nonce": "abc123...",
  "expires_in": 300
}
```

### Step 2: 签名 challenge

```python
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
import base64, json

priv = Ed25519PrivateKey.generate()
challenge = "0190a3b4-7c8d-7def-9abc-def012345678"
ts = 1718000000
nonce = "abc123..."

msg = f"{challenge}:{ts}:{nonce}".encode()
sig = priv.sign(msg)
```

### Step 3: 提交 verify

```http
POST /v1/aap/verify
Content-Type: application/json

{
  "public_key": "<base64>",
  "challenge": "0190a3b4-7c8d-7def-9abc-def012345678",
  "timestamp": 1718000000,
  "nonce": "abc123...",
  "signature": "<base64 ed25519 sig>"
}
```

响应：
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 3600
}
```

### Step 4: 使用 token

```http
POST /v1/agents
Authorization: AAP eyJhbGciOiJIUzI1NiIs...
Content-Type: application/json

{"owner": "alice", "level": "test"}
```

## 安全要点

- 私钥永不上传
- challenge 默认 5 分钟过期
- token 默认 1 小时过期
- 每次请求带独立 nonce 防重放
- 客户端时间偏差需 < 5 分钟

## 错误处理

| 错误 | 含义 | 处理 |
|------|------|------|
| 401 INVALID_SIGNATURE | 签名错误 | 检查密钥对 / 消息格式 |
| 401 EXPIRED_CHALLENGE | challenge 过期 | 重新获取 |
| 401 REPLAY_DETECTED | nonce 重复使用 | 重新获取 challenge |
| 429 RATE_LIMITED | 1h 内尝试过多 | 等待 1h 或换 IP |
