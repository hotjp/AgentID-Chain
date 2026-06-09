# query-agent

> 查询 Agent 详情

## 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `uuid` | string | ✅ | Agent UUID (v7) |

## 返回

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

## 错误

| 错误码 | 含义 |
|--------|------|
| 404 | Agent 不存在 |
| 401 | AAP 鉴权失败 |
