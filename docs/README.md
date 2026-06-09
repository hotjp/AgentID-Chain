# AgentID-Chain Documentation

> 完整文档中心 — 面向不同角色的入口指南

## 📚 文档索引

### 按角色

| 角色 | 推荐阅读 | 关键文档 |
|------|---------|---------|
| **🆕 新用户** | 5 分钟上手 | [Quick Start](guides/quickstart.md) → [First Agent](guides/first-agent.md) |
| **👨‍💻 开发者** | 接入 / 二次开发 | [Architecture](architecture/overview.md) → [Contributing](contributing/development.md) → [API Reference](api/openapi.md) |
| **🛠️ 运维** | 部署 / 监控 / 排障 | [Operations](operations/deployment.md) → [Runbooks](runbooks/) → [Observability](observability/grafana-dashboard.json) |
| **🔐 安全** | 审计 / 合规 | [Security Audit](SECURITY_AUDIT.md) → [Secret Rotation](SECRET_ROTATION.md) |

### 按主题

| 分类 | 文档 |
|------|------|
| **🏛️ 架构** | [Overview](architecture/overview.md) · [5-Layer](architecture/5-layer.md) · [Storage](architecture/storage.md) · [ADR](architecture/adr/) |
| **🔌 API** | [OpenAPI](api/openapi.md) · [AAP Protocol](api/aap.md) · [A2A Protocol](api/a2a.md) · [MCP](api/mcp.md) |
| **🚀 部署** | [Docker](operations/deployment.md) · [Local Dev](operations/local-dev.md) · [Config](operations/configuration.md) |
| **📊 观测** | [Metrics](operations/metrics.md) · [SLO](SLO.md) · [Profiling](PROFILING.md) · [Dashboards](observability/grafana-dashboard.json) |
| **📖 指南** | [Quick Start](guides/quickstart.md) · [User Journeys](guides/journeys.md) · [FAQ](guides/faq.md) |
| **🔧 运维** | [Runbooks](runbooks/) · [Troubleshooting](operations/troubleshooting.md) · [Migration](operations/migration.md) |
| **⚡ 性能** | [Benchmarks](perf/) · [Connection Pool](perf/connection-pool-tuning.md) · [Redis Pipeline](perf/redis-pipeline.md) |
| **🔒 安全** | [Audit](SECURITY_AUDIT.md) · [Secret Rotation](SECRET_ROTATION.md) · [Govulncheck](security/govulncheck.md) |
| **🛠️ 贡献** | [Development](contributing/development.md) · [Style](contributing/style.md) · [PR Process](contributing/pr-process.md) |

### 快速链接

- [需求基线](AgentID-Chain-技术文档-v2.0.1.md) — v2.0.1 完整规格
- [架构规范](architecture.md) — 5 层分层 + 依赖倒置
- [Changelog](../CHANGELOG.md) — 版本变更记录
- [License](../LICENSE) — Apache-2.0

## 🗂️ 目录结构

```
docs/
├── README.md                       # 本文件（文档总索引）
├── SUMMARY.md                      # GitBook 风格目录
├── AgentID-Chain-技术文档-v2.0.1.md  # 需求基线（frozen）
├── architecture.md                 # 架构规范（frozen）
│
├── architecture/                   # 架构详解
│   ├── overview.md
│   ├── 5-layer.md
│   ├── storage.md
│   ├── protocols.md
│   └── adr/                        # 架构决策记录
│       ├── 0001-storage-hybrid.md
│       ├── 0002-aap-eddsa.md
│       └── 0003-uuid-v7.md
│
├── api/                            # 协议规范
│   ├── openapi.md                  # OpenAPI 3.0 入口
│   ├── aap.md                      # AAP 协议
│   ├── a2a.md                      # A2A 协议
│   ├── mcp.md                      # MCP 协议
│   └── openapi.yaml                # 自动生成的规范
│
├── operations/                     # 运维文档
│   ├── deployment.md
│   ├── local-dev.md
│   ├── configuration.md
│   ├── metrics.md
│   └── troubleshooting.md
│
├── runbooks/                       # 故障排查手册
│   ├── README.md
│   ├── high-error-rate.md
│   ├── db-connection-pool.md
│   ├── chain-rpc-failure.md
│   ├── aap-replay-attack.md
│   └── disk-pressure.md
│
├── guides/                         # 用户旅程
│   ├── quickstart.md
│   ├── first-agent.md
│   ├── journeys.md                 # 5 个典型用户旅程
│   └── faq.md
│
├── contributing/                   # 贡献者指南
│   ├── development.md
│   ├── style.md
│   └── pr-process.md
│
├── observability/                  # 监控资产
│   ├── grafana-dashboard.json
│   └── prometheus-alerts.yaml
│
├── perf/                           # 性能工程
│   ├── uuid-benchmark.md
│   ├── aap-benchmark.md
│   ├── rbac-benchmark.md
│   ├── register-benchmark.md
│   ├── connection-pool-tuning.md
│   ├── redis-pipeline.md
│   ├── slow-query-monitoring.md
│   └── leak-detection.md
│
└── security/                       # 安全相关
    ├── SECURITY_AUDIT.md
    ├── SECRET_ROTATION.md
    └── govulncheck.md
```

## 🔍 文档维护

### 验证文档

```bash
# 校验所有 markdown 链接
./scripts/check-docs.sh
```

### 文档编写约定

- 使用 **GFM** (GitHub Flavored Markdown)
- 中英文混排时，全角标点前后保留 1 空格
- 代码块必须声明语言（` ```go ` 而非 ` ``` `）
- 每个子目录必须有 `README.md` 索引
- ADR 使用 [MADR](https://adr.github.io/madr/) 模板

## 📝 反馈

发现文档问题？[提交 Issue](https://github.com/agentid-chain/agentid-chain/issues/new?template=docs.md)
