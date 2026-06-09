# Contributing to AgentID-Chain

Welcome! This document covers everything you need to contribute code, documentation, or skills to AgentID-Chain.

## 📜 行为准则

我们承诺提供一个开放、包容的环境。期望所有贡献者：

- 尊重不同观点和经验
- 优雅地接受建设性批评
- 聚焦对社区最有利的事

## 🚀 快速开始

```bash
# 1. Fork & clone
git clone https://github.com/agentid-chain/agentid-chain.git
cd agentid-chain

# 2. 安装开发工具链
make bootstrap    # 安装 golangci-lint / gosec / govulncheck / commitlint

# 3. 启动基础设施
docker-compose -f docker-compose.dev.yml up -d postgres redis

# 4. 跑测试
make test
make gates         # 执行质量宪法门控

# 5. 起服务
go run ./cmd/agentid
```

## 🧭 开发流程

1. **创建 issue**（如未存在）讨论变更
2. **分支**：从 `main` 拉 `feature/<short-desc>` 或 `fix/<short-desc>`
3. **开发**：遵循 [style.md](docs/contributing/style.md) 与 [development.md](docs/contributing/development.md)
4. **测试**：单元测试覆盖率 ≥ 70%（门控强制）
5. **Commit**：遵循 [Conventional Commits](https://www.conventionalcommits.org/)
6. **PR**：参考 [pr-process.md](docs/contributing/pr-process.md)
7. **CI**：所有门控通过 + 1 reviewer approval
8. **合并**：squash merge

## 🧪 测试要求

| 层级 | 工具 | 要求 |
|------|------|------|
| 单元 | `go test ./...` | 通过；覆盖率 ≥ 70% |
| 集成 | testcontainers | 通过 |
| Lint | `golangci-lint run` | 0 issue |
| 安全 | `gosec` + `govulncheck` | 0 high |
| 文档 | `scripts/check-docs.sh` | 0 error |
| 质量宪法 | `cmd/constitution-gates` | 0 失败 |

## 📁 目录结构

```
agentid-chain/
├── cmd/                          # 可执行入口
│   ├── agentid/                  # 主服务 (Gateway)
│   ├── constitution-gates/       # 质量门控 CLI
│   ├── migration-tool/           # DB 迁移工具
│   └── mock-chain/               # 链 mock（开发用）
├── internal/
│   ├── cli/                      # CLI 框架 (cobra)
│   ├── config/                   # 配置加载
│   ├── domain/                   # L2 领域层
│   ├── gates/                    # 质量门控框架 + 7 门
│   ├── gateway/                  # L5 网关（connect-go）
│   ├── mcp/                      # MCP 服务器
│   ├── prompt/                   # Prompt 模板
│   ├── service/                  # L4 服务层
│   ├── storage/                  # L1 存储层
│   └── telemetry/                # OTel / 日志 / 脱敏
├── deploy/
│   ├── helm/                     # Helm chart
│   └── gitops/                   # ArgoCD
├── docs/                         # 详细文档
│   ├── architecture/             # 架构 + ADR
│   ├── api/                      # API 协议
│   ├── operations/               # 部署/运维
│   ├── runbooks/                 # 故障 runbook
│   ├── guides/                   # 用户指南
│   └── contributing/             # 贡献流程
├── .long-run-agent/              # LRA 任务治理
└── scripts/                      # 构建/发布/检查脚本
```

## 🏗️ 架构层级（铁律）

```
L5-Gateway → L3-Authz → L4-Service → L2-Domain → L1-Storage
```

依赖单向向下。详见 [docs/architecture/5-layer.md](docs/architecture/5-layer.md)。

## 🎯 贡献类型

### 🐛 Bug 报告

- 用 [bug report template](.github/ISSUE_TEMPLATE/bug.md)
- 包含：复现步骤、期望/实际、版本、commit hash
- 附日志/trace（如有）

### ✨ 新功能

- 先开 RFC issue 讨论设计
- 引用 ADR（如涉及架构变更）
- 拆分为可独立合并的子任务

### 📚 文档

- 中文 / English 任一即可
- 包含代码示例
- 跑 `scripts/check-docs.sh` 验证

### 🎁 Agent Skills / Prompts

参考 [examples/agent-skills/](examples/agent-skills/) 模板，需含：

- `SKILL.md`（YAML frontmatter：name/version/description）
- `schema.json`（含 name + parameters）
- `examples/`（调用示例）
- 通过 `scripts/test-skills.sh`

## 🔒 安全

发现安全漏洞请**不要**公开 issue，发送邮件到 security@agentid-chain.example。
详见 [docs/SECURITY.md](docs/SECURITY.md)。

## 📜 许可证

贡献即同意 [Apache License 2.0](LICENSE)。

## 🙏 致谢

感谢所有贡献者！名单见 [docs/CONTRIBUTORS.md](docs/CONTRIBUTORS.md)。

## 📚 延伸阅读

- [开发指南](docs/contributing/development.md) — 详细开发流程
- [代码风格](docs/contributing/style.md) — Go 编码规范
- [PR 流程](docs/contributing/pr-process.md) — 评审与合并
- [治理章程](docs/contributing/governance.md) — 项目治理原则
- [架构总览](docs/ARCHITECTURE.md) — 系统架构入口
