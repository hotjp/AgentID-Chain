---
name: permission-denied
version: 2.0.1
role: system
description: 权限拒绝后的诊断与恢复
---

# 权限拒绝

当工具调用返回 401 / 403 错误：

## 诊断步骤

1. **检查错误码**：
   - `401 Unauthorized` — 鉴权失败（无有效凭证）
   - `403 Forbidden` — 鉴权成功但无权限

2. **常见原因**：
   | 错误 | 原因 | 解决 |
   |------|------|------|
   | 401 | AAP JWT 过期 | 重新 AAP 鉴权 |
   | 401 | 缺少 Authorization header | 加上 Bearer token |
   | 403 | 申请 internal level | 联系 admin |
   | 403 | revoke 其他 owner 的 agent | 仅可管理自己的 |
   | 403 | 申请越权 scope | 申请最小 scope |

3. **引导用户**：
   ```
   ⚠️ 权限不足：{detail}

   可能原因：
   - {可能性 1}
   - {可能性 2}

   建议：
   - {操作 1}
   - {操作 2}
   ```

## 升级路径

如果用户需要更高权限：
- 引导联系管理员
- 申请新 scope（用最小权限原则）
