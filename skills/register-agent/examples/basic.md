# register-agent: 基础示例

## 场景 1: 注册测试 agent

**用户输入**：
```
为 alice 注册一个 test 等级的 agent
```

**LLM 推理**：
```json
{
  "tool_call": {
    "name": "register_agent",
    "arguments": {
      "owner": "alice",
      "level": "test"
    }
  }
}
```

**MCP 调用**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "register_agent",
    "arguments": {"owner": "alice", "level": "test"}
  }
}
```

**响应**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{
      "type": "text",
      "text": "{\"agent_id\":\"0190a3b4-7c8d-7def-9abc-def012345678\",\"owner\":\"alice\",\"level\":\"test\",\"status\":\"active\",\"created_at\":\"2026-06-09T12:34:56Z\"}"
    }]
  }
}
```

## 场景 2: 注册带元数据

**用户输入**：
```
为 team-b 注册一个 prod 等级的 agent，标记为 payment-service
```

**LLM 推理**：
```json
{
  "tool_call": {
    "name": "register_agent",
    "arguments": {
      "owner": "team-b",
      "level": "prod",
      "metadata": {
        "service": "payment",
        "env": "prod",
        "region": "us-west"
      }
    }
  }
}
```

## 场景 3: 错误处理 — 缺少必填参数

**用户输入**：
```
帮我注册一个 agent
```

**LLM 推理**：
```json
{
  "tool_call": {
    "name": "register_agent",
    "arguments": {}
  }
}
```

**错误响应**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": {
      "type": "https://agentid-chain/errors/invalid-args",
      "title": "Invalid Arguments",
      "status": 400,
      "detail": "owner and level are required"
    }
  }
}
```

**LLM 应继续询问用户**：「请问 owner 是谁？level 是什么？」

## 场景 4: AAP 鉴权失败

**响应**：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32001,
    "message": "AAP failed",
    "data": {
      "type": "https://agentid-chain/errors/aap-failed",
      "status": 401,
      "detail": "AAP token expired or invalid"
    }
  }
}
```

**LLM 应提示用户**：「请先完成 AAP 鉴权：[链接]」
