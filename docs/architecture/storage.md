# 存储后端

> 三种存储模式的设计原理、配置方式与适用场景

## 📋 模式概览

| 模式 | 写入路径 | 读路径 | 适用场景 |
|------|---------|--------|---------|
| `local` | PG | PG | 高频读、可信环境 |
| `onchain` | 链 | 链 + 事件索引 | 不可篡改审计 |
| `hybrid` | PG（主） + 链（镜像） | PG | 平衡性能与不可篡改 |

## 🏠 Local (PostgreSQL)

### 架构

```
Register → L4 → L2 → L1 (PG) → ent → pgx/v5 → PostgreSQL
```

### Schema（核心表）

- `agents` — 主表（uuid, owner, level, status, created_at, ...）
- `audit_logs` — 审计日志
- `outbox_events` — 领域事件 outbox
- `revocations` — 撤销记录（与 Redis 同步）

### 优势
- 写入延迟 < 10ms
- 完整 SQL 查询能力
- ACID 事务

### 劣势
- 中心化（需信任 DB 管理员）
- 不可独立审计链上

## ⛓️ Onchain

### 支持的链

| 链 | 状态 | 适配器 |
|---|------|-------|
| **FISCO BCOS** | 生产 | `core/chain_adapter/fisco` |
| **Polygon** | 生产 | `core/chain_adapter/polygon` |
| **BSC** | 试验 | `core/chain_adapter/bsc` |
| **mock** | 测试 | `core/chain_adapter/mock` |

### 智能合约接口

```solidity
interface IAgentIDRegistry {
    function register(bytes32 uuid, address owner, uint8 level) external;
    function upgrade(bytes32 uuid, uint8 newLevel) external;
    function revoke(bytes32 uuid, string reason) external;
    function getAgent(bytes32 uuid) external view returns (Agent memory);
    event Registered(bytes32 indexed uuid, address indexed owner, uint8 level);
    event Upgraded(bytes32 indexed uuid, uint8 newLevel);
    event Revoked(bytes32 indexed uuid, string reason);
}
```

### 优势
- 不可篡改
- 公开可验证
- 跨组织互信

### 劣势
- 写入延迟 1-15s（出块时间）
- Gas 成本
- 隐私（UUID 与 owner 公开）

## 🔀 Hybrid（推荐生产配置）

### 工作机制

```
Register
  ↓
PG 写入（同步，提供立即可读性）
  ↓
Outbox 事件入队
  ↓
异步 Worker 推送至链上
  ↓
链上确认后，更新 PG 的 `chain_tx_hash` 字段
```

### 读路径
- 业务读：**PG**（毫秒级）
- 审计读：**链上**（RPC 调用）
- 不可篡改验证：取 PG 中 `chain_tx_hash`，链上查询

### 一致性
- **最终一致**：PG 立即可读，链上 1-15s 后确认
- 失败回滚：链上失败不回滚 PG，但标记 `chain_status=failed`，周期性重试

### 优势
- 高读性能（PG 承担）
- 不可篡改（链上承担）
- 隐私（业务数据仅 PG）

### 劣势
- 复杂度高
- 需要 worker 进程

## 🔧 配置

`configs/app.yaml`：

```yaml
storage:
  backend: hybrid  # local | onchain | hybrid
  local:
    driver: postgres
    dsn: postgres://devuser:devpass@localhost:5432/agentid?sslmode=disable
  chain:
    type: polygon  # fisco | polygon | bsc | mock
    rpc: https://polygon-rpc.com
    contract: 0x1234...
    private_key: ${CHAIN_PRIVATE_KEY}
  hybrid:
    read_from: local  # 读路径
    write_to: local   # 写主路径
    mirror_to: chain  # 镜像目标
    worker_interval: 5s
```

## 🔍 故障与恢复

| 场景 | 影响 | 恢复 |
|------|------|------|
| PG 宕机 | 写入失败 | 主从切换 / PITR |
| 链上 RPC 失败 | hybrid 镜像延迟 | 自动重试 + 告警 |
| outbox worker 崩溃 | 链上同步停滞 | 重启 worker，自动从 checkpoint 恢复 |

## 📚 进一步阅读

- [ADR-0001: 混合存储架构](adr/0001-storage-hybrid.md)
- [故障 Runbook: 链上 RPC 失败](../runbooks/chain-rpc-failure.md)
- [性能: 慢查询监控](../perf/slow-query-monitoring.md)
