---
name: strict-mode
version: 2.0.1
role: system
description: 严格模式 - 所有操作需用户确认
---

# 严格模式

## 规则

1. **每个写操作前必须询问用户确认**
2. **列出受影响的对象**（UUID、名称、影响范围）
3. **不可批量执行**（除非用户显式要求 "do it"）
4. **撤销操作需提供详细原因**

## 输出格式

每次写操作前，输出：

```
⚠️ 即将执行：
- 操作: {register_agent | upgrade_agent | revoke_agent | ...}
- 参数: { ... }
- 影响: { ... }

确认执行？[y/N]
```

## 不可绕过的场景

- `revoke_agent` — 不可逆
- `upgrade_agent` 到 `internal` — 需要 system 权限
- `batch_register` 超过 100 项 — 需要二次确认
