# ADR-0001: 混合存储架构

## 状态

✅ Accepted (2026-01-15)

## 上下文

AgentID-Chain 需要存储 Agent 注册记录。需求包括：

1. **高频读**：业务调用方需要在毫秒级获取 Agent 详情
2. **不可篡改**：合规审计要求操作记录不可被篡改
3. **隐私**：Agent 与 owner 的关联关系不应完全公开
4. **跨组织**：需要第三方可独立验证 Agent 真实性

单一存储后端无法同时满足这些要求：

- **仅 PG**：性能好但可篡改、需信任 DB 管理员
- **仅链**：不可篡改但读慢（1-15s）、隐私差、Gas 成本高

## 决策

采用 **Hybrid 模式**：
- **读路径**：PG（毫秒级，含索引）
- **写主路径**：PG（同步写入，提供立即可读性）
- **写镜像路径**：链上（异步，1-15s 内确认）
- **审计读**：链上（通过 PG 存储的 `chain_tx_hash` 索引）

## 后果

### 正面
- ✅ 业务读性能达到 PG 水平（<10ms）
- ✅ 审计与跨组织验证可走链上
- ✅ 隐私数据（owner / metadata）不公开
- ✅ Gas 成本可控（按需镜像）

### 负面
- ❌ 复杂度高（需 worker 进程处理 outbox）
- ❌ 最终一致（链上 1-15s 延迟）
- ❌ 故障场景多（PG 宕、链 RPC 失败、worker 崩溃）

### 中性
- 🔄 需要 outbox 表 + 周期性扫描
- 🔄 需要监控 `chain_status` 字段
- 🔄 需要 chain RPC 失败重试机制

## 替代方案

| 方案 | 优点 | 缺点 | 否决理由 |
|------|------|------|---------|
| **仅 PG** | 简单 / 高性能 | 不可审计 / 需信任 DBA | 不满足合规 |
| **仅链** | 不可篡改 / 跨组织 | 读慢 / 隐私差 / Gas 贵 | 性能与隐私不可接受 |
| **PG + 周期快照到 IPFS** | 不可篡改 / 成本低 | 验证链路复杂 / IPFS 可用性 | 验证路径不直观 |
| **PG 主 + 链镜像**（已选） | 平衡 / 直观 | 复杂度 | ✅ |

## 实现细节

```
configs/app.yaml
storage:
  backend: hybrid
  hybrid:
    read_from: local
    write_to: local
    mirror_to: chain
    worker_interval: 5s
    max_retry: 3
```

`internal/storage/hybrid.go`：
- `Register()` 同步写 PG
- 入队 `outbox_event { type: "agent.registered", payload, chain_status: "pending" }`
- Worker 周期扫描 `chain_status="pending"`，推送到链
- 链上确认后更新 `chain_status="confirmed"` 和 `chain_tx_hash`

## 参考

- [Outbox Pattern](https://microservices.io/patterns/data/transactional-outbox.html)
- [架构规范](../../architecture.md)
