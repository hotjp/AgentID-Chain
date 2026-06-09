# Migration Guide

> AgentID-Chain 数据库 / 协议 / 配置迁移指南

## 🗄️ 数据库迁移

### CLI 方式

```bash
# 查看状态
agentid migrate status

# 升级到最新
agentid migrate up

# 降级 N 步
agentid migrate down 1

# 强制重做
agentid migrate force <version>
```

### 程序化（启动时自动）

`agentid` 启动时若 `auto_migrate: true` 则自动 apply pending migrations。
生产环境**推荐** `auto_migrate: false`，由 DBA 手动执行。

### 迁移文件结构

```
migrations/
├── 001_init.up.sql
├── 001_init.down.sql
├── 002_add_agents.up.sql
├── 002_add_agents.down.sql
└── ...
```

- 命名：`NNN_<name>.{up,down}.sql`（3 位 + snake_case）
- 版本号严格递增，不允许跳号
- 必含 `up` + `down` 一一对应

## 🔌 协议升级

### AAP 协议版本

| 版本 | 状态 | 特性 |
|------|------|------|
| v1 | ✅ stable | ED25519 签名 + nonce + timestamp |
| v2 | 🚧 draft | 增加 Dilithium 后量子支持 |

向后兼容策略：

- 服务端同时支持 v1 + v2
- 客户端可选升级
- v1 计划废弃（EOL：2027-01-01）

### MCP 协议版本

跟随官方 MCP spec。AgentID-Chain 兼容 MCP 2025-06-03。

## ⚙️ 配置升级

### v1.x → v2.x 关键变化

- `storage.mode` 新增 `hybrid` 选项
- 旧 `storage.chain_url` 重命名为 `storage.chain.primary_url`
- 移除 `legacy_signature_alg`

迁移工具：

```bash
agentid config migrate --from v1 --to v2
```

## 🔄 链切换

从 `local` 切到 `onchain`：

```yaml
# config.yaml
storage:
  mode: onchain
  chain:
    primary_url: https://polygon-rpc.com
    chain_id: 137
    contract: "0x..."
```

数据迁移：

```bash
agentid chain anchor --batch 1000
```

详见 [operations/migration.md](operations/migration.md)

## 🆙 版本升级流程

```bash
# 1. 备份
agentid backup full

# 2. 升级
helm upgrade agentid-chain deploy/helm/agentid-chain

# 3. 验证
agentid health
go run ./cmd/constitution-gates

# 4. 失败回滚
helm rollback agentid-chain
```

## 📚 详细文档

- [DB 迁移操作](operations/migration.md)
- [配置迁移](operations/configuration.md)
- [协议 ADR](architecture/adr/)
- [升级案例](operations/upgrade-case-studies.md)
