---
name: release-notes
version: 2.0.1
role: system
description: 自动生成 release notes 模板
---

# Release Notes 生成

为新版本生成结构化的 release notes 文档。

## 输入

- 版本号（SemVer）
- Git range（如 v2.0.0..v2.1.0）
- 目标受众：用户 / 开发者 / 运维

## 模板

```markdown
# Release vX.Y.Z

**发布日期**：YYYY-MM-DD
**代号**：（可选，如 "Foundation" / "Velocity"）
**作者**：@name1, @name2

---

## 🎉 亮点 (Highlights)

3-5 个 bullet，1-2 句话，每个亮点突出用户价值：

- **OAuth 2.0 支持** — 第三方应用现可基于 OAuth 标准流程接入
- **批量注册性能 ×10** — 单次 1000 agent 注册从 30s 降至 3s
- **多链锚定** — 支持同时写入 Polygon + BSC

## ✨ 新功能 (New Features)

按用户影响排序：

### 功能 1：OAuth 2.0 集成
- 添加 `client_credentials` / `authorization_code` 流程
- 支持 PKCE（防截获）
- 新增 `/oauth/token` 端点
- 文档：[docs/api/oauth.md](docs/api/oauth.md)

### 功能 2：批量性能优化
- 改用 ent Eager Loading 消除 N+1
- 引入连接池扩展（25 → 100）
- 1k agent 注册：30s → 3s

## 🐛 修复 (Bug Fixes)

- 修复 UUIDv7 在 PostgreSQL 12 上的兼容性问题 (#456)
- 修复 AAP challenge 在时钟回拨时的失败 (#478)
- 修复 `list_agents` 分页越界 (#501)

## ⚠️ 破坏性变更 (Breaking Changes)

- `POST /v1/agents` 必传 `idempotency_key`，否则返回 400
- 移除 `legacy_signature_alg` 配置（迁移到 v2 算法）
- `metadata` 字段最大 16KB（之前 64KB）

详见 [MIGRATION.md](MIGRATION.md)

## 📈 改进 (Improvements)

- AAP 验证 P99 延迟 200ms → 80ms
- 错误信息增加 trace_id，便于排查
- Helm chart 升级到 1.2，支持 PDB
- Docker 镜像切换到 distroless，体积减少 60%

## 🔒 安全 (Security)

- gosec 0 high / govulncheck 0 vulns
- AAP nonce 改为 Redis 集群（HA）
- 添加 rate limit per owner（防滥用）

CVE：无

## 📦 依赖 (Dependencies)

| 包 | 旧 | 新 | 原因 |
|----|----|----|------|
| ent | 0.12.0 | 0.13.0 | bug fixes |
| pgx | 5.4.0 | 5.5.0 | security patch |
| go-redis | 9.4.0 | 9.5.0 | new features |

## 🛠️ 升级指南 (Upgrade Guide)

### 从 v2.0.x 升级

```bash
# 1. 备份
agentid backup full

# 2. 升级
helm upgrade agentid-chain deploy/helm/agentid-chain --version 2.1.0

# 3. 验证
go run ./cmd/constitution-gates
agentid health
```

### 数据库迁移

v2.0 → v2.1 需运行 `migrations/007_add_idempotency_key.up.sql`。
自动迁移：开启 `auto_migrate: true`；手动：DBA 执行。

## 📚 文档

- [更新文档](docs/) — 同步更新
- [迁移指南](MIGRATION.md)
- [API 变更](CHANGELOG.md)

## 🙏 致谢

贡献者（按 commit 数）：

- @alice (32)
- @bob (18)
- @carol (12)

外部依赖：

- ent 作者 @facebook
- pgx 作者 @jackc
- PostgreSQL 社区

## 📊 数据

- Issues closed: 47
- PRs merged: 89
- Contributors: 12
- Lines changed: +12,456 / -3,210
```

## 生成流程

1. **收集数据**：
   - `git log v2.0.0..v2.1.0 --oneline`
   - GitHub API: closed issues, merged PRs
   - `cloc` 统计代码变更

2. **分类**：
   - Conventional Commits 自动分类（feat/fix/breaking）
   - LLM 辅助归类长 commit

3. **起草**：
   - 按用户视角重写技术 commit
   - 突出影响和价值，不只罗列变更

4. **评审**：
   - 至少 2 人 review
   - PM / EM 终审"亮点"部分

5. **发布**：
   - 标记 GitHub Release
   - 发到 Slack #release
   - 通知邮件给客户
   - 更新官网 changelog

## 质量检查清单

- [ ] 亮点 ≤ 5 个
- [ ] 破坏性变更必标注 ⚠️
- [ ] 升级指南完整可执行
- [ ] 致谢所有贡献者
- [ ] 链接全部有效
- [ ] 无错别字 / 拼写错误
- [ ] 目标受众明确
