# MCP 协议 (Model Context Protocol)

> LLM 工具调用协议 — JSON-RPC 2.0

## 🎯 目标

让 LLM（Claude / GPT / Qwen 等）能够通过标准化接口：
- 注册新 Agent
- 查询 Agent 详情
- 升级 / 撤销 Agent
- 验证 AAP 签名

## 📡 端点

`POST /mcp/v1/rpc`

## 🔐 鉴权

Bearer Token（来自 AAP）：

```
Authorization: Bearer <aap-jwt>
```

## 🛠️ 工具清单

| 工具 | 参数 | 描述 |
|------|------|------|
| `register_agent` | owner, level, metadata? | 注册新 Agent |
| `get_agent` | uuid | 查询 Agent 详情 |
| `upgrade_agent` | uuid, new_level | 升级等级 |
| `revoke_agent` | uuid, reason | 撤销 Agent |
| `verify_aap` | challenge, signature, public_key | 验证 AAP 签名 |
| `list_agents` | owner, level?, status?, limit?, cursor? | 列出 Agent |

## 📋 请求格式

### register_agent

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "register_agent",
    "arguments": {
      "owner": "alice",
      "level": "test",
      "metadata": {
        "team": "infra"
      }
    }
  }
}
```

### 响应

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"agent_id\":\"0190a3b4-...\",\"owner\":\"alice\",\"level\":\"test\",\"status\":\"active\"}"
      }
    ]
  }
}
```

### get_agent

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_agent",
    "arguments": { "uuid": "0190a3b4-7c8d-7def-9abc-def012345678" }
  }
}
```

### list_agents

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "list_agents",
    "arguments": {
      "owner": "alice",
      "level": "test",
      "limit": 50,
      "cursor": null
    }
  }
}
```

### 响应（带分页）

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"agents\":[...],\"next_cursor\":\"eyJ...\"}"
      }
    ]
  }
}
```

## ❌ 错误格式

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request",
    "data": {
      "type": "https://agentid-chain/errors/invalid-args",
      "title": "Invalid Arguments",
      "status": 400,
      "detail": "owner is required"
    }
  }
}
```

### 错误码

| 码 | 含义 |
|----|------|
| -32700 | Parse error |
| -32600 | Invalid Request |
| -32601 | Method not found |
| -32602 | Invalid params |
| -32603 | Internal error |
| -32001 | AAP failed（鉴权） |
| -32002 | Not found |
| -32003 | Conflict (UUID 冲突) |

## 🛠️ 客户端示例

### Claude Desktop 配置

`~/.config/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "agentid-chain": {
      "url": "http://localhost:8080/mcp/v1/rpc",
      "transport": "http",
      "headers": {
        "Authorization": "Bearer <aap-jwt>"
      }
    }
  }
}
```

### Cline (VS Code) 配置

`.cline/mcp_settings.json`:
```json
{
  "mcpServers": {
    "agentid-chain": {
      "url": "http://localhost:8080/mcp/v1/rpc",
      "headers": {
        "Authorization": "Bearer <aap-jwt>"
      }
    }
  }
}
```

### Python (mcp 库)

```python
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

async with streamablehttp_client("http://localhost:8080/mcp/v1/rpc",
                                  headers={"Authorization": f"Bearer {token}"}) as (r, w):
    async with ClientSession(r, w) as session:
        await session.initialize()
        result = await session.call_tool("register_agent", {
            "owner": "alice",
            "level": "test"
        })
        print(result)
```

## 📊 监控

| 指标 | 标签 | 描述 |
|------|------|------|
| `mcp_calls_total` | tool, result | 工具调用次数 |
| `mcp_call_duration_seconds` | tool | 工具调用延迟 |
| `mcp_active_sessions` | - | 当前活跃 MCP 会话 |

## 🔒 安全

- 必须使用 AAP JWT（不接受长期 API Key）
- 工具调用记录到 audit_log（含 caller_ip, user_agent, tool, args 摘要）
- 限流：每 Agent 每分钟 60 次工具调用
- 不接受 `level=internal`（仅系统服务可注册 internal agent）

## 📚 相关

- [AAP 协议](aap.md)
- [A2A 协议](a2a.md)
- [MCP 官方规范](https://modelcontextprotocol.io/)
