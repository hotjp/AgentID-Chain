# register-agent

> 注册新 Agent 到 AgentID-Chain

## 📋 描述

为指定的 owner 注册一个新 Agent，分配唯一 UUID。

**适用场景**：
- 初始化新服务 / 脚本
- 为微服务分配身份
- LLM 工具调用

## 🔧 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `owner` | string | ✅ | Agent 所有者（用户名 / 团队名 / 服务名） |
| `level` | string | ✅ | 等级：`test` / `prod` / `internal` |
| `metadata` | object | | 自定义元数据（键值对） |

## 📤 返回

```json
{
  "agent_id": "0190a3b4-7c8d-7def-9abc-def012345678",
  "owner": "alice",
  "level": "test",
  "status": "active",
  "created_at": "2026-06-09T12:34:56Z",
  "metadata": {},
  "chain_tx_hash": "0xabc..."
}
```

## 🛡️ 权限

- 需要 AAP 鉴权
- 注册 `level=internal` 需要 system 权限

## ⚠️ 错误

| 错误码 | 含义 |
|--------|------|
| 400 | 参数错误 |
| 401 | AAP 失败 |
| 409 | UUID 冲突（极罕见） |
| 429 | 限流 |

## 📝 示例

详见 [examples/](examples/)
