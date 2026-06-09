# Agent Skills

> 预制的 Agent 能力 — LLM 可直接调用

## 📋 索引

| Skill | 描述 | 范式 |
|-------|------|------|
| [register-agent](register-agent/) | 注册新 Agent | MCP + Function Calling |
| [query-agent](query-agent/) | 查询 Agent 详情 | MCP + Function Calling |
| [upgrade-agent](upgrade-agent/) | 升级 Agent 等级 | MCP + Function Calling |
| [revoke-agent](revoke-agent/) | 撤销 Agent | MCP + Function Calling |
| [batch-register](batch-register/) | 批量注册 | MCP + Function Calling |
| [aap-verify](aap-verify/) | AAP 协议验证 | MCP + Function Calling |
| [a2a-negotiate](a2a-negotiate/) | A2A Token 协商 | MCP + Function Calling |
| [list-agents](list-agents/) | 列出 Agent | MCP + Function Calling |

## 🚀 快速使用

### Claude Desktop / Cline

```json
{
  "mcpServers": {
    "agentid-chain": {
      "url": "http://localhost:8080/mcp/v1/rpc",
      "headers": { "Authorization": "Bearer <aap-jwt>" }
    }
  }
}
```

### OpenAI Function Calling

```python
import openai

tools = [
    {
        "type": "function",
        "function": {
            "name": "register_agent",
            "description": "注册新 Agent",
            "parameters": {
                "type": "object",
                "properties": {
                    "owner": {"type": "string"},
                    "level": {"type": "string", "enum": ["test", "prod"]}
                },
                "required": ["owner", "level"]
            }
        }
    }
]

response = openai.ChatCompletion.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "为 alice 注册一个 test agent"}],
    tools=tools
)
```

## 📚 详细说明

每个 Skill 目录包含：
- `SKILL.md` — 详细文档
- `schema.json` — JSON Schema 定义
- `examples/` — 多个使用示例
- `tests/` — 验证脚本

## 🛠️ 验证

```bash
./scripts/test-skills.sh
```

## 📂 目录结构

```
skills/
├── README.md                  # 本文件
├── register-agent/
│   ├── SKILL.md
│   ├── schema.json
│   ├── examples/
│   └── tests/
├── query-agent/
├── upgrade-agent/
├── revoke-agent/
├── batch-register/
├── aap-verify/
├── a2a-negotiate/
└── list-agents/
```
