# conditional-upgrade

> 条件工作流 — 根据 owner 决定是否升级

## 📋 场景

批量升级 agent，但仅对"生产团队"的 owner 执行升级。

## 🔄 工作流

```
1. list_agents 获取所有 agent
2. 对每个 agent：
   if owner in PROD_TEAMS:
     upgrade_agent
   else:
     skip
3. 输出汇总
```

## 📝 完整对话

```
USER: 帮我把所有 team-prod-* 的 test agent 升级到 prod

LLM:
  I'll filter and upgrade only production team agents.

TOOL_CALL: list_agents({
  "level": "test",
  "limit": 200
})

TOOL_RESPONSE:
  {
    "agents": [
      {"agent_id": "...", "owner": "team-prod-payment", "level": "test"},
      {"agent_id": "...", "owner": "team-prod-order", "level": "test"},
      {"agent_id": "...", "owner": "team-dev-infra", "level": "test"},
      ...
    ]
  }

LLM (思考):
  - 找到 N 个 agent
  - 过滤 owner 以 "team-prod-" 开头的
  - 预计升级 M 个
  - 跳过 dev team

ASSISTANT:
  Found 10 test agents. 6 belong to production teams.

  Will upgrade:
  - team-prod-payment (3)
  - team-prod-order (2)
  - team-prod-billing (1)

  Will skip:
  - team-dev-infra (4)

  Proceed? [y/N]

USER: y

TOOL_CALL: upgrade_agent({uuid: "...", new_level: "prod"})
TOOL_CALL: upgrade_agent({uuid: "...", new_level: "prod"})
... (6 calls)

ASSISTANT:
  ✅ Upgraded 6 of 6 production agents.

  | Agent ID | Owner | New Level |
  |----------|-------|-----------|
  | 0190... | team-prod-payment | prod |
  ...
```

## 🛠️ 实现

```python
PROD_TEAMS = {"team-prod-payment", "team-prod-order", "team-prod-billing"}

def conditional_upgrade():
    # 1. 列出所有 test agent
    agents = call_tool("list_agents", {"level": "test", "limit": 200})

    # 2. 过滤
    to_upgrade = [a for a in agents["agents"]
                  if a["owner"] in PROD_TEAMS]

    # 3. 用户确认
    if not confirm(f"Upgrade {len(to_upgrade)} agents?"):
        return

    # 4. 升级
    results = []
    for agent in to_upgrade:
        r = call_tool("upgrade_agent", {
            "uuid": agent["agent_id"],
            "new_level": "prod"
        })
        results.append(r)

    return format_results(results)
```
