# 协议概览

> AgentID-Chain 实现的 4 种协议：CLI / MCP / A2A / Prompt

## 📋 协议对照表

| 协议 | 传输 | 鉴权 | 适用 |
|------|------|------|------|
| **CLI** | 本地进程 | 配置文件 / mTLS | 运维、脚本 |
| **MCP** | JSON-RPC 2.0 | Bearer Token | LLM 工具调用 |
| **A2A** | HTTP/JSON | EdDSA JWT | Agent 间互信 |
| **Prompt** | 终端 / HTTP | 用户意图 | 人类对话入口 |

## 🖥️ CLI

### 框架
[cobra](https://github.com/spf13/cobra) + [viper](https://github.com/spf13/viper)

### 命令清单

```bash
agentid register --owner alice --level test --backend local
agentid get <uuid>
agentid upgrade <uuid> --level prod
agentid revoke <uuid> --reason compromised
agentid batch --file agents.yaml
agentid verify-aap --challenge <base64> --signature <hex>
agentid prompt "为 alice 注册一个测试 agent"
```

### 配置文件
`~/.agentid/config.yaml`：
```yaml
endpoint: http://localhost:8080
api_key: ${AGENTID_API_KEY}
output: json  # json | yaml | table
```

## 🤖 MCP (Model Context Protocol)

### 端点
`POST /mcp/v1/rpc`

### 工具列表

| 工具 | 参数 | 描述 |
|------|------|------|
| `register_agent` | owner, level, metadata? | 注册新 Agent |
| `get_agent` | uuid | 查询 Agent 详情 |
| `upgrade_agent` | uuid, new_level | 升级等级 |
| `revoke_agent` | uuid, reason | 撤销 Agent |
| `verify_aap` | challenge, signature | 验证 AAP 签名 |
| `list_agents` | owner, level?, status? | 列出 Agent |

### 示例

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "register_agent",
    "arguments": {
      "owner": "alice",
      "level": "test"
    }
  }
}
```

详见 [api/mcp.md](../api/mcp.md)

## 🔗 A2A (Agent-to-Agent)

### 流程

```
Agent A                    AgentID-Chain                   Agent B
   │                              │                            │
   │──── POST /a2a/negotiate ────>│                            │
   │     { agent_id, scope }      │                            │
   │                              │                            │
   │<─── 200 OK ──────────────────│                            │
   │     { token, expires_in }    │                            │
   │                              │                            │
   │                                                            │
   │  (sign request with private key from token)                │
   │                                                            │
   │──── POST /api/agents/... ─────────────────────────────────>│
   │     Header: X-A2A-Token: <jwt>                             │
   │     Header: X-A2A-Signature: <ed25519>                     │
   │                                                            │
   │<─── 200 OK ────────────────────────────────────────────────│
   │     (verified by B's local public key)                     │
```

### Token 格式
- **签名算法**：EdDSA (Ed25519)
- **生命周期**：默认 1h，可续期
- **撤销**：Redis Set，TTL = token 剩余有效期

详见 [api/a2a.md](../api/a2a.md)

## 💬 Prompt

### 工作原理

```
User Input (NL)
    ↓
Intent Parser (LLM / 规则引擎)
    ↓
Structured Command { action, args }
    ↓
CLI / MCP / A2A 路径之一
```

### 支持的意图

- **注册**：「为 X 注册一个 Y 等级的 agent」
- **查询**：「查一下 X 的 agent 列表」
- **升级**：「把 X 的 agent 升级到 Y 等级」
- **撤销**：「撤销 X 的 agent，因为 Y」

### 配置

```yaml
prompt:
  parser: rule  # rule | llm
  rules_file: configs/prompt_rules.yaml
  # 当 parser=llm 时
  llm_endpoint: http://localhost:11434
  llm_model: qwen2.5:7b
```

## 🔄 协议间关系

```
┌─────────────────────────────────────────────────────────┐
│                      User / Agent                        │
└──────────┬──────────────┬──────────────┬────────────────┘
           │              │              │
      ┌────▼────┐    ┌────▼────┐    ┌────▼────┐
      │   CLI   │    │   MCP   │    │  A2A    │
      └────┬────┘    └────┬────┘    └────┬────┘
           │              │              │
           │         ┌────▼────┐         │
           │         │ Prompt  │         │
           │         └────┬────┘         │
           │              │              │
           └──────────────┼──────────────┘
                          ↓
                 ┌────────────────┐
                 │  L5 Gateway    │
                 └────────────────┘
```

所有协议最终都通过 L5 Gateway 入口 → L3 Authz → L4 Service 路径处理。
