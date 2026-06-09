# AgentID-Chain — 项目治理

> Constitution + ADR + 季度评审 — 持续改进的飞轮

## 🏛️ 治理结构

```
┌─────────────────────────────────────────────┐
│           Constitution (宪法)                │
│   .long-run-agent/constitution.yaml          │
│   ──────────────────────────────────         │
│   定义不可妥协 / 强制 / 可配置规则           │
└──────────────────┬──────────────────────────┘
                   │
        ┌──────────┼──────────┐
        ↓          ↓          ↓
   ┌────────┐ ┌────────┐ ┌──────────┐
   │  ADR   │ │Quality │ │ Quarterly│
   │        │ │ Gates  │ │  Review  │
   │ 架构决 │ │  CI    │ │ 评估/    │
   │ 策记录 │ │ 检查   │ │ 演进     │
   └────────┘ └────────┘ └──────────┘
```

## 📜 Constitution 等级

| Type | 含义 | 可绕过 |
|------|------|--------|
| `NON_NEGOTIABLE` | 不可妥协 | ❌ |
| `MANDATORY` | 强制 | ⚠️ 例外需 Tech Lead 批准 |
| `CONFIGURABLE` | 可配置 | ✅ |
| `BEST_PRACTICE` | 最佳实践 | ✅ |

## 📋 Constitution 核心原则

### 不可妥协 (NON_NEGOTIABLE)

1. **交付物必须存在** — 任务声明的文件必须真实存在
2. **测试覆盖率 ≥ 70%** — 强制门禁
3. **5 层架构铁律** — L2 Domain 零第三方依赖
4. **链上操作私钥加密存储** — AES-256-GCM
5. **TLS 1.3** — 生产强制
6. **所有写操作有 audit** — 包括 reason

### 强制 (MANDATORY)

1. **AAP 鉴权先行** — 所有写操作前
2. **结构化日志** — JSON 格式 + 敏感字段脱敏
3. **Prometheus 指标** — HTTP/AAP/A2A/backend/cache
4. **OpenTelemetry trace** — 跨服务传播
5. **Conventional Commits** — feat/fix/docs/...
6. **CHANGELOG.md 更新** — 每个 PR

### 可配置 (CONFIGURABLE)

1. **限流阈值** — 随流量调整
2. **缓存 TTL** — 随业务调整
3. **存储后端** — local/onchain/hybrid
4. **认证算法** — 当前 EdDSA，未来可加 ECDSA

## 📐 治理流程

### 1. 提出变更

任何对 Constitution 的修订：

```bash
# 1. 创建 issue
# 标题: [constitution] 修改 XXX

# 2. 在 issue 中讨论
# - 现状
# - 改进建议
# - 影响范围
# - 替代方案

# 3. 达成共识 → PR 修改 constitution.yaml
```

### 2. ADR 流程

架构决策记录（Architecture Decision Record）：

```
1. 复制 templates/adr-template.md
2. 在 docs/architecture/adr/ 创建 NNNN-short-title.md
3. 提交 PR
4. Review 通过后合并
5. 在 README.md 添加索引
```

详见 [docs/architecture/adr/README.md](../docs/architecture/adr/README.md)

### 3. 季度评审

每季度评审 Constitution：

```
- 是否所有规则仍适用？
- 是否有新规则需要加入？
- 是否有规则应升级 / 降级？
- 哪些 ADR 已被 Superseded？
```

详见 [quarterly-review.md](quarterly-review.md)

## 🔄 持续改进飞轮

```
   实施
    ↓
  观察（指标 / 反馈）
    ↓
  评估（季度评审）
    ↓
  修订（Constitution / ADR）
    ↓
  实施
   ...
```

## 📊 指标

- **Constitution 违规数**（按类别）
- **ADR 平均 Review 时间**
- **季度评审完成率**
- **Contributor 反馈满意度**

## 🛠️ 工具

| 工具 | 用途 |
|------|------|
| `scripts/constitution-check.sh` | 自动验证 |
| `.long-run-agent/constitution.yaml` | 规则定义 |
| `docs/architecture/adr/` | 决策记录 |
| `quarterly-review.md` | 评审模板 |
| `.github/ISSUE_TEMPLATE/` | 反馈渠道 |

## 📚 相关

- [Constitution YAML](constitution.yaml)
- [ADR 索引](../docs/architecture/adr/README.md)
- [贡献者指南](../docs/contributing/development.md)
- [治理指南](../docs/contributing/governance.md)
