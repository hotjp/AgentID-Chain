# OpenAPI 规范

> AgentID-Chain 的 HTTP/REST API 入口

## 📋 端点总览

| 路径 | 方法 | 鉴权 | 描述 |
|------|------|------|------|
| `/healthz` | GET | 无 | 健康检查 |
| `/live` | GET | 无 | 存活探针 |
| `/ready` | GET | 无 | 就绪探针 |
| `/v1/agents` | POST | AAP | 注册 Agent |
| `/v1/agents/{uuid}` | GET | AAP / A2A | 查询 Agent |
| `/v1/agents/{uuid}` | PATCH | AAP | 升级 Agent |
| `/v1/agents/{uuid}` | DELETE | AAP | 撤销 Agent |
| `/v1/agents:batch` | POST | AAP | 批量注册 |
| `/v1/aap/challenge` | POST | 无 | 获取 Challenge |
| `/v1/aap/verify` | POST | 无 | 验证签名 |
| `/v1/a2a/negotiate` | POST | AAP | 颁发 A2A Token |
| `/v1/a2a/revoke` | POST | AAP | 撤销 A2A Token |
| `/mcp/v1/rpc` | POST | Bearer | MCP JSON-RPC 入口 |

## 📦 完整 OpenAPI 规范

完整规范见 [openapi.yaml](openapi.yaml)（OpenAPI 3.0）。

## 🔍 关键响应

### 注册成功 (201)

```json
{
  "agent_id": "0190a3b4-7c8d-7def-9abc-def012345678",
  "owner": "alice",
  "level": "test",
  "status": "active",
  "created_at": "2026-06-09T12:34:56Z",
  "metadata": {},
  "chain_tx_hash": "0xabc123..."
}
```

### AAP 挑战 (200)

```json
{
  "challenge": "base64_encoded_32_bytes",
  "expires_in": 60
}
```

### 错误格式 (RFC 7807)

```json
{
  "type": "https://agentid-chain/errors/aap-failed",
  "title": "AAP Verification Failed",
  "status": 401,
  "detail": "signature does not match challenge",
  "instance": "/v1/agents",
  "trace_id": "abc123..."
}
```

## 🔐 鉴权流程

```
1. POST /v1/aap/challenge
   → { challenge, expires_in }

2. (客户端用私钥签名 challenge)
   → signature

3. POST /v1/aap/verify
   Body: { challenge, signature, public_key }
   → { aap_token (JWT, 1h) }

4. POST /v1/agents
   Header: Authorization: Bearer <aap_token>
   Body: { owner, level, ... }
   → 201 Created
```

## 📚 子文档

- [AAP 协议详解](aap.md)
- [A2A 协议详解](a2a.md)
- [MCP 协议详解](mcp.md)

## 🛠️ 工具链

### 生成客户端 SDK

```bash
# TypeScript
npx @openapitools/openapi-generator-cli generate \
  -i docs/api/openapi.yaml \
  -g typescript-fetch \
  -o sdks/typescript/

# Go
oapi-codegen -config oapi-codegen.yaml docs/api/openapi.yaml
```

### 验证规范

```bash
npx @redocly/cli lint docs/api/openapi.yaml
```
