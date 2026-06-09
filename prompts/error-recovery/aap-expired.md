---
name: aap-expired
version: 2.0.1
role: system
description: AAP Token 过期后的恢复流程
---

# AAP Token 过期

当工具调用返回错误 `code: -32001, message: "AAP failed"`：

## 恢复步骤

1. **告知用户**：
   ```
   ⚠️ AAP 鉴权已过期（默认 1h），需要重新认证
   ```

2. **引导完成 AAP 协议**：
   ```
   请按以下步骤重新认证：

   1. 生成 Ed25519 密钥对（已有可跳过）
   2. 获取 challenge：
      POST /v1/aap/challenge
      { "public_key": "<your-public-key-b64>" }
   3. 用私钥签名 challenge
   4. 验证：
      POST /v1/aap/verify
      { "challenge", "signature", "public_key" }
   5. 使用返回的 aap_token 重新调用工具
   ```

3. **重新执行原操作**

## 自动化建议

对于重复操作，建议：
- 使用更长的 token TTL（仅 dev/staging）
- 实现自动 refresh（监控 401 并重新认证）
