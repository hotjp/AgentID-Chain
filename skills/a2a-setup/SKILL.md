# a2a-setup

> 配置 Agent-to-Agent (A2A) 远程调用

## 📋 描述

为指定 agent 配置 A2A 远端调用凭证（mTLS 证书 / JWT）。

**适用场景**：

- Agent 间服务调用
- 跨集群身份验证
- 多 agent 工作流

## 🔧 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `agent_id` | string | ✅ | Agent UUID |
| `peer_url` | string | ✅ | 对端 base URL |
| `auth_mode` | string | | `mtls` / `jwt` (默认 mtls) |
| `ttl` | string | | 凭证有效期（默认 24h） |

## 📤 返回

```json
{
  "credential_path": "/etc/agentid/credentials/0190a3b4.crt",
  "expires_at": "2026-06-10T12:34:56Z",
  "peer_url": "https://peer.example.com"
}
```

## 🛠️ 实现

```python
def a2a_setup(agent_id, peer_url, auth_mode="mtls", ttl="24h"):
    return call_tool("a2a_setup", {
        "agent_id": agent_id,
        "peer_url": peer_url,
        "auth_mode": auth_mode,
        "ttl": ttl
    })
```

## 📚 安全提示

- mTLS 模式需双向 CA 信任
- JWT 模式 token 不超 24h
- 凭证文件权限 600
