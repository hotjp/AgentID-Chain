---
name: moltcaptcha-solve
version: 2.0.1
role: system
description: MoltCaptcha 求解与异常处理
---

# MoltCaptcha 求解

MoltCaptcha 是 AgentID-Chain 的反机器人验证机制。当系统判定请求为可疑自动化流量时触发。

## 触发条件

- 1 分钟内 ≥ 5 次失败 AAP 验证
- 来自同一 IP / ASN 的高频注册
- User-Agent 缺失或异常
- 客户端时间偏差 > 5 分钟

## 求解流程

### Step 1: 获取 challenge

```http
GET /v1/captcha/challenge
```

响应：
```json
{
  "challenge_id": "cap_abc123",
  "question": "What is 7 × 8?",
  "type": "math",
  "expires_in": 60
}
```

支持类型：
- `math`: 简单算术
- `image`: 图片验证码
- `slider`: 滑块拼图
- `molt`: 拖动排序（多 agent 任务）

### Step 2: 提交答案

```http
POST /v1/captcha/verify
Content-Type: application/json

{
  "challenge_id": "cap_abc123",
  "answer": "56"
}
```

响应（成功）：
```json
{
  "token": "cap_tok_xyz789",
  "expires_in": 600
}
```

### Step 3: 后续请求带 captcha token

```http
POST /v1/agents
X-Captcha-Token: cap_tok_xyz789
...
```

## 异常处理

### ❌ CAPTCHA_FAILED

```json
{"error": "CAPTCHA_FAILED", "remaining_attempts": 2}
```

**动作**：
1. 不重试同一 challenge
2. 重新获取新 challenge
3. 通知用户："验证失败，已自动重新生成"

### ❌ CAPTCHA_EXPIRED

挑战超过 60s 过期。

**动作**：
1. 重新获取 challenge
2. 重新求解

### ❌ CAPTCHA_RATE_LIMITED

```json
{"error": "RATE_LIMITED", "retry_after": 3600}
```

**动作**：
1. **不要**自动重试
2. 通知用户："已触发限流，请 1 小时后再试或联系管理员"
3. 提示更换 IP / 使用不同的 agent

## 何时应该告知用户

- 连续 3 次失败
- 触发了限流
- 图片 / 滑块验证码（LLM 无法直接求解）

示例告知：

```
⚠️ 需要人工完成验证码

我无法自动完成此验证。请：
1. 打开 https://agentid-chain.example.com/captcha/...
2. 完成验证
3. 把 token 贴回来

或者等待 1 小时后重试。
```

## 客户端最佳实践

- 缓存成功的 captcha token（10 分钟内可复用）
- 实现指数退避（1s, 2s, 4s, 8s）
- 区分人机请求：实时交互 vs 后台批处理
- 报告异常到 metrics（成功率 / 延迟）
