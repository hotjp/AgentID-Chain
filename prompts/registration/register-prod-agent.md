---
name: register-prod-agent
version: 2.0.1
role: user
description: 引导用户注册 prod 等级 Agent（带安全检查）
variables:
  - owner: Agent 所有者
  - service: 服务名
  - env: 环境
tools:
  - register_agent
---

# 任务

帮助用户注册一个 **prod 等级** 的 Agent（生产环境）。

# 前置条件

- ✅ 已完成 AAP 鉴权
- ✅ owner 已确认
- ✅ service 名称符合命名规范（小写 + 短横线）

# 步骤

1. 收集必填信息：
   - `owner` — 团队 / 用户
   - `service` — 服务名（用于 metadata.service）
   - `env` — 环境（prod / staging / dev）

2. 构造 metadata（推荐）：
   ```json
   {
     "service": "{service}",
     "env": "{env}",
     "region": "...",
     "team": "...",
     "contact": "..."
   }
   ```

3. 调用 `register_agent`：
   ```json
   {
     "owner": "{owner}",
     "level": "prod",
     "metadata": { ... }
   }
   ```

# 安全提醒

⚠️ prod 等级 agent 享有正式权限，**撤销前需业务方确认**。
