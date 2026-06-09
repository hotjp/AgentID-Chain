# task_022

## 描述

[P20] 文档体系

## 需求 (requirements)

P19 完成

## 验收标准 (acceptance)

- docs/ 目录结构清晰（architecture / api / operations / runbooks / faq）
- 完整 README 索引（按角色：用户/开发者/运维/安全）
- 架构文档与代码同步（ADR 记录关键决策）
- 5 套典型用户旅程文档（注册 / 升级 / 撤销 / 链上 / 监控）
- OpenAPI 3.0 / Connect 协议规范自动生成
- 故障排查 Runbook（5+ 场景）
- FAQ 覆盖常见问题
- 文档 Lint（markdown-link-check / markdownlint）

## 交付物 (deliverables)

- docs/README.md（总索引）
- docs/architecture/（架构 + ADR）
- docs/api/（OpenAPI 规范）
- docs/operations/（部署 + 运维 + Runbook）
- docs/guides/（用户旅程 + FAQ）
- docs/contributing/（开发者指南）
- docs/SUMMARY.md（gitbook 风格目录）
- scripts/check-docs.sh（文档 Lint）
- mkdocs.yml（可选：mkdocs 静态站点）
