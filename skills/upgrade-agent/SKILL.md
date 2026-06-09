# upgrade-agent

> 升级 Agent 等级

## 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `uuid` | string | ✅ | Agent UUID |
| `new_level` | string | ✅ | 目标等级：`test` / `prod` / `internal` |

## 返回

更新后的 Agent 对象（同 query-agent）

## 错误

| 错误码 | 含义 |
|--------|------|
| 400 | 等级无效或路径不合法（如 test → internal） |
| 404 | Agent 不存在 |
| 409 | 状态不允许（已撤销 / 已过期） |

## 等级升级路径

```
test → prod          (允许)
test → internal      (仅 system)
prod → internal      (仅 system)
* → *                (其他组合不允许降级)
```
