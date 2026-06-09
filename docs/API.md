# API Reference

> AgentID-Chain 对外 API 入口文档

AgentID-Chain 提供 4 种接入范式：

| 范式 | 协议 | 适用场景 | 详细文档 |
|------|------|---------|---------|
| **CLI** | cobra commands | 运维 / 脚本 | [docs/api/cli.md](api/cli.md) |
| **MCP** | JSON-RPC 2.0 (stdio/HTTP) | Claude / 智能体 | [docs/api/mcp.md](api/mcp.md) |
| **A2A** | HTTPS + ED25519 signature | Agent-to-Agent | [docs/api/a2a.md](api/a2a.md) |
| **Prompt** | 自然语言 → Tool Call | LLM 客户端 | [docs/api/openapi.md](api/openapi.md) |

## 🚀 快速开始

### CLI

```bash
agentid register --owner alice --level test
agentid get <agent_id>
agentid list --owner alice
agentid upgrade <agent_id> --level prod
agentid revoke <agent_id>
```

### MCP（Claude Desktop）

```json
{
  "mcpServers": {
    "agentid": {
      "command": "agentid",
      "args": ["mcp", "serve"]
    }
  }
}
```

### A2A

```bash
# 1. 生成密钥对
agentid aap keygen > aap.key

# 2. 注册
curl -X POST https://api.agentid-chain.example/v1/agents \
  -H "AAP-Signature: ed25519=..." \
  -d '{"owner":"alice","level":"test"}'
```

### Prompt（自然语言）

详见 [prompts/](prompts/) 目录的 13 个模板。

## 📊 核心资源

| 资源 | 端点 | 描述 |
|------|------|------|
| Agent | `POST /v1/agents` | 注册新 agent |
| Agent | `GET /v1/agents/{id}` | 查询详情 |
| Agent | `PATCH /v1/agents/{id}` | 升级 / 吊销 |
| Agent | `GET /v1/agents` | 列表（分页） |
| Batch | `POST /v1/agents/batch` | 批量注册 |
| Health | `GET /live`, `/healthz` | 健康检查 |
| Metrics | `GET /metrics` | Prometheus |

## 🔐 鉴权

所有写操作必带 AAP 签名（详见 [docs/api/aap.md](api/aap.md)）：

```
POST /v1/agents
AAP-Timestamp: 1718000000
AAP-Signature: ed25519=<base64>
AAP-Public-Key: <base64>
AAP-Nonce: <uuid>

{"owner":"alice","level":"test"}
```

## 📐 错误码

| HTTP | 错误 | 含义 |
|------|------|------|
| 400 | INVALID_REQUEST | 参数错误 |
| 401 | UNAUTHORIZED | AAP 校验失败 |
| 403 | FORBIDDEN | 状态机拒绝 |
| 404 | NOT_FOUND | agent 不存在 |
| 409 | CONFLICT | UUID 冲突 / 状态非法 |
| 429 | RATE_LIMITED | 触发限流 |
| 500 | INTERNAL | 服务异常 |
| 503 | BACKEND_DOWN | PG / Redis / Chain 全不可用 |

## 📚 OpenAPI 规范

完整 schema：[docs/api/openapi.yaml](api/openapi.yaml)

## 🧪 测试

```bash
# 单元
go test ./internal/... -short

# 集成（需 docker）
go test ./test/integration/...

# 契约
go test ./internal/mcp/... ./internal/gateway/handler/...
```

## 📚 延伸阅读

- [AAP 协议](api/aap.md) — Agent-to-Agent Protocol
- [A2A 协议](api/a2a.md) — Agent-to-Agent 远程调用
- [MCP 协议](api/mcp.md) — Model Context Protocol
- [CLI 命令](api/cli.md) — 命令行参考
- [OpenAPI](api/openapi.md) — REST 规范
