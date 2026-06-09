# Runbook: 链上 RPC 失败

## 严重度
**P1** — 30min 内 ack

## 触发告警
- `ChainHighFailureRate` — chain RPC 失败 > 10% (5min)
- `ChainRPCLatency` — chain P99 > 5s (5min)

## 症状
- `hybrid` 模式下：链上镜像失败，`chain_status=pending` 累积
- `onchain` 模式下：Register / Upgrade 失败
- 业务可能仍可用（PG 读路径），但审计验证受影响

## 立即行动（5min 内）

1. **确认范围**
   ```bash
   # 链上 RPC 指标
   curl -s http://localhost:9090/metrics | grep "^backend_requests_total{type=\"chain\""
   ```

2. **测试 RPC 节点**
   ```bash
   # Polygon
   curl -X POST -H "Content-Type: application/json" \
     --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
     https://polygon-rpc.com

   # 期望：返回 block number
   ```

3. **查看 outbox 堆积**
   ```bash
   psql "$POSTGRES_DSN" -c "SELECT count(*), chain_status FROM outbox_events GROUP BY chain_status"
   ```

## 诊断

### 1. RPC 节点状态

| 节点 | 状态页 | 说明 |
|------|--------|------|
| Polygon | https://polygonstatus.com/ | 主网 RPC 健康 |
| Polygon 公共 | https://polygon-rpc.com | 公共节点（限速） |
| Infura | https://status.infura.io/ | 第三方托管 |
| Alchemy | https://status.alchemy.com/ | 第三方托管 |

### 2. 钱包 / 账户问题

```bash
# 余额
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getBalance","params":["<your_address>","latest"],"id":1}' \
  https://polygon-rpc.com

# 期望：返回 wei 数
```

### 3. Gas 价格

```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_gasPrice","params":[],"id":1}' \
  https://polygon-rpc.com
```

如果 gas 价格极高（> 500 gwei），考虑：
- 等待 gas 回落
- 提高 `gas_price_gwei` 配置

### 4. 合约错误

```bash
# 模拟调用
curl -X POST -H "Content-Type: application/json" \
  --data '{
    "jsonrpc":"2.0",
    "method":"eth_call",
    "params":[{
      "to":"<contract>",
      "data":"<encoded_function_call>"
    },"latest"],
    "id":1
  }' https://polygon-rpc.com
```

如果返回 revert，看 revert reason（通常以 `0x08c379a0` 开头）。

## 缓解（短期）

1. **切换 RPC 节点**
   ```yaml
   chain:
     rpc: https://polygon-mainnet.g.alchemy.com/v2/<key>  # 切换到 Alchemy
   ```

2. **切换到 mock 链**（仅测试）
   ```yaml
   chain:
     type: mock
   ```

3. **暂停链上镜像**（业务不受影响）
   ```yaml
   hybrid:
     mirror_to: ""  # 关闭
   ```

4. **手动重试 outbox**
   ```bash
   go run ./cmd/agentid chain retry
   ```

## 修复（根本）

### 1. RPC 节点选型

| 方案 | 优点 | 缺点 |
|------|------|------|
| 公共节点 | 免费 | 限速、不稳定 |
| 自建节点 | 高可用、可控 | 运维成本 |
| 第三方托管 (Alchemy/Infura) | SLA、稳定 | 付费 |

生产建议：**多 RPC 备份 + 自动切换**

### 2. 多 RPC 配置（未来增强）

```yaml
chain:
  rpc_endpoints:
    - https://polygon-mainnet.g.alchemy.com/v2/key1
    - https://polygon-mainnet.infura.io/v3/key2
    - https://polygon-rpc.com  # 公共，兜底
  rpc_strategy: failover  # failover | round-robin
```

### 3. outbox worker 优化

```yaml
hybrid:
  worker_interval: 5s
  worker_batch_size: 50
  max_retry: 5
  retry_backoff: 1m, 5m, 30m, 2h, 12h
```

## 验证

- [ ] `ChainHighFailureRate` 告警恢复
- [ ] `outbox_events` 中 `chain_status=pending` 数量稳定下降
- [ ] 业务读写延迟恢复
- [ ] （可选）手动注册测试 Agent 验证链上确认

## 📚 相关

- [architecture/storage.md](../architecture/storage.md)
- [ADR-0001: 混合存储](../architecture/adr/0001-storage-hybrid.md)
- [operations/troubleshooting.md](../operations/troubleshooting.md)
