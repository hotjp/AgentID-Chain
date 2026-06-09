# Agent 工作流示例

> 端到端工作流 — 展示 Agent Skills 与 Prompts 的组合使用

## 📋 索引

| 工作流 | 模式 | 描述 |
|--------|------|------|
| [register-and-query](register-and-query.md) | 顺序 | 注册 → 查询 |
| [batch-then-verify](batch-then-verify.md) | 顺序 | 批量注册 → 逐一验证 |
| [conditional-upgrade](conditional-upgrade.md) | 条件 | 根据 owner 决定是否升级 |
| [chain-cot](chain-cot.md) | CoT | 思考链推理 |
| [react-loop](react-loop.md) | ReAct | 推理 + 行动循环 |

## 🔄 工作流模式

### 1. 顺序工作流

```mermaid
graph LR
  A[Step 1] --> B[Step 2] --> C[Step 3]
```

### 2. 条件工作流

```mermaid
graph LR
  A[Step 1] --> B{条件}
  B -- yes --> C[Step 2a]
  B -- no --> D[Step 2b]
```

### 3. ReAct 循环

```mermaid
graph LR
  A[思考] --> B[行动]
  B --> C[观察]
  C --> A
```

## 📂 文件命名

`{pattern}-{description}.md` 例如：
- `register-and-query.md` — 顺序
- `conditional-upgrade.md` — 条件
- `react-loop.md` — ReAct
