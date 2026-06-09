# rollback

> 回滚 AgentID-Chain 到上一个版本

## 📋 描述

Helm 回滚 / 数据库迁移回滚 / 链上交易回滚（支持撤销）。

**适用场景**：

- 发布事故恢复
- A/B 测试不达预期
- 数据问题修复

## 🔧 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `scope` | string | ✅ | `app` / `db` / `chain` / `all` |
| `revision` | int | | 指定回滚版本号（默认上一版） |
| `dry_run` | bool | | 演练（默认 false） |

## 📤 返回

```json
{
  "scope": "app",
  "from_revision": 12,
  "to_revision": 11,
  "dry_run": false,
  "duration_s": 45
}
```

## 🛠️ 实现

```python
def rollback(scope, revision=None, dry_run=False):
    args = {"scope": scope, "dry_run": dry_run}
    if revision: args["revision"] = revision
    return call_tool("rollback", args)
```

## 📚 回滚策略

| Scope | 工具 | RPO | RTO |
|-------|------|-----|-----|
| app | helm rollback | 0 | < 60s |
| db | migrate down | ≤ 5min | < 5min |
| chain | revoke | 0 | < 1min |
| all | 组合 | ≤ 5min | < 10min |

## ⚠️ 注意

- DB 回滚可能丢数据，先备份
- 链上回滚需 owner 二次确认
- 生产回滚需 2 人审批
