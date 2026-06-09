# AgentID-Chain

> **AI Agent 分布式身份与权限网关** — v2.0.1 (PostgreSQL + 混合存储 + 四种接入范式)

[![Status](https://img.shields.io/badge/status-在建-yellow.svg)](#)
[![Go](https://img.shields.io/badge/go-1.26.1-00ADD8.svg)](https://go.dev/)
[![Spec](https://img.shields.io/badge/spec-v2.0.1%20FROZEN-blue.svg)](docs/AgentID-Chain-技术文档-v2.0.1.md)
[![Build](https://github.com/agentid-chain/agentid-chain/actions/workflows/build.yml/badge.svg)](https://github.com/agentid-chain/agentid-chain/actions/workflows/build.yml)
[![Test](https://github.com/agentid-chain/agentid-chain/actions/workflows/test.yml/badge.svg)](https://github.com/agentid-chain/agentid-chain/actions/workflows/test.yml)
[![Lint](https://github.com/agentid-chain/agentid-chain/actions/workflows/lint.yml/badge.svg)](https://github.com/agentid-chain/agentid-chain/actions/workflows/lint.yml)
[![Coverage](https://github.com/agentid-chain/agentid-chain/actions/workflows/coverage-check.yml/badge.svg)](https://github.com/agentid-chain/agentid-chain/actions/workflows/coverage-check.yml)
[![Docker Build](https://github.com/agentid-chain/agentid-chain/actions/workflows/docker-build.yml/badge.svg)](https://github.com/agentid-chain/agentid-chain/actions/workflows/docker-build.yml)
[![Security](https://github.com/agentid-chain/agentid-chain/actions/workflows/security.yml/badge.svg)](https://github.com/agentid-chain/agentid-chain/actions/workflows/security.yml)
[![codecov](https://codecov.io/gh/agentid-chain/agentid-chain/branch/main/graph/badge.svg)](https://codecov.io/gh/agentid-chain/agentid-chain)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Scorecard](https://img.shields.io/ossf-scorecard/github.com/agentid-chain/agentid-chain?label=openssf%20scorecard)](https://scorecard.dev/viewer/?uri=github.com/agentid-chain/agentid-chain)

AgentID-Chain 是面向 AI Agent 生态的分布式身份与权限网关。在保持「混合存储 + 多协议接入」核心能力的基础上，
v2.0.1 将数据库统一为 PostgreSQL，对齐 5 层架构规范，并补全 AAP / A2A / MoltCaptcha 协议细节，
可直接进入工程实施阶段。

---

## ✨ 核心特性

| 维度 | 能力 |
|---|---|
| **身份容量** | 万亿级 UUID（v4 / v7 双策略），架构层面无冲突 |
| **存储后端** | `local` (PostgreSQL + ent) / `onchain` (FISCO/Polygon/BSC/mock) / `hybrid` |
| **准入控制** | **AAP**（Challenge-Response, EdDSA）+ **MoltCaptcha**（SMHL 反向 CAPTCHA，Go 重写） |
| **接入范式** | **CLI**（cobra） / **MCP**（JSON-RPC） / **A2A**（EdDSA JWT + Redis 撤销） / **Prompt**（NL 解析） |
| **权限模型** | RBAC 位掩码 + 可配置等级模板（默认两级：测试 / 普通） |
| **观测体系** | OpenTelemetry Trace + Prometheus Metrics + slog 结构化日志 |
| **部署模式** | 二进制 / Docker 全栈 compose（dev / local-only / hybrid） |

---

## 🚀 快速开始

> 详细安装与配置请参考 [§7 Quick Start](docs/AgentID-Chain-技术文档-v2.0.1.md#7-quick-start)

### 本地独立运行（开发模式）

```bash
# 1. 启动基础设施（PostgreSQL + Redis）
docker-compose -f docker-compose.dev.yml up -d postgres redis

# 2. 拉起业务服务（API Gateway / Auth / Tag Sense）
go run ./cmd/agentid serve

# 3. 验证健康
curl http://localhost:8080/live
```

端口规划遵循 **尾号规则**：

| 服务 | HTTP | Metrics | Pprof |
|---|---|---|---|
| API Gateway | `8080` | `9090` | `6060` |
| Auth Center | `8081` | `9091` | `6061` |
| Tag Sense   | `8082` | `9092` | `6062` |

### Docker Compose 全栈

```bash
docker-compose -f docker-compose.dev.yml up -d
docker-compose -f docker-compose.dev.yml logs -f api-gateway
```

### 第一个 Agent 注册

```bash
# CLI 范式
agentid register --owner alice --level test --backend local

# 或通过 Prompt 范式（自然语言）
agentid prompt "为 alice 注册一个测试等级的 agent，本地后端"
```

---

## 🧱 架构总览

AgentID-Chain 遵循严格的 **5 层架构**（详见 [architecture.md](docs/architecture.md)）：

```
L5  Gateway     ──┐  TLS / 限流 / UA 拦截 / 路由
                  │
L3  Authz       ──┤  AAP / MoltCaptcha / A2A Token / RBAC
                  │
L4  Service     ──┤  Register / Upgrade / Batch / Revoke 编排
                  │
L2  Domain      ──┤  Agent 实体 / UUID 生成 / 状态机 / 事件
                  │
L1  Storage     ──┘  Backend 接口 → PostgreSQL / 链上 / Redis
```

**铁律**：
- 严格自上而下依赖，**禁止跨层跳跃**
- L2 Domain **禁止 import 第三方包**（除标准库）
- L4 Service 通过 interface 注入插件，**禁止直接 import 插件包**

完整模块职责矩阵见技术文档 [§2.2](docs/AgentID-Chain-技术文档-v2.0.1.md#22-模块职责)。

---

## 📚 文档索引

| 文档 | 说明 |
|---|---|
| [技术文档 v2.0.1 (FROZEN)](docs/AgentID-Chain-技术文档-v2.0.1.md) | **当前需求基线** — 写完只接受 bug 修复 |
| [architecture.md](docs/architecture.md) | 5 层分层 + 依赖倒置规范 |
| [CLAUDE.md](CLAUDE.md) | Claude Code 专属约定（端口规划 / 服务清单） |
| [agent.md](agent.md) | LRA 工具使用入口 |
| [lra.md](lra.md) | LRA 任务管理命令参考 |
| [技术文档 v2.0](docs/AgentID-Chain-技术文档-v2.0.md) | 历史版本（已被 v2.0.1 取代，保留作对照） |

> 任务进度由 **LRA**（Long-Running Agent）统一管理：
> ```bash
> lra ready              # 查看可认领任务
> lra show <id>          # 查看任务详情
> ```

---

## 🛠️ 技术栈

| 类别 | 选型 |
|---|---|
| 语言 | Go 1.26.1 |
| RPC | connectrpc.com/connect (Connect-Go) |
| ORM | ent + pgx/v5 |
| 缓存 | go-redis/v9 |
| 配置 | koanf v2 |
| 日志 | log/slog |
| 观测 | OpenTelemetry + Prometheus + otelpgx |
| CLI | cobra |
| 加密 | crypto/ed25519（EdDSA） |
| Lint | golangci-lint v2 |
| 提交规范 | commitlint (Conventional Commits) + Husky |

---

## 🤝 贡献

1. **任务认领**：所有开发任务通过 LRA 调度 — `lra ready` 查看可领；不要使用 markdown TODO。
2. **分层约束**：提交前自检 `architecture.md` 5 层铁律，违反者 CI 会拒绝。
3. **提交规范**：遵循 [Conventional Commits](https://www.conventionalcommits.org/)；commit-msg hook 会自动校验。
4. **测试覆盖**：包级覆盖率 ≥ **70%**（NON_NEGOTIABLE），CI 强制门禁。
5. **变更范围**：单 PR 单 LRA task，commit body 必须带 `Refs: task_xxx_xx (LRA Px.x)`。

详细贡献流程请见技术文档 [§10 Contributing](docs/AgentID-Chain-技术文档-v2.0.1.md#10-贡献指南)。

---

## 📄 License

待定（项目初始化阶段）。

---

*Last updated: 2026-06-08*
