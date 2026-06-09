# revoke-agent

> 撤销 Agent（不可逆）

## 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `uuid` | string | ✅ | Agent UUID |
| `reason` | string | ✅ | 撤销原因（用于审计） |

## 返回

无（HTTP 204 No Content）

## 错误

| 错误码 | 含义 |
|--------|------|
| 404 | Agent 不存在 |
| 409 | 已被撤销 |

## ⚠️ 警告

- **不可逆**：撤销后无法恢复
- 撤销会写入 `audit_logs`（含 reason）
- hybrid 模式下会同步到链上
