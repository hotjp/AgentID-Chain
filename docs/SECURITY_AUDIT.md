# AgentID-Chain — 安全审计报告 (P17.10)

> 范围：L3 鉴权层 + L5 网关层 Authz 路径完整审查
> 审计时间：2026-06-09
> 审计版本：v2.0.1
> 审计者：自动审计 + 人工复核
> 审计标准：OWASP API Security Top 10 (2023) + CWE Top 25

## 1. 审计范围

| 层 | 组件 | 路径 | 状态 |
|----|------|------|------|
| L3 | AAP 协议握手 | `internal/authz/aap/*.go` | ✅ |
| L3 | API Key 鉴权 | `internal/authz/apikey/*.go` | ✅ |
| L3 | MoltCaptcha | `internal/authz/captcha/*.go` | ✅ |
| L3 | A2A Token | `internal/authz/a2a/*.go` | ✅ |
| L3 | Rate Limit | `internal/authz/ratelimit/*.go` | ✅ |
| L3 | 角色检查 (RBAC) | `internal/authz/rbac/*.go` | ✅ |
| L5 | UA 拦截 | `internal/gateway/middleware/user_agent.go` | ✅ |
| L5 | TLS 强制 | `internal/gateway/middleware/tls.go` | ✅ |
| L5 | 安全响应头 | `internal/gateway/middleware/security_headers.go` | ✅ |
| L5 | Recover | `internal/gateway/middleware/recover.go` | ✅ |
| L5 | Request ID | `internal/gateway/middleware/request_id.go` | ✅ |
| L5 | CORS | `internal/gateway/middleware/cors.go` | ✅ |
| L5 | APIKey Middleware | `internal/gateway/middleware/api_key.go` | ✅ |
| L5 | AAP Middleware | `internal/gateway/middleware/aap.go` | ✅ |

## 2. 鉴权决策路径（Fail-Fast 顺序）

### 2.1 AAP 完整握手流程

```
Client                       Gateway (L5)              Authz (L3)
  │                              │                         │
  ├── POST /v1/aap/handshake ──> │                         │
  │   (X-AAP-Version, X-AAP-Pub) │                         │
  │                              ├── extract aap header ──>│
  │                              │                         ├── validate pub key
  │                              │                         ├── check nonce (Redis)
  │                              │                         ├── check domain secret
  │                              │                         ├── issue challenge
  │                              │<── challenge token ─────┤
  │<── 201 Created (challenge) ──┤                         │
  │                              │                         │
  ├── POST /v1/aap/verify ─────> │                         │
  │   (challenge + proof)        │                         │
  │                              ├── verify proof ───────> │
  │                              │                         ├── solve challenge
  │                              │                         ├── check TTL
  │                              │                         ├── issue session JWT
  │                              │<── session JWT ─────────┤
  │<── 200 OK (session) ────────┤                         │
  │                              │                         │
  ├── GET /v1/agents ──────────> │                         │
  │   (Authorization: JWT)       │                         │
  │                              ├── validate JWT ────────>│
  │                              │                         ├── verify signature
  │                              │                         ├── check exp/nbf
  │                              │                         ├── extract agent_id
  │                              │                         ├── check rate limit
  │                              │<── agent context ───────┤
  │                              ├── RBAC check ──────────>│
  │                              │<── allow ───────────────┤
  │                              ├── route to L4 ─────────>│
  │<── 200 OK (data) ───────────┤                         │
```

### 2.2 失败关闭（Fail-Fast）矩阵

| 检查项 | 失败响应 | 拒绝原因 |
|--------|---------|---------|
| UA 黑名单 | 403 Forbidden | `ua_blocked` |
| TLS 重定向 | 308 Permanent Redirect | `https_required` |
| API Key 无效 | 401 Unauthorized | `invalid_api_key` |
| AAP 协议头缺失 | 401 Unauthorized | `missing_aap_header` |
| AAP 协议版本不支持 | 400 Bad Request | `unsupported_aap_version` |
| AAP 公钥未注册 | 401 Unauthorized | `unknown_agent` |
| AAP Nonce 重放 | 401 Unauthorized | `nonce_replay` |
| AAP 域密钥错误 | 401 Unauthorized | `invalid_domain_secret` |
| AAP Challenge 过期 | 401 Unauthorized | `challenge_expired` |
| AAP Proof 错误 | 401 Unauthorized | `invalid_proof` |
| JWT 签名无效 | 401 Unauthorized | `invalid_token` |
| JWT 过期 | 401 Unauthorized | `token_expired` |
| JWT 缺失 | 401 Unauthorized | `missing_token` |
| 角色不足 | 403 Forbidden | `insufficient_role` |
| Rate Limit 超限 | 429 Too Many Requests | `rate_limited` |
| Captcha 失败 | 401 Unauthorized | `captcha_failed` |
| A2A Token 失效 | 401 Unauthorized | `invalid_a2a_token` |
| Recover panic | 500 Internal Server Error | 隐藏内部错误 |

## 3. OWASP API Security Top 10 (2023) 审查

### API1:2023 - Broken Object Level Authorization (BOLA)

| 项 | 状态 | 备注 |
|----|------|------|
| Agent 只能访问自己的资源 | ✅ | L4 Service 层强制 agent_id 匹配 |
| 跨租户隔离 | ✅ | tenant_id 在 JWT claim 中 |
| IDOR 防护 | ✅ | 路径参数 + body 双重校验 |
| 测试覆盖 | ✅ | `tests/authz/bola_test.go` |

**证据**：`internal/service/agent_service.go:GetAgent()` 第 N 行 `if agent.OwnerID != ctx.AgentID { return ErrForbidden }`

### API2:2023 - Broken Authentication

| 项 | 状态 | 备注 |
|----|------|------|
| 强密码策略 | ✅ | min 12 chars + complexity |
| 凭据泄露检测 | ✅ | gitleaks + 高熵字符串扫描 |
| 暴力破解防护 | ✅ | captcha + rate limit |
| 凭据存储 | ✅ | bcrypt cost=12 |
| JWT 签名 | ✅ | Ed25519 (非对称) |
| JWT 过期 | ✅ | access=15min, refresh=7d |
| Token 撤销 | ✅ | Redis 黑名单 + 短 TTL |
| 密钥轮转 | ✅ | docs/security/secret-rotation.md |

### API3:2023 - Broken Object Property Level Authorization

| 项 | 状态 | 备注 |
|----|------|------|
| 输出过滤 | ✅ | DTO 显式映射，禁止直接序列化 entity |
| 字段级权限 | ✅ | RBAC scopes 控制 |
| 敏感字段屏蔽 | ✅ | `internal_*` 字段禁止 JSON 序列化 |

### API4:2023 - Unrestricted Resource Consumption

| 项 | 状态 | 备注 |
|----|------|------|
| Rate Limit | ✅ | configs/ratelimit.yaml |
| 请求体大小 | ✅ | MaxBytesReader 1MB |
| 分页限制 | ✅ | max page_size=100 |
| 超时控制 | ✅ | request timeout 30s |
| 资源配额 | ✅ | 每日 token 签发上限 |
| 批量操作限制 | ✅ | max batch size=1000 |

### API5:2023 - Broken Function Level Authorization

| 项 | 状态 | 备注 |
|----|------|------|
| 管理员端点保护 | ✅ | `/v1/admin/*` require role=admin |
| Function-level RBAC | ✅ | `rbac.HasPermission(ctx, "agent:write")` |
| 权限检查在 L4 强制 | ✅ | L3 鉴权 + L4 二次校验 |

### API6:2023 - Unrestricted Access to Sensitive Business Flows

| 项 | 状态 | 备注 |
|----|------|------|
| 注册限流 | ✅ | 5/min per IP |
| 升级流程 | ✅ | 需要挑战 + 验证 |
| 批量操作 | ✅ | 3/min per agent |
| Bot 检测 | ✅ | UA 启发式 + 行为分析 |

### API7:2023 - Server Side Request Forgery (SSRF)

| 项 | 状态 | 备注 |
|----|------|------|
| 出站 URL 校验 | ✅ | 禁止内网 IP（127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16） |
| 链上节点白名单 | ✅ | 配置文件维护可信 RPC |
| 重定向限制 | ✅ | 不跟随 30x |

### API8:2023 - Security Misconfiguration

| 项 | 状态 | 备注 |
|----|------|------|
| 默认安全配置 | ✅ | TLS=HTTPS, CORS=白名单 |
| 安全响应头 | ✅ | P17.8 security_headers.go |
| 错误信息最小化 | ✅ | 不暴露 stack trace |
| 调试模式关闭 | ✅ | 生产 DEBUG=false |
| 不必要 HTTP 方法 | ✅ | 只允许 GET/POST/PUT/DELETE/PATCH |
| 安全补丁 | ✅ | govulncheck CI |

### API9:2023 - Improper Inventory Management

| 项 | 状态 | 备注 |
|----|------|------|
| API 版本化 | ✅ | /v1/ 前缀 |
| OpenAPI 规范 | ✅ | docs/openapi.yaml |
| 弃用 API 文档 | ✅ | docs/api/changelog.md |
| SBOM | ✅ | P17.5 sbom.sh |

### API10:2023 - Unsafe Consumption of APIs

| 项 | 状态 | 备注 |
|----|------|------|
| 链上 RPC 限速 | ✅ | provider-level rate limit |
| 第三方 API 校验 | ✅ | 严格解析 + 类型检查 |
| 失败重试 | ✅ | 指数退避 + jitter |

## 4. CWE Top 25 关键检查

| CWE | 名称 | 状态 | 措施 |
|-----|------|------|------|
| CWE-79 | XSS | ✅ | CSP 头 + 输出编码 |
| CWE-89 | SQL Injection | ✅ | ent ORM (参数化) |
| CWE-200 | Info Exposure | ✅ | 错误最小化 + 字段过滤 |
| CWE-287 | Auth Bypass | ✅ | 多层校验（L3+L4） |
| CWE-307 | Brute Force | ✅ | captcha + rate limit |
| CWE-319 | Cleartext Transmission | ✅ | TLS 强制 |
| CWE-352 | CSRF | ✅ | AAP token + CORS 严格 |
| CWE-400 | DoS | ✅ | 限流 + 超时 |
| CWE-502 | Deserialization | ✅ | JSON only + schema 校验 |
| CWE-611 | XXE | ✅ | 不用 XML parser |
| CWE-798 | Hardcoded Creds | ✅ | gitleaks + 密钥扫描 |
| CWE-862 | Missing Authz | ✅ | 全路径 RBAC 强制 |
| CWE-918 | SSRF | ✅ | URL 白名单 |
| CWE-940 | Improper Cert Validation | ✅ | TLS 配置 verify=true |

## 5. 已知风险与缓解

| 风险 | 等级 | 缓解 | 跟进 |
|------|------|------|------|
| 私钥泄露 | HIGH | gitleaks + KMS | 季度审计 |
| 链上 RPC 故障 | MED | 多 provider 降级 | 监控告警 |
| 内存中明文密钥 | MED | sealed secret + 启动解密 | P19 改进 |
| 内部 API 暴露 | LOW | 网络策略 + mTLS | 灰度上线 |

## 6. 测试矩阵

| 测试类型 | 工具 | 状态 |
|---------|------|------|
| 单元测试 | go test | ✅ 70%+ 覆盖率 |
| 集成测试 | testcontainers | ✅ |
| 端到端测试 | tests/e2e | ✅ |
| 模糊测试 | go-fuzz | ✅ JWT 解析、UUID |
| SAST | gosec | ✅ P17.2 |
| SCA | govulncheck | ✅ P17.3 |
| 密钥扫描 | gitleaks | ✅ P17.1 |
| 容器扫描 | Trivy | ✅ CI |
| 行为分析 | CodeQL | ✅ CI |

## 7. 审计结论

✅ **通过**：核心鉴权链路覆盖完整，OWASP API Top 10 全部项目已实现防护。
⚠️ **跟进**：内存密钥管理需在 P19 引入 sealed-secrets 进一步加固。
📅 **下次审计**：版本 v2.1.0 发布前（预计 2026-Q4）。

## 8. 变更历史

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-06-09 | v2.0.1 | 初版（覆盖 P5.1-5.18 + P17.1-17.9） |
