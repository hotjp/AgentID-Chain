---
name: a2a-negotiate
version: 2.0.1
role: system
description: A2A (Agent-to-Agent) 协商流程
---

# A2A 协商

当两个 agent 需要互相调用时，通过 A2A 协议建立信任。

## 场景

- Agent A 需要调用 Agent B 的服务
- 双方在 AgentID-Chain 注册过
- 互相信任关系未建立

## 协商流程

```
┌──────────┐                              ┌──────────┐
│ Agent A  │                              │ Agent B  │
└────┬─────┘                              └────┬─────┘
     │ 1. POST /v1/a2a/negotiate                │
     │    {target_agent_id, scopes: [...]}      │
     │ ───────────────────────────────────────> │
     │ <─────── {negotiation_id, challenges}    │
     │ 2. A 完成自己的 challenge                │
     │ 3. POST /v1/a2a/respond                  │
     │ ───────────────────────────────────────> │
     │ <─────── {status, b_challenge}           │
     │ 4. B 验证 A 签名, 完成自己的 challenge   │
     │ 5. 双向 verify                          │
     │ <─────── {session_token, expires_at}     │
     │ 6. 后续用 session_token 调用             │
```

## 步骤详解

### Step 1: 发起协商

```http
POST /v1/a2a/negotiate
Authorization: AAP <token>
Content-Type: application/json

{
  "target_agent_id": "0190a3b5-...",
  "scopes": ["read:profile", "write:interaction"],
  "ttl": "24h"
}
```

### Step 2: 完成 challenge

```http
POST /v1/a2a/respond
Content-Type: application/json

{
  "negotiation_id": "neg_abc",
  "challenge_response": "<base64 ed25519 sig>"
}
```

### Step 3: 双向验证后获取 session

```http
POST /v1/a2a/verify
Content-Type: application/json

{
  "negotiation_id": "neg_abc",
  "b_challenge_response": "<base64 ed25519 sig>"
}
```

响应：
```json
{
  "session_token": "a2a_sess_xyz",
  "expires_at": "2026-06-11T12:00:00Z",
  "granted_scopes": ["read:profile", "write:interaction"]
}
```

### Step 4: 调用

```http
POST https://peer.example.com/api/v1/interact
X-A2A-Session: a2a_sess_xyz
...
```

## scope 列表

| Scope | 含义 |
|-------|------|
| `read:profile` | 读取对方 profile |
| `read:audit` | 读取对方审计日志 |
| `write:interaction` | 写入交互记录 |
| `delegate:*` | 委派对方代表自己 |

## 错误处理

| 错误 | 含义 | 处理 |
|------|------|------|
| 403 NEGOTIATION_DENIED | 对方拒绝 | 通知用户，等待手动确认 |
| 408 NEGOTIATION_TIMEOUT | 60s 内未完成 | 重新发起 |
| 409 SCOPE_NOT_GRANTED | scope 不足 | 调整 scopes 重试 |
| 410 SESSION_EXPIRED | session 过期 | 重新协商 |

## 最佳实践

- 最小权限：只请求必要 scope
- 短期 session：默认 24h，过期重协商
- 记录协商：所有协商写 audit log
- 撤销流程：发现异常立即撤销 session

## 撤销

```http
DELETE /v1/a2a/session/{session_token}
```

效果：
- 立即生效（Redis pub/sub）
- 双方都会收到 webhook 通知
- 已发出的在途请求允许完成
