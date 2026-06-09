# 开发流程

> 贡献者完整指南：从环境搭建到 PR 合并

## 🛠️ 环境搭建

### 1. 克隆仓库

```bash
git clone https://github.com/agentid-chain/agentid-chain.git
cd agentid-chain
```

### 2. 安装依赖

```bash
# Go 工具
go mod download

# 开发工具
make tools  # 安装 golangci-lint, pre-commit, mockery 等

# pre-commit hooks
pre-commit install
```

### 3. 启动基础设施

```bash
docker-compose -f docker-compose.dev.yml up -d postgres redis
```

### 4. 验证

```bash
go build ./...
go test ./... -short
```

## 🔄 开发循环

```
1. 选任务
   └─ 在 GitHub Issues / Project 中找
   
2. 创建分支
   └─ git checkout -b feat/my-feature

3. 开发
   ├─ 写代码
   ├─ 写测试（TDD 优先）
   └─ 跑 lint

4. 验证
   ├─ go test ./... -race
   ├─ go test ./... -cover
   └─ pre-commit run --all-files

5. 提交
   └─ git commit -m "feat(scope): subject"

6. 推送 & 提 PR
   ├─ git push origin feat/my-feature
   └─ 在 GitHub 创建 PR

7. Code Review
   └─ 根据反馈修改

8. 合并
   └─ Maintainer 合并到 main
```

## 📁 项目结构

```
.
├── cmd/                  # 入口
│   ├── agentid/         # CLI
│   ├── migration-tool/  # 数据库迁移
│   └── mock-chain/      # 链上 mock
│
├── internal/            # 业务代码
│   ├── gateway/         # L5
│   ├── authz/           # L3 (aap, a2a, rbac, ...)
│   ├── service/         # L4
│   ├── domain/          # L2 (零第三方依赖!)
│   └── storage/         # L1
│
├── core/                # 跨服务核心
│   ├── backend/
│   └── chain_adapter/   # 链上适配
│
├── ent/                 # ORM schema
│
├── configs/             # 配置文件
│
├── docs/                # 文档
│
├── scripts/             # 工具脚本
│
├── test/                # e2e / load 测试
│
├── deploy/              # 部署配置
│   ├── docker/
│   └── helm/
│
└── .github/             # CI 配置
    └── workflows/
```

## ✍️ 编码规范

详见 [style.md](style.md)

## 🧪 测试要求

| 类型 | 位置 | 覆盖率 |
|------|------|--------|
| **单元** | `*_test.go` 与代码同目录 | ≥ 70% |
| **集成** | `internal/*/integration_test.go` | 关键路径 100% |
| **e2e** | `tests/e2e/` | 主要流程 |
| **性能** | `*_benchmark_test.go` | 关键热点 |

## 📦 提交规范

使用 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type

| Type | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | bug 修复 |
| `docs` | 仅文档 |
| `style` | 格式（不影响代码） |
| `refactor` | 重构（既非 feat 也非 fix） |
| `perf` | 性能优化 |
| `test` | 测试 |
| `chore` | 杂项（构建、CI、依赖） |
| `revert` | 回滚 |

### Scope

| Scope | 模块 |
|-------|------|
| `gateway` | L5 |
| `authz` | L3 |
| `service` | L4 |
| `domain` | L2 |
| `storage` | L1 |
| `chain` | 链上 |
| `cli` | CLI |
| `mcp` | MCP |
| `a2a` | A2A |
| `aap` | AAP |
| `rbac` | RBAC |
| `obs` | 可观测性 |
| `ci` | CI/CD |
| `docker` | Docker |
| `test` | 测试基础设施 |
| `security` | 安全 |
| `perf` | 性能 |
| `docs` | 文档 |
| `constitution` | 项目宪法 |

### 示例

```bash
git commit -m "feat(aap): add EdDSA challenge verification"
git commit -m "fix(storage): prevent duplicate chain_tx_hash in hybrid mode"
git commit -m "perf(cache): use Redis pipeline for batch MGet"
```

## 🔀 PR 流程

详见 [pr-process.md](pr-process.md)

## 🐛 报告 Bug

[GitHub Issues](https://github.com/agentid-chain/agentid-chain/issues/new?template=bug.md)

提供：
- 复现步骤
- 期望行为
- 实际行为
- 环境（OS / Go 版本 / 部署模式）
- 关键日志

## 💡 提交 Feature Request

[GitHub Issues](https://github.com/agentid-chain/agentid-chain/issues/new?template=feature.md)

描述：
- 解决的痛点
- 建议方案
- 替代方案
- 影响范围

## 📞 联系方式

- 邮件: dev@agentid-chain.example.com
- Slack: `#agentid-dev`
- 会议: 周三 10:00 (UTC+8)
