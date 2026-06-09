---
name: batch-register
version: 2.0.1
role: user
description: 批量注册 Agent（带确认与分页）
variables:
  - agents_yaml: YAML 格式的 agent 列表
tools:
  - batch_register
---

# 任务

帮助用户**批量注册**多个 Agent。

# 输入格式

用户提供 YAML / JSON / 表格：

```yaml
- owner: team-a
  level: prod
  metadata:
    service: payment
- owner: team-a
  level: prod
  metadata:
    service: order
```

# 步骤

1. **解析**用户输入，验证格式
2. **预估数量**：超过 100 项需分批
3. **展示摘要**让用户确认：
   ```
   将注册 N 个 agent：
   - 5 个 test（开发）
   - 3 个 prod（生产）
   - 2 个 internal（系统）
   
   确认？[y/N]
   ```
4. 调用 `batch_register`：
   ```json
   {
     "agents": [...]
   }
   ```
5. 展示结果（成功 N / 失败 M），失败项需解释

# 错误处理

- 部分失败不影响其他
- 输出失败项的 error 详情
- 询问用户是否重试失败项
