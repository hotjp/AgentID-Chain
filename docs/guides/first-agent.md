# 第一个 Agent

> 完整流程：注册 → 查询 → 升级 → 撤销

## 🎯 目标

通过 CLI 范式完整走一遍 Agent 生命周期。

## 📋 准备

确保服务已启动：

```bash
curl http://localhost:8080/live
```

## 🛠️ 步骤

### Step 1: AAP 鉴权（一次性）

AAP 准入协议是所有写操作的前置。

#### 1.1 生成密钥对

```bash
# 用 OpenSSL 生成
openssl genpkey -algorithm ed25519 -out priv.pem
openssl pkey -in priv.pem -pubout -out pub.pem

# 提取 base64
PUBKEY_B64=$(base64 -i pub.pem -w 0)
echo "$PUBKEY_B64"
```

或用 Go：

```go
package main

import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/base64"
    "fmt"
)

func main() {
    pub, priv, _ := ed25519.GenerateKey(rand.Reader)
    fmt.Println("Public Key:", base64.StdEncoding.EncodeToString(pub))
    fmt.Println("Private Key:", base64.StdEncoding.EncodeToString(priv))
}
```

#### 1.2 获取 Challenge

```bash
CHALLENGE_RESP=$(curl -s -X POST http://localhost:8080/v1/aap/challenge \
  -H "Content-Type: application/json" \
  -d "{\"public_key\":\"$PUBKEY_B64\"}")

echo "$CHALLENGE_RESP" | jq .
# {
#   "challenge": "dG9wIHNlY3JldA==",
#   "expires_in": 60
# }
```

#### 1.3 签名 Challenge

```bash
CHALLENGE=$(echo "$CHALLENGE_RESP" | jq -r .challenge)
CHALLENGE_BIN=$(echo "$CHALLENGE" | base64 -d)

# 签名（用 OpenSSL 较复杂，建议用 Go）
```

Go 示例：

```go
challenge, _ := base64.StdEncoding.DecodeString(challengeB64)
sig := ed25519.Sign(priv, challenge)
sigB64 := base64.StdEncoding.EncodeToString(sig)
```

#### 1.4 Verify 并获取 JWT

```bash
VERIFY_RESP=$(curl -s -X POST http://localhost:8080/v1/aap/verify \
  -H "Content-Type: application/json" \
  -d "{\"challenge\":\"$CHALLENGE\",\"signature\":\"$SIG_B64\",\"public_key\":\"$PUBKEY_B64\"}")

AAP_TOKEN=$(echo "$VERIFY_RESP" | jq -r .aap_token)
echo "AAP Token: $AAP_TOKEN"
```

### Step 2: 注册 Agent

```bash
REGISTER_RESP=$(curl -s -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $AAP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"owner":"alice","level":"test"}')

echo "$REGISTER_RESP" | jq .
# {
#   "agent_id": "0190a3b4-7c8d-7def-9abc-def012345678",
#   "owner": "alice",
#   "level": "test",
#   "status": "active",
#   "created_at": "2026-06-09T12:34:56Z"
# }

AGENT_ID=$(echo "$REGISTER_RESP" | jq -r .agent_id)
echo "Agent ID: $AGENT_ID"
```

或使用 CLI（更简单）：

```bash
go run ./cmd/agentid register --owner alice --level test
```

### Step 3: 查询 Agent

```bash
curl -s http://localhost:8080/v1/agents/$AGENT_ID \
  -H "Authorization: Bearer $AAP_TOKEN" | jq .
```

或 CLI：

```bash
go run ./cmd/agentid get $AGENT_ID
```

### Step 4: 升级 Agent

将 test 等级升级到 prod：

```bash
curl -s -X PATCH http://localhost:8080/v1/agents/$AGENT_ID \
  -H "Authorization: Bearer $AAP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"new_level":"prod"}' | jq .
```

或 CLI：

```bash
go run ./cmd/agentid upgrade $AGENT_ID --level prod
```

### Step 5: 撤销 Agent

```bash
curl -s -X DELETE http://localhost:8080/v1/agents/$AGENT_ID \
  -H "Authorization: Bearer $AAP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason":"compromised"}'
# → 204 No Content
```

或 CLI：

```bash
go run ./cmd/agentid revoke $AGENT_ID --reason compromised
```

## 📋 完整脚本

```bash
#!/bin/bash
set -e

# 1. AAP 准备
PUBKEY_B64=$(go run ./scripts/genkey.go)
CHALLENGE=$(curl -s -X POST http://localhost:8080/v1/aap/challenge \
  -H "Content-Type: application/json" \
  -d "{\"public_key\":\"$PUBKEY_B64\"}" | jq -r .challenge)
# ... 签名 ...
AAP_TOKEN=$(curl -s -X POST http://localhost:8080/v1/aap/verify ... | jq -r .aap_token)

# 2. 注册
AGENT_ID=$(curl -s -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer $AAP_TOKEN" \
  -d '{"owner":"alice","level":"test"}' | jq -r .agent_id)
echo "Registered: $AGENT_ID"

# 3. 升级
curl -s -X PATCH http://localhost:8080/v1/agents/$AGENT_ID \
  -H "Authorization: Bearer $AAP_TOKEN" \
  -d '{"new_level":"prod"}' | jq .

# 4. 撤销
curl -s -X DELETE http://localhost:8080/v1/agents/$AGENT_ID \
  -H "Authorization: Bearer $AAP_TOKEN"
echo "Revoked: $AGENT_ID"
```

## 🎯 接下来

- [用户旅程](journeys.md) — 5 个典型场景
- [API 完整参考](../api/openapi.md)
- [架构: 5 层分层](../architecture/5-layer.md)
