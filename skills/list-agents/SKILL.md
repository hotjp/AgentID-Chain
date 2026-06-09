# list-agents

> 列出 Agent（带过滤与分页）

## 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `owner` | string | | 按 owner 过滤 |
| `level` | string | | 按 level 过滤 |
| `status` | string | | 按 status 过滤 |
| `limit` | integer | | 单页数量（默认 50，最大 200） |
| `cursor` | string | | 上一页响应的 next_cursor |

## 返回

```json
{
  "agents": [
    {"agent_id": "...", "owner": "alice", "level": "test", "status": "active", "created_at": "..."}
  ],
  "next_cursor": "eyJ..."  // 下一页；null 表示结束
}
```
