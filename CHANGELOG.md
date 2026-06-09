# Changelog

All notable changes to AgentID-Chain will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- 完整文档体系（架构、API、运维、Runbook、贡献指南）
- Agent Skills 库（8 套 MCP / Function Calling Skills）
- Prompt 模板库（13 套系统/场景/错误恢复模板）
- 5 套 Agent 工作流示例（CoT、ReAct、条件、顺序）

## [2.0.1] - 2026-06-09

### Added
- **P19 可观测性**：
  - W3C Trace Context 传播
  - OpenTelemetry 语义约定（HTTP/DB/agentid/AAP/A2A/chain/cache）
  - Prometheus 指标（HTTP/AAP/A2A/backend/cache）
  - 结构化日志 + 敏感字段脱敏
  - Grafana 仪表板（9 面板）
  - Prometheus 告警（可用性、SLO 燃烧、延迟、AAP、缓存、资源、链上）
- **P18 性能工程**：
  - 微基准（UUID 205ns、AAP 53μs、RBAC 8ns、Register 11μs）
  - k6 负载测试脚本（register / a2a / cache）
  - pprof 性能分析指南
  - Redis Pipeline（5-80x 加速）
  - 连接池调优（25/10/5m）
  - 慢查询监控
  - SLO 定义（99.9% 可用性 / P99<100ms）
- **P17 安全与合规**：
  - Cosign 镜像签名（key + keyless OIDC）
  - TLS 中间件（HSTS / XFP）
  - 安全响应头（CSP / COOP/COEP / CORP）
  - 限流（per IP/agent/endpoint/global）
  - 密钥轮换（KeySet 双密钥）
  - 日志脱敏（JWT/DSN/AWS/PEM 等）
  - OWASP API Top 10 + CWE Top 25 审计
- **P16 CI/CD**：
  - 7 个 GitHub Actions workflow
  - pre-commit + commitlint + husky
  - CODEOWNERS
- **P15 测试基础设施**：
  - testcontainers（PG/Redis）
  - miniredis
  - gomock 模板
  - 覆盖率门槛 ≥ 70%
- **P14 Docker 部署**：
  - 4 个 distroless 镜像
  - buildx 多平台构建
  - cosign 集成
- **P13 提示范式**：自然语言意图解析
- **P12 MCP 集成**：JSON-RPC 工具调用
- **P11 CLI**：cobra 框架
- **P10 API Gateway**：connect-go 入口

### Changed
- 存储统一为 PostgreSQL（v1 时代 MySQL/SQLite 弃用）
- 架构对齐 5 层分层规范
- 严格自上而下依赖（铁律）

### Security
- AAP 协议使用 EdDSA Ed25519
- 链上操作使用 AES-256-GCM 加密私钥

## [2.0.0] - 2026-01-15

### Added
- 初始架构：5 层分层 + 依赖倒置
- 混合存储：local / onchain / hybrid
- AAP / MoltCaptcha / A2A / RBAC 鉴权
- 链上适配：FISCO / Polygon / BSC / mock
- CLI / MCP / A2A / Prompt 四种接入范式

## [1.0.0] - 2025-06-01

### Added
- 初始版本
- 基础 Agent 注册 / 查询
- MySQL / SQLite 支持

[Unreleased]: https://github.com/agentid-chain/agentid-chain/compare/v2.0.1...HEAD
[2.0.1]: https://github.com/agentid-chain/agentid-chain/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/agentid-chain/agentid-chain/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/agentid-chain/agentid-chain/releases/tag/v1.0.0
