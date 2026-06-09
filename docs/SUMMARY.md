# Summary

> GitBook 风格的目录树 — 可与 `mkdocs nav` 或 `gitbook serve` 配合使用

## 入门

* [简介](README.md)
* [快速开始](guides/quickstart.md)
* [第一个 Agent](guides/first-agent.md)

## 架构

* [架构总览](architecture/overview.md)
* [5 层分层](architecture/5-layer.md)
* [存储后端](architecture/storage.md)
* [协议概览](architecture/protocols.md)
* [架构决策记录 (ADR)](architecture/adr/README.md)
  * [ADR-0001: 混合存储架构](architecture/adr/0001-storage-hybrid.md)
  * [ADR-0002: AAP 使用 EdDSA](architecture/adr/0002-aap-eddsa.md)
  * [ADR-0003: UUID v7 默认](architecture/adr/0003-uuid-v7.md)

## API 与协议

* [OpenAPI 规范](api/openapi.md)
* [AAP 协议](api/aap.md)
* [A2A 协议](api/a2a.md)
* [MCP 协议](api/mcp.md)

## 运维指南

* [部署](operations/deployment.md)
* [本地开发](operations/local-dev.md)
* [配置参考](operations/configuration.md)
* [指标与监控](operations/metrics.md)
* [故障排查](operations/troubleshooting.md)
* [数据迁移](operations/migration.md)

## 故障 Runbook

* [Runbook 索引](runbooks/README.md)
* [高错误率](runbooks/high-error-rate.md)
* [数据库连接池耗尽](runbooks/db-connection-pool.md)
* [链上 RPC 失败](runbooks/chain-rpc-failure.md)
* [AAP 重放攻击](runbooks/aap-replay-attack.md)
* [磁盘压力](runbooks/disk-pressure.md)

## 用户指南

* [用户旅程](guides/journeys.md)
* [常见问题](guides/faq.md)

## 性能工程

* [UUID 基准](perf/uuid-benchmark.md)
* [AAP 基准](perf/aap-benchmark.md)
* [RBAC 基准](perf/rbac-benchmark.md)
* [Register 基准](perf/register-benchmark.md)
* [连接池调优](perf/connection-pool-tuning.md)
* [Redis Pipeline](perf/redis-pipeline.md)
* [慢查询监控](perf/slow-query-monitoring.md)
* [内存泄漏检测](perf/leak-detection.md)
* [性能分析手册](PROFILING.md)
* [SLO 定义](SLO.md)

## 安全

* [安全审计](SECURITY_AUDIT.md)
* [密钥轮换](SECRET_ROTATION.md)
* [漏洞扫描](security/govulncheck.md)

## 贡献

* [开发流程](contributing/development.md)
* [代码规范](contributing/style.md)
* [PR 流程](contributing/pr-process.md)

## 监控资产

* [Grafana 仪表板](observability/grafana-dashboard.json)
* [Prometheus 告警](observability/prometheus-alerts.yaml)
