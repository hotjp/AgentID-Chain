# batch-register

> 批量注册多个 Agent

## 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `agents` | array | ✅ | Agent 列表（每项同 register_agent） |

每项包含：
- `owner` (string, 必填)
- `level` (string, 必填)
- `metadata` (object, 可选)

## 返回

```json
{
  "results": [
    {"agent_id": "...", "owner": "alice", "level": "test", "status": "active"},
    {"agent_id": "...", "owner": "bob", "level": "prod", "status": "active"}
  ],
  "success_count": 2,
  "failure_count": 0
}
```

## 限制

- 最多 1000 个 / 批
- 部分失败不影响其他项
- 整体超时 30s
