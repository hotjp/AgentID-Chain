# Troubleshooting Guide

> AgentID-Chain 故障排查入口

## 🔍 快速诊断

```bash
# 1. 进程是否运行
kubectl get pods -n agentid
agentid status

# 2. 健康检查
curl -i http://agentid.local:8080/live
curl -i http://agentid.local:8080/healthz

# 3. 看日志
agentid logs --tail=200 -f
# 或
kubectl logs -n agentid -l app=agentid-chain --tail=200

# 4. 看 metrics
curl -s http://agentid.local:9090/metrics | head

# 5. 跑质量门控（自动诊断）
go run ./cmd/constitution-gates
```

## 🐛 常见问题

### ❌ AAP 签名验证失败

**症状**：`401 UNAUTHORIZED` + `signature mismatch`

**原因**：
- 客户端时间偏差 > 5min
- 重放 nonce 已被使用
- 密钥不匹配（公钥绑定错误）

**修复**：
```bash
# 检查时间同步
chronyc tracking

# 清理 nonce 缓存
redis-cli DEL "aap:nonce:<hash>"

# 重置密钥
agentid aap keygen
```

详见 [runbooks/aap-replay-attack.md](runbooks/aap-replay-attack.md)

### ❌ DB 连接池耗尽

**症状**：`pq: sorry, too many clients already`

**修复**：
```bash
# 临时：调大 pool_size
agentid config set storage.pool_size 50

# 长期：检查慢查询
agentid db slow-queries
```

详见 [runbooks/db-connection-pool.md](runbooks/db-connection-pool.md)

### ❌ 链 RPC 失败

**症状**：`502 BAD_GATEWAY` + `chain rpc timeout`

**修复**：
```bash
# 切换 fallback RPC
agentid config set chain.fallback_url https://...

# 切到本地 mock
agentid config set chain.mode mock
```

详见 [runbooks/chain-rpc-failure.md](runbooks/chain-rpc-failure.md)

### ❌ 高错误率（>5%）

**症状**：5xx 错误率超过 SLO

**修复**：
```bash
# 1. 查看错误分类
agentid metrics --filter error_rate_by_status

# 2. 锁定错误源
agentid logs --filter level=error --since 10m

# 3. 必要时回滚
helm rollback agentid-chain
```

详见 [runbooks/high-error-rate.md](runbooks/high-error-rate.md)

### ❌ 磁盘空间不足

**症状**：Prometheus 告警 `DiskPressure`

**修复**：
```bash
# 清理旧日志
agentid logs prune --older-than 30d

# 清理旧 block
agentid chain prune --keep 1000000
```

详见 [runbooks/disk-pressure.md](runbooks/disk-pressure.md)

## 🧪 调试技巧

### 启用 debug 日志

```bash
agentid config set log.level debug
agentid restart
```

### 单次请求 trace

```bash
curl -H "X-Trace-Id: my-trace-001" \
  http://agentid.local:8080/v1/agents/abc
# 然后 grep 该 trace_id 的所有日志
```

### 本地复现

```bash
# 用 mock 链 + 内存 DB 复现
go test ./internal/service/... -run TestReproduce
```

## 📚 延伸阅读

- [Runbook 列表](runbooks/) — 5 个详细 runbook
- [Operations](OPERATIONS.md) — 运维总入口
- [Metrics](operations/metrics.md) — 可观测性
- [FAQ](guides/faq.md) — 常见问答
