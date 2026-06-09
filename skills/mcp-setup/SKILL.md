# mcp-setup

> 配置 Claude Desktop / Cursor 等 MCP 客户端连接 AgentID-Chain

## 📋 描述

生成 MCP 客户端配置文件（含 stdio 与 HTTP 两种模式）。

**适用场景**：

- Claude Desktop 集成
- Cursor IDE 集成
- Cline / Continue.dev 等

## 🔧 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `client` | string | ✅ | `claude-desktop` / `cursor` / `cline` / `continue` |
| `transport` | string | | `stdio` (默认) / `http` |
| `http_url` | string | | transport=http 时必填 |
| `aap_key` | string | | Agent AAP 私钥路径 |

## 📤 返回

```json
{
  "config_path": "/Users/.../Library/Application Support/Claude/claude_desktop_config.json",
  "config": {
    "mcpServers": {
      "agentid": {
        "command": "agentid",
        "args": ["mcp", "serve"]
      }
    }
  }
}
```

## 🛠️ 实现

```python
def mcp_setup(client, transport="stdio", http_url=None, aap_key=None):
    args = {"client": client, "transport": transport}
    if http_url: args["http_url"] = http_url
    if aap_key: args["aap_key"] = aap_key
    return call_tool("mcp_setup", args)
```

## 📚 安全提示

- AAP 私钥必设文件权限 600
- 优先用 1Password / Keychain 注入
- HTTP 模式需 mTLS
