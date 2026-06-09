# react-loop

> ReAct 模式 — Reasoning + Acting 循环

## 📋 场景

需要多次尝试、观察反馈、再决策的场景。

## 🔄 循环结构

```
THOUGHT → ACTION → OBSERVATION → THOUGHT → ...
```

## 📝 示例：诊断失败的注册

```
USER: 帮 team-prod 注册一个 agent，但一直失败

LLM (THOUGHT 1):
  The user wants to register an agent for team-prod but it's failing.
  I need to:
  1. Try to register to see the error
  2. Diagnose based on the error
  3. Apply fix
  4. Retry

ACTION 1: register_agent({
  "owner": "team-prod",
  "level": "prod"
})

OBSERVATION 1:
  Error: 401 Unauthorized
  Detail: "AAP token invalid or expired"

LLM (THOUGHT 2):
  The AAP token is invalid. I need to refresh it.
  Plan:
  1. Inform user about AAP expiration
  2. Guide through AAP re-auth
  3. Retry register

ACTION 2: (output text, not a tool call)
  "⚠️ AAP token expired. Please complete AAP re-auth:
   ..."

USER: 已完成，重新尝试

ACTION 3: register_agent({
  "owner": "team-prod",
  "level": "prod"
})

OBSERVATION 3:
  Error: 403 Forbidden
  Detail: "owner 'team-prod' requires admin approval"

LLM (THOUGHT 4):
  Different error. This is a permission issue, not auth.
  Plan:
  1. Check what owners the AAP can manage
  2. Try with a different owner (e.g., team-prod-frontend)
  3. Or suggest user contact admin

ACTION 4: get_agent on existing team-prod agents
  → list_agents({owner: "team-prod", limit: 5})

OBSERVATION 4:
  No agents found for team-prod owner

LLM (THOUGHT 5):
  Team-prod owner doesn't exist yet. The "admin approval" error
  means the AAP public key isn't authorized for this owner.
  This needs admin intervention.

ACTION 5: (output text)
  "❌ Registration failed. The AAP key you're using isn't authorized
   for owner 'team-prod'.

   Options:
   1. Use a different owner (e.g., your personal username)
   2. Contact admin to grant your AAP key access
   3. Use level=test (no admin needed)

   Which would you like?"
```

## 🛠️ 关键模式

### 1. 错误分类

```python
ERROR_PATTERNS = {
    "401": "AAP token issue",
    "403": "Permission issue",
    "404": "Resource not found",
    "409": "Conflict (e.g., duplicate)",
    "429": "Rate limited",
    "5xx": "Server error"
}
```

### 2. 重试策略

```python
RETRY_CONFIG = {
    "401": {"max_retries": 1, "delay": 0, "action": "re-auth"},
    "403": {"max_retries": 0, "action": "ask user"},
    "429": {"max_retries": 3, "delay": "exponential", "max_delay": 60},
    "5xx": {"max_retries": 3, "delay": "exponential", "max_delay": 30}
}
```

### 3. 循环终止

避免无限循环：
- 最大重试次数
- 已尝试的错误类型不再重试
- 必要时请求用户介入

## 📚 提示技巧

```
When errors occur, follow ReAct loop:
1. THOUGHT: Analyze the error pattern
2. ACTION: Try a fix or gather more info
3. OBSERVATION: Read the result
4. THOUGHT: Decide next step
5. Repeat until success or max retries

Never retry the same failing action more than 3 times.
After 3 failures, ask the user for guidance.
```
