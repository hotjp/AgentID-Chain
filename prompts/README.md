# Prompt 模板库

> 预制 Prompt 模板 — 用于 LLM 系统提示与用户引导

## 📋 分类

| 类别 | 数量 | 模板 |
|------|------|------|
| **系统提示** | 3 | [system/](./system/) |
| **注册场景** | 3 | [registration/](./registration/) |
| **查询场景** | 2 | [query/](./query/) |
| **生命周期** | 3 | [lifecycle/](./lifecycle/) |
| **错误恢复** | 2 | [error-recovery/](./error-recovery/) |

## 🚀 快速使用

```python
from langchain.prompts import load_prompt

prompt = load_prompt("prompts/registration/register-test-agent.yaml")
formatted = prompt.format(owner="alice", level="test")
```

## 📂 目录

```
prompts/
├── README.md
├── system/
│   ├── assistant.md           # 通用助手系统提示
│   ├── strict-mode.md         # 严格模式
│   └── security-first.md      # 安全优先模式
├── registration/
│   ├── register-test-agent.md
│   ├── register-prod-agent.md
│   └── batch-register.md
├── query/
│   ├── find-active-agents.md
│   └── check-agent-status.md
├── lifecycle/
│   ├── upgrade-agent.md
│   ├── revoke-agent.md
│   └── audit-trail.md
└── error-recovery/
    ├── aap-expired.md
    └── permission-denied.md
```

## 🛠️ 模板规范

每个模板包含：
- **role**: system / user / assistant
- **version**: 模板版本
- **variables**: 可填充变量
- **tools**: 可调用的工具列表
- **examples**: few-shot 示例

## ✅ 自检

```bash
./scripts/test-prompts.sh
```
