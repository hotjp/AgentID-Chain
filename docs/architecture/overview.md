# 架构总览

> AgentID-Chain 的设计哲学、核心原则与模块边界

## 🎯 设计目标

1. **身份容量**：支持万亿级 Agent ID，UUID 层面无冲突
2. **多范式接入**：CLI / MCP / A2A / Prompt 四种方式同等地位
3. **混合存储**：本地 PG / 链上合约 / 二者结合，按场景切换
4. **可验证安全**：AAP (Challenge-Response) + MoltCaptcha (反向 CAPTCHA) 双闸
5. **可观测性**：OTel Trace + Prometheus Metrics + slog 默认开启

## 🏛️ 5 层架构

```
┌─────────────────────────────────────────────────────────────┐
│ L5  Gateway          (入口：UA 拦截 / 路由 / 限流)         │
│      ↑                                                    │
│ L3  Authz            (鉴权：AAP / MoltCaptcha / A2A / RBAC)│
│      ↑                                                    │
│ L4  Service          (编排：Register / Upgrade / Batch)    │
│      ↑                                                    │
│ L2  Domain           (领域：Agent 实体 / 状态机 / 事件)    │
│      ↑                                                    │
│ L1  Storage          (存储：Backend 接口 → PG / 链 / Redis)│
└─────────────────────────────────────────────────────────────┘
```

**铁律**：
- 严格自上而下依赖
- L2 Domain **禁止 import 任何第三方包**（除 Go 标准库）
- L1 Storage 是唯一允许 import ent / pgx / redis-go 的层
- 跨层跳跃是 **反模式**

详见 [5-layer.md](5-layer.md)

## 🔄 请求生命周期

```
Client → L5 Gateway
        ├─ UA 拦截
        ├─ TLS 终止（HSTS）
        └─ 路由到 L3
           ↓
L3 Authz
        ├─ AAP 验证（Challenge-Response）
        ├─ MoltCaptcha（高风险路径）
        ├─ A2A Token 校验
        ├─ RBAC 检查
        └─ 通过 → L4
           ↓
L4 Service
        ├─ Register / Upgrade / Revoke
        ├─ 事务边界控制
        └─ 调用 L2
           ↓
L2 Domain
        ├─ UUID 生成（v4 / v7）
        ├─ 状态机迁移
        └─ 发出领域事件
           ↓
L1 Storage
        ├─ ent ORM → PostgreSQL
        ├─ 链上调用（FISCO/Polygon/BSC）
        └─ Redis 缓存 / nonce / 撤销集
```

## 🗃️ 存储后端

| 后端 | 适用场景 | 写入延迟 | 持久性 |
|------|---------|---------|--------|
| `local` (PG) | 高频读、可信环境 | <10ms | 高 |
| `onchain` (链) | 不可篡改审计 | 1-15s | 不可篡改 |
| `hybrid` | 读 PG / 写链 | 写入 1-15s | 混合 |

详见 [storage.md](storage.md)

## 🔐 安全模型

- **准入**：AAP Challenge-Response（EdDSA）+ MoltCaptcha
- **会话**：A2A Token（EdDSA JWT）+ Redis 撤销集
- **传输**：TLS 1.3 + HSTS（生产强制）
- **审计**：所有写入链路发出领域事件，写入 outbox

## 📊 可观测性

| 信号 | 工具 | 导出 |
|------|------|------|
| Trace | OpenTelemetry | OTLP |
| Metrics | Prometheus client | `/metrics` (port 909x) |
| Logs | slog (JSON) | stdout/stderr |
| Profiles | pprof | `/debug/pprof` (port 606x) |

## 🔌 多协议接入

| 范式 | 协议 | 适用 |
|------|------|------|
| **CLI** | cobra 命令 | 运维 / 脚本 |
| **MCP** | JSON-RPC 2.0 | LLM 工具调用 |
| **A2A** | EdDSA JWT | Agent-to-Agent |
| **Prompt** | NL 解析 | 人类对话入口 |

## 📐 关键设计原则

1. **Fail Fast**：L3 鉴权失败立即拒绝，不下沉到 L4
2. **Outbox 模式**：领域事件先写 outbox，异步发布到外部系统
3. **Idempotency**：Register/Upgrade/Revoke 操作天然幂等（基于 UUID）
4. **Zero Trust**：内部调用也走 AAP / Token 校验
5. **Backpressure**：Redis 限流 + 队列长度监控

## 📚 深入阅读

- [5 层分层详解](5-layer.md)
- [存储后端](storage.md)
- [协议概览](protocols.md)
- [架构决策记录 (ADR)](adr/README.md)
