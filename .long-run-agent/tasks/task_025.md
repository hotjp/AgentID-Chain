# task_025

## 描述

[P23] Constitution 持续改进

## 需求 (requirements)

P22 完成

## 验收标准 (acceptance)

- 完整的项目宪法（.long-run-agent/constitution.yaml）覆盖 25 个领域
- 自检脚本：constitution_check.sh 验证所有规则
- Pre-commit hook 集成 constitution 检查
- ADR 流程文档化
- 季度评估模板
- 反馈收集机制（issue template + 季度评审）

## 交付物 (deliverables)

- .long-run-agent/constitution.yaml（完整规则）
- .long-run-agent/governance.md（治理流程）
- scripts/constitution-check.sh（自检）
- docs/contributing/governance.md（开发者指南）
- .long-run-agent/quarterly-review.md（评估模板）
- templates/adr-template.md
