# AgentID-Chain — 密钥轮转策略 (P17.11)

> 范围：`AGENTID_PRIVATE_KEY` / `AGENTID_DOMAIN_PRIVATE_KEY` / `AGENTID_A2A_PRIVATE_KEY` / `AGENTID_HMAC_SECRET` / `AGENTID_JWT_SIGNING_KEY` 等核心凭据

## 1. 轮转原则

1. **零停机**：双密钥并行运行，新旧密钥同时有效
2. **可回滚**：旧密钥保留 N 天（默认 30），可紧急回退
3. **审计**：所有轮转操作记录到审计日志
4. **自动化**：通过 CI + 调度任务（cron / k8s CronJob）触发
5. **告警**：异常轮转请求立即告警（P0 事件）

## 2. 密钥清单

| 密钥名 | 用途 | 长度 | 轮转周期 | 保留期 | 紧急回滚 |
|--------|------|------|---------|--------|---------|
| `AGENTID_PRIVATE_KEY` | Ed25519 节点身份签名 | 64B (ed25519) | 90 天 | 30 天 | 支持 |
| `AGENTID_DOMAIN_PRIVATE_KEY` | AAP 域密钥 | 32B (HMAC-SHA256) | 30 天 | 7 天 | 支持 |
| `AGENTID_A2A_PRIVATE_KEY` | A2A 跨链 Token | 64B (ed25519) | 90 天 | 30 天 | 支持 |
| `AGENTID_HMAC_SECRET` | 内部 HMAC | 32B | 30 天 | 7 天 | 支持 |
| `AGENTID_JWT_SIGNING_KEY` | JWT 签名（EdDSA） | 64B (ed25519) | 30 天 | 7 天 | 支持 |
| `AGENTID_DB_DSN` | PG 密码 | URL 内嵌 | 180 天 | 30 天 | 支持 |
| `AGENTID_REDIS_PASSWORD` | Redis 密码 | 任意 | 180 天 | 30 天 | 支持 |

## 3. 轮转流程（通用）

### 3.1 准备阶段（D-3）

```bash
# 1. 生成新密钥对
./scripts/secret-rotation.sh generate ed25519
./scripts/secret-rotation.sh generate hmac
# 2. 备份旧密钥到离线保险库（不写到 git）
./scripts/secret-rotation.sh backup --vault=./keys/backup-$(date +%Y%m%d).tar.gz.enc
# 3. 创建 git branch
git checkout -b chore/rotate-keys-$(date +%Y%m%d)
```

### 3.2 双写阶段（D-1）

```bash
# 4. 同时部署新旧密钥（节点识别 "key.<id>" 格式）
export AGENTID_PRIVATE_KEY_PRIMARY="$(cat keys/agentid.new.key)"
export AGENTID_PRIVATE_KEY_SECONDARY="$(cat keys/agentid.old.key)"
# 5. 重启节点（灰度：先 1/3 节点）
kubectl rollout restart deployment/agentid-gateway --record
# 6. 监控：错误率 < 0.1%
```

### 3.3 切换阶段（D+0）

```bash
# 7. 启用新密钥，禁用旧密钥
./scripts/secret-rotation.sh activate --key=agentid-private --new=keys/agentid.new.key
# 8. 重启所有节点
kubectl rollout restart deployment/agentid-gateway
# 9. 验证：新密钥签名的 token 可用
./scripts/secret-rotation.sh verify --key=agentid-private
```

### 3.4 回收阶段（D+7）

```bash
# 10. 确认无旧密钥引用
./scripts/secret-rotation.sh audit --window=7d
# 11. 清理旧密钥
./scripts/secret-rotation.sh purge --key=agentid-private --secondary
```

## 4. 双密钥并行的实现

### 4.1 KeySet 数据结构

```go
// internal/keystore/keyset.go
type KeySet struct {
    Primary   Key        // 当前主密钥
    Secondary Key        // 旧密钥（仅验证）
    ActivatedAt time.Time
    ExpiresAt   time.Time  // Secondary 失效时间
}

type Key struct {
    ID        string    // kid（JWT header）
    Algorithm string    // EdDSA | HS256
    Material  []byte    // 密钥材料
    CreatedAt time.Time
}
```

### 4.2 验证逻辑

```
验证 token：
  1. 从 JWT header 提取 kid
  2. 命中 Primary → 用 Primary 验签
  3. 命中 Secondary → 用 Secondary 验签（仅当 ExpiresAt > now）
  4. 未命中 → 401

签发 token：
  1. 始终用 Primary 签发
  2. 在 JWT header 写入 Primary.kid
```

### 4.3 配置示例

```yaml
# configs/keystore.yaml
keys:
  - name: agentid_private_key
    primary:
      kid: "2026-q2-agentid"
      path: /run/secrets/agentid/primary.key
    secondary:
      kid: "2026-q1-agentid"
      path: /run/secrets/agentid/secondary.key
      expires_at: "2026-07-09T00:00:00Z"
    rotation_policy:
      auto_rotate_days: 90
      notify_before_days: 7
```

## 5. 紧急回滚

### 5.1 触发条件

- 新密钥签发的 token 验证失败率 > 1%
- 签名/验签性能下降 > 50%
- 节点无法启动（密钥格式错误）
- 安全事件（密钥泄露）

### 5.2 回滚步骤

```bash
# 1. 立即将 Secondary 切换为 Primary
./scripts/secret-rotation.sh rollback --key=agentid-private
# 2. 批量重启
kubectl rollout restart deployment/agentid-gateway
# 3. 通知值班（PagerDuty / 钉钉）
./scripts/notify.sh "SECRET_ROLLBACK agentid-private @ $(date -u)"
# 4. 1 小时内完成根因分析
```

## 6. 自动化调度

### 6.1 k8s CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: secret-rotation-check
spec:
  schedule: "0 2 * * *"  # 每天凌晨 2 点检查
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: rotate
            image: agentid/cli:latest
            command: ["./scripts/secret-rotation.sh", "check"]
            env:
            - name: AGENTID_VAULT_ADDR
              valueFrom:
                secretKeyRef:
                  name: vault-addr
                  key: addr
```

### 6.2 检查项

```bash
# 密钥年龄
# 距离下次轮转天数
# Secondary 是否已过期
# 是否有未消费的旧 kid 引用（从 JWT 审计日志统计）
```

## 7. 审计日志

每次轮转必须记录：

```json
{
  "event": "secret.rotation",
  "key_name": "agentid_private_key",
  "old_kid": "2026-q1-agentid",
  "new_kid": "2026-q2-agentid",
  "operator": "alice@agentid-chain.io",
  "reason": "scheduled",
  "approved_by": "ops-team-lead",
  "timestamp": "2026-06-09T02:00:00Z",
  "old_expires_at": "2026-07-09T00:00:00Z"
}
```

## 8. 密钥存储

| 环境 | 存储方案 |
|------|---------|
| 本地开发 | `keys/*.pem`（gitignore） |
| CI | GitHub Actions Secrets |
| Staging | HashiCorp Vault |
| Production | HashiCorp Vault + AWS KMS auto-unseal |
| 备份 | 加密 tarball（age / gpg）存到 S3 Object Lock |

## 9. 监控告警

| 指标 | 告警阈值 |
|------|---------|
| `secret_age_days` | > 轮转周期 × 0.8 |
| `jwt_unknown_kid_total` | > 100/min |
| `secret_rotation_failed_total` | > 0 |
| `secondary_key_usage_ratio` | > 0.3（说明旧密钥引用过多） |

## 10. 检查清单

- [ ] 新密钥已生成并备份
- [ ] KeySet 配置已更新（Primary + Secondary + ExpiresAt）
- [ ] 灰度发布：先 1/3 节点
- [ ] 监控无异常（5 分钟观察期）
- [ ] 全量发布完成
- [ ] 7 天后执行 purge
- [ ] 审计日志已归档
- [ ] 值班通知已发送

## 11. 变更历史

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-06-09 | v2.0.1 | 初版（覆盖 7 类核心凭据） |
