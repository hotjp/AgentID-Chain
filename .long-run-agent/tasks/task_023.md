# task_023

## 描述

[P21] Agent Skills 与 Prompts

## 需求 (requirements)

P20 完成

## 验收标准 (acceptance)

- 提供 5+ 套预制 Agent Skills（注册/查询/升级/撤销/批量）
- Skills 遵循 MCP / OpenAI Function Calling 规范
- 提供 10+ 套 Prompt 模板（按场景分类）
- Skills 与 Prompts 可被 LLM 直接调用
- 包含示例工作流（Chain-of-Thought、ReAct）
- 自检脚本验证 Skills 可被加载

## 交付物 (deliverables)

- skills/ 目录（5+ 套 Skills）
- prompts/ 目录（10+ 套模板）
- examples/agent-workflows/（工作流示例）
- scripts/test-skills.sh（加载验证）
- README.md（使用说明）
