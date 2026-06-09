# Rollback 流程

> AgentID-Chain 回滚操作手册

## 🛑 何时回滚

| 条件 | 严重度 | 动作 |
|------|--------|------|
| 错误率 > 5% 持续 5min | SEV1 | 立即回滚 |
| P99 延迟 > 2x SLO 持续 10min | SEV1 | 立即回滚 |
| 数据丢失 / 不一致 | SEV0 | 立即回滚 + 修复 |
| 错误率 1-5% | SEV2 | 评估后决定 |
| 功能问题（非阻塞） | SEV3 | 修复而非回滚 |

## ⚡ 快速回滚（应用层）

### Helm 回滚

```bash
# 1. 查看历史
helm history agentid-chain -n agentid

# 输出：
# REVISION  STATUS    CHART           APP VERSION  DESCRIPTION
# 1         failed    agentid-2.0.1   2.0.1        Install failed
# 2         superseded agentid-2.0.1   2.0.1        Upgrade complete
# 3         deployed  agentid-2.0.1   2.0.1        Upgrade complete
# 4         deployed  agentid-2.1.0   2.1.0        Upgrade complete  ← 当前

# 2. 回滚到上一版本
helm rollback agentid-chain 3 -n agentid

# 3. 验证
kubectl rollout status deployment/agentid-chain -n agentid
curl http://agentid.local:8080/healthz
```

### kubectl 回滚

```bash
# 查看历史
kubectl rollout history deployment/agentid-chain -n agentid

# 回滚
kubectl rollout undo deployment/agentid-chain -n agentid

# 回滚到指定版本
kubectl rollout undo deployment/agentid-chain --to-revision=3 -n agentid
```

## 🗄️ 数据库回滚

```bash
# 1. 备份当前状态（防止回滚出问题）
pg_dump $DATABASE_URL > /tmp/before-rollback.sql

# 2. 执行回滚
./scripts/migrate_v2.0.1_to_v2.1.0.sh --rollback

# 3. 验证
psql $DATABASE_URL -c "SELECT version FROM schema_version;"

# 4. 重启应用（新 schema 生效）
kubectl rollout restart deployment/agentid-chain -n agentid
```

## ⛓️ 链上数据回滚

AgentID-Chain 的链上写入是**不可变**的。无法"回滚"链上交易。

替代方案：

1. **撤销操作**：调用 `revoke_agent` 标记无效
2. **新交易覆盖**：写入新状态（旧状态保留历史）
3. **链重组**：仅 51% 攻击后由社区决策（罕见）

```bash
# 撤销 agent（不删数据，只改状态）
agentid revoke <agent_id> --reason "rollback from v2.1.0"
```

## 🔄 配置回滚

```bash
# 1. 查看 config 历史
agentid config history

# 2. 回滚到指定版本
agentid config rollback <revision>

# 3. 重启应用使配置生效
kubectl rollout restart deployment/agentid-chain -n agentid
```

## 🛡️ 回滚前检查清单

- [ ] **影响评估**：回滚会破坏什么？
- [ ] **数据兼容性**：旧版本能否读新版本写入的数据？
  - v2.0.1 ← v2.1.0：通常 OK（除非有 schema 变更）
  - v2.1.0 ← v2.0.1：⚠️ 不兼容（新增的 idempotency_key 列）
- [ ] **回滚窗口**：是否在维护窗口内？
- [ ] **通讯**：是否通知客户 / 团队？
- [ ] **监控**：是否有人在看监控？
- [ ] **决策人**：2 人同意（PM + Tech Lead）

## 🚨 回滚失败怎么办

1. **应用回滚失败**：
   - 手动 scale 旧版本：`kubectl scale deployment/agentid-chain --replicas=0 -n agentid`
   - 应用旧 manifest
   - 必要时降级到 2 个版本之前

2. **数据库回滚失败**：
   - **不要**再回滚（防止更多不一致）
   - 切到读模式：`agentid config set storage.read_only true`
   - 联系 DBA
   - 准备 restore 备份

3. **链上数据问题**：
   - 接受现状，记录 postmortem
   - 必要时联系链治理

## 📝 回滚后必做

- [ ] 监控 1 小时（错误率、延迟、链路）
- [ ] 关闭相关 incident
- [ ] 写 postmortem（48h 内）
- [ ] 更新 runbook
- [ ] 评估是否需要 hotfix 后重新发布

## 📚 相关文档

- [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md)
- [postmortem 模板](../templates/postmortem.md)
- [MIGRATION.md](MIGRATION.md)
- [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md)
