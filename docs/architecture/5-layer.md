# 5 层分层详解

> AgentID-Chain 的依赖铁律：L5 → L3 → L4 → L2 → L1

## 📐 分层定义

### L5 Gateway（网关层）

**职责**：
- TLS 终止（HSTS / mTLS）
- User-Agent 拦截（拒绝已知 bot / scraper）
- 路由（按 path / method 分发到 L3）
- 入口限流（IP 维度）

**禁止**：
- 业务逻辑
- 直接调用 L4 / L2
- 持有业务状态

**位置**：`internal/gateway/`

### L3 Authz（鉴权层）

**职责**：
- AAP 协议 Challenge-Response 验证
- MoltCaptcha 反向 CAPTCHA
- A2A Token 校验（含 Redis 撤销检查）
- RBAC 位掩码权限判断
- 入口限流（Agent 维度）

**Fail Fast**：任何一步失败立即返回 401/403

**位置**：`internal/authz/`

### L4 Service（服务编排层）

**职责**：
- Register / Upgrade / Revoke / Batch 工作流
- 事务边界控制
- 插件接口（PreRegister / PostUpgrade Hook）
- 调用 L2 完成业务逻辑

**禁止**：
- 直接 SQL（必须经 L2 → L1）
- 跨服务调用

**位置**：`internal/service/`

### L2 Domain（领域层）

**职责**：
- Agent 实体定义
- UUID 生成（v4 / v7）
- 状态机（Active / Suspended / Revoked / Expired）
- 领域事件发出
- 业务规则校验（如：level 升级路径）

**铁律**：**禁止 import 任何第三方包**（除 Go 标准库）
- 允许：`errors`, `context`, `time`, `crypto/sha256`
- 禁止：`database/sql`, `github.com/...`, `gRPC`

**位置**：`internal/domain/`

### L1 Storage（存储层）

**职责**：
- 实现 `Backend` 接口（Register / Get / Update / List）
- 适配 PostgreSQL（ent + pgx/v5）
- 适配链上（FISCO / Polygon / BSC / mock）
- 适配混合模式
- Redis 客户端（缓存 / nonce / 撤销 / 限流）

**位置**：`internal/storage/`, `core/chain_adapter/`

## 🔗 依赖图

```
                 ┌────────────┐
                 │  cmd/*     │  (入口)
                 └─────┬──────┘
                       ↓
              ┌────────────────┐
              │  L5 Gateway    │
              └───────┬────────┘
                      ↓
              ┌────────────────┐
              │  L3 Authz      │
              └───────┬────────┘
                      ↓
              ┌────────────────┐
              │  L4 Service    │
              └───────┬────────┘
                      ↓
              ┌────────────────┐
              │  L2 Domain     │  ← 零第三方依赖
              └───────┬────────┘
                      ↓
              ┌────────────────┐
              │  L1 Storage    │
              └────────────────┘
```

## ⚖️ 跨层检查清单

PR Review 时必须验证：

- [ ] L2 文件 `import` 块**不包含**任何 `github.com/` 或 `ent/`
- [ ] L3 不直接调用 L1（经 L4 → L2 → L1）
- [ ] L5 不含业务逻辑
- [ ] L4 含事务边界（`tx.Begin()` / `tx.Commit()`）
- [ ] 没有循环依赖

## 🛠️ 工具验证

```bash
# 检查 L2 是否纯净（应只 import 标准库）
go list -f '{{ join .Imports "\n" }}' ./internal/domain/... | grep -v '^[^/]*$' | grep '^github.com/' && echo "❌ L2 违反铁律" || echo "✅ L2 纯净"

# 检查依赖方向（应只有 L1 → L2 → L3 → L4 → L5 的方向）
go list -deps ./internal/gateway/... | sort -u
```

## 📚 历史

- v1.0：3 层（API / Business / Data）
- v2.0：5 层（Gateway / Authz / Service / Domain / Storage）

详见 [ADR-0001](../architecture.md) 关于分层的决策记录
