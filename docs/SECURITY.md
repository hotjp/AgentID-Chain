# Security Policy

> AgentID-Chain 安全策略与漏洞响应

## 🛡️ 支持的版本

| 版本 | 支持状态 |
|------|---------|
| 2.0.x | ✅ 当前稳定 |
| 1.5.x | ⚠️ 仅 critical 修复（EOL: 2026-12-31） |
| < 1.5 | ❌ 已停止支持 |

## 🔐 报告漏洞

**请勿**通过 GitHub Issues 公开披露安全漏洞。

请发送邮件至：**security@agentid-chain.example**

邮件应包含：

- 漏洞描述与影响
- 复现步骤 / PoC
- 受影响版本
- 您的联系信息（可选）

我们承诺：

- 24 小时内确认
- 7 天内评估严重度
- 30 天内修复 critical 漏洞
- 修复后公开致谢（如您同意）

## 🏆 漏洞奖励

严重等级与奖励（私下沟通）：

| 等级 | 示例 | 奖励 |
|------|------|------|
| Critical | RCE、未授权 root | $$$$ |
| High | AAP 绕过、SQL 注入 | $$$ |
| Medium | DoS、信息泄露 | $$ |
| Low | 文档错误、UI 缺陷 | $ |

## 🛡️ 安全设计原则

1. **零信任**：所有跨边界调用都需鉴权（AAP / mTLS / OAuth）
2. **纵深防御**：L3 鉴权 + L4 业务校验 + L1 约束
3. **Fail Fast**：发现攻击立即拒绝，不静默降级
4. **最小权限**：服务账户仅给必需权限
5. **密钥隔离**：EDD25519 私钥加密存储（KMS / Vault）
6. **审计完整**：所有写操作记录 audit_log

## 🔒 核心安全机制

### AAP（Agent-to-Agent Protocol）

- ED25519 签名
- 5 分钟时间戳窗口（防重放）
- Nonce 单次使用（Redis 跟踪）
- 公钥指纹绑定 agent（不可改）

详见 [api/aap.md](api/aap.md)

### 限流

- 滑动窗口（per IP / per agent）
- L3 前置关卡
- 异常模式自动熔断

### 审计

- 所有状态机变更写 audit_log
- 保留 ≥ 90 天
- 异常模式（短时间内大量失败）触发告警

### 密钥管理

- 私钥加密（AES-256-GCM + KMS master key）
- 公钥指纹索引（SHA-256 截断 16 字节）
- 定期轮换（90 天）
- 撤销即时生效（Redis pub/sub）

## 🧰 安全工具

```bash
# 静态扫描
gosec -severity=high ./...
govulncheck -mode=source ./...

# 依赖审计
go list -m -u all

# SBOM
syft . -o spdx-json=sbom.spdx.json

# 镜像签名
cosign sign --key cosign.key agentid-chain:v2.0.1
```

CI 强制门控见 [cmd/constitution-gates](../cmd/constitution-gates/)。

## 🔄 CVE 数据库

- [GitHub Security Advisories](https://github.com/agentid-chain/agentid-chain/security/advisories)
- 每月 1 号自动跑 `govulncheck` 报告

## 📜 合规

- 数据：GDPR 兼容（无 PII 必填）
- 加密：TLS 1.3 / ED25519
- 审计：SOC2 Type II（计划中）

## 📚 延伸阅读

- [AAP 协议](api/aap.md)
- [威胁模型](security/threat-model.md)（待补）
- [密钥管理](security/key-management.md)（待补）
- [事故响应](security/incident-response.md)（待补）
- [Operations 总览](OPERATIONS.md)
- [Runbook 列表](runbooks/)
