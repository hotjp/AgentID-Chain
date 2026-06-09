# 常见问题 (FAQ)

> 来自用户和开发者的常见疑问

## 📋 分类

- [通用](#通用)
- [AAP 鉴权](#aap-鉴权)
- [存储后端](#存储后端)
- [性能](#性能)
- [部署](#部署)
- [安全](#安全)

---

## 通用

### Q: AgentID-Chain 是什么？

A: 一个**AI Agent 分布式身份与权限网关**。提供：
- **身份**：万亿级 UUID 容量
- **鉴权**：AAP 准入协议 + MoltCaptcha
- **互信**：A2A Agent-to-Agent
- **存储**：混合（PG + 链上）
- **接入**：CLI / MCP / A2A / Prompt

### Q: 与传统 CA (证书颁发机构) 的区别？

| 维度 | 传统 CA | AgentID-Chain |
|------|--------|---------------|
| 颁发方 | 中心化 CA | 去中心化（AAP 验签） |
| 验证 | 证书链 | 一次性 challenge |
| 撤销 | CRL / OCSP | Redis Set (O(1) 查询) |
| 审计 | CA 自行 | 链上可独立验证 |
| 适用 | HTTPS / S/MIME | AI Agent |

### Q: 适用哪些场景？

- ✅ LLM 工具调用的鉴权（MCP 接入）
- ✅ Agent-to-Agent 微服务
- ✅ AI 工作流的身份管理
- ✅ 跨组织 Agent 互信
- ❌ 不适合：纯人类用户认证（用 OAuth/OIDC）

### Q: 容量上限？

- **UUID 空间**：2^122 ≈ 5.3 × 10^36（实际 128-bit 减去版本/变体）
- **DB 容量**：PG 横向扩展无上限
- **实际瓶颈**：PG 写吞吐（需读写分离 / 分库分表）

---

## AAP 鉴权

### Q: 为什么要先 AAP 鉴权？

A: 防止**匿名写操作**：
- 不验证私钥 → 任何人可注册任意 owner
- 验证后 → 注册行为可追溯到密钥对

### Q: JWT 有效期多长？

A: 默认 **1 小时**。可配置 `authz.aap.token_ttl`。
- 短（5min）：更安全，需要频繁刷新
- 长（24h）：用户体验好，泄露风险大
- 推荐：**配合 refresh token**（未来增强）

### Q: 私钥丢了怎么办？

A: **无法恢复**。AAP 的安全模型就是「私钥即身份」。
- 防御措施：备份私钥到 KMS / HSM
- 泄露后：用 `revoke` 主动撤销

### Q: 能用 RSA / ECDSA P-256 替代 Ed25519 吗？

A: 当前实现**仅支持 EdDSA Ed25519**（见 [ADR-0002](../architecture/adr/0002-aap-eddsa.md)）。
- 性能：Ed25519 53μs，ECDSA P-256 ~200μs
- 安全：同等 128-bit
- 如需其他算法，可作为插件

### Q: 能否用同一个密钥注册多个 Agent？

A: 可以。密钥对 = 身份 owner。一个 owner 可注册多个 Agent。

---

## 存储后端

### Q: local / onchain / hybrid 怎么选？

| 场景 | 推荐 |
|------|------|
| 内部系统，性能优先 | `local` |
| 公开服务，需审计 | `onchain` |
| 平衡（生产推荐） | `hybrid` |

详见 [architecture/storage.md](../architecture/storage.md)

### Q: 链上写入失败，PG 也回滚吗？

A: **不会**。hybrid 模式：
- PG 同步写入（立即可读）
- 链上异步镜像（失败标记 `chain_status=failed`）
- worker 周期性重试

这样保证**业务可用性**优先于**链上一致性**。

### Q: 如何确保 PG 和链上最终一致？

A: 通过 `chain_status` 字段 + 监控告警：
- `chain_status=pending` 累积 → 告警
- 重试后仍失败 → 人工介入
- 业务读永远走 PG（毫秒级），不阻塞

### Q: 数据保留多久？

A: 默认：
- `agents`：永久（业务核心）
- `audit_logs`：90 天
- `outbox_events`：30 天（确认后）

可配置 `retention.*` 字段。

---

## 性能

### Q: Register 延迟多少？

A: ~ 11μs（见 [register-benchmark.md](../perf/register-benchmark.md)）
- UUID 生成：205ns
- AAP verify：53μs（首次）
- PG insert：~3ms
- 链上镜像：1-15s（异步）

### Q: 缓存命中率低怎么办？

A: 排查步骤：
1. 看仪表板 [cache hit rate](../operations/metrics.md#缓存)
2. 检查 TTL 是否太短
3. 检查 key 设计是否合理
4. 检查 Redis 内存是否压力

详见 [redis-pipeline.md](../perf/redis-pipeline.md)

### Q: P99 延迟突然上升？

A: 见 [Runbook: 高错误率](../runbooks/high-error-rate.md)
- 检查慢查询（[slow-query-monitoring.md](../perf/slow-query-monitoring.md)）
- 检查连接池（[db-connection-pool.md](../runbooks/db-connection-pool.md)）
- 检查链上 RPC（[chain-rpc-failure.md](../runbooks/chain-rpc-failure.md)）

---

## 部署

### Q: 最小部署是什么？

A: 1 个 Go 进程 + PostgreSQL + Redis。
- 单实例足以支撑 1000+ RPS
- 适合 demo / 小规模

### Q: K8s 怎么部署？

A: 详见 [deployment.md#kubernetes](../operations/deployment.md#kubernetes)
- Helm chart: `deploy/helm/agentid-chain/`
- 3 副本起步，HPA 自动扩缩

### Q: 需要多少资源？

| 规模 | CPU | 内存 | 存储 |
|------|-----|------|------|
| 开发 | 100m | 256Mi | 1GB |
| 小型（< 100 RPS） | 500m | 512Mi | 10GB |
| 中型（< 1K RPS） | 1000m | 1Gi | 100GB |
| 大型（> 10K RPS） | 4000m | 4Gi | 1TB+ |

---

## 安全

### Q: 私钥如何存储？

A: 客户端：
- **服务端存储**（推荐）：私钥用 AES-256-GCM 加密后存 PG，密钥从 KMS 派生
- **本地存储**：仅 demo；生产禁止
- **HSM / KMS**：金融级场景

### Q: 传输层加密？

A: 强制 TLS 1.3（生产）。详见 [security_headers.go 配置](../operations/configuration.md#tls)

### Q: 如何轮换 JWT 签名密钥？

A: 详见 [SECRET_ROTATION.md](../SECRET_ROTATION.md)
- KeySet 双密钥并行（平滑切换）
- 旧密钥给宽限期

### Q: 怎么做安全审计？

A: 详见 [SECURITY_AUDIT.md](../SECURITY_AUDIT.md)
- OWASP API Top 10 检查
- CWE Top 25 检查
- 自动化：gosec / govulncheck 在 CI 中

---

## 🆘 仍有疑问？

- 📖 [文档中心](../README.md)
- 🐛 [提交 Issue](https://github.com/agentid-chain/agentid-chain/issues/new)
- 💬 [讨论区](https://github.com/agentid-chain/agentid-chain/discussions)
