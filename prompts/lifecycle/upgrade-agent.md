---
name: upgrade-agent
version: 2.0.1
role: user
description: 升级 Agent 等级
variables:
  - uuid: Agent UUID
  - new_level: 目标等级
tools:
  - upgrade_agent
---

# 任务

升级 Agent 到更高等级。

# 等级路径

```
test → prod          (允许)
test → internal      (需 system 权限)
prod → internal      (需 system 权限)
* → *                (其他组合不允许)
```

# 步骤

1. 验证升级路径合法
2. **确认用户意图**（升级不可降级）
3. 调用 `upgrade_agent`：
   ```json
   {
     "uuid": "{uuid}",
     "new_level": "{new_level}"
   }
   ```

# 前置检查

- [ ] Agent 状态为 `active`
- [ ] 当前 owner 有升级权限
- [ ] 目标 level 在允许路径内

# 输出

```
✅ 升级成功
- {uuid}: test → prod
- 链上确认：{chain_tx_hash}
```

# 拒绝

- 路径不合法 → 解释并建议
- 状态非 active → 提示用户先恢复
