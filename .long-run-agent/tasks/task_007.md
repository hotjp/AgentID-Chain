# task_007 — P5 L3 Authz 父任务

## 需求 (requirements)

- P5.1 - P5.18 全部完成
- L3 鉴权层 (authz) 五个子包齐备：rbac / aap / moltcaptcha / a2a / ratelimit
- 单元测试覆盖率 ≥70%（实际：rbac 92.8% / aap 92.1% / moltcaptcha 92.3% / a2a 96.7% / ratelimit 95.8%）
- 集成测试 (//go:build integration) 端到端跑通

## 验收标准 (acceptance)

- [x] internal/authz/rbac/ — RBAC 引擎 + 等级模板
- [x] internal/authz/aap/ — Challenge / Verify / Proof / Middleware / RateLimit
- [x] internal/authz/moltcaptcha/ — Challenge 生成 + 验证 + 语义匹配
- [x] internal/authz/a2a/ — Token 签发/验签 + TrustLevel + 撤销 + JWKS
- [x] internal/authz/ratelimit/ — 通用限流器
- [x] 12 个单元测试文件，覆盖率 90%+
- [x] 3 个集成测试文件（AAP/MoltCaptcha/A2A 端到端）

## 交付物 (deliverables)

- internal/authz/{rbac,aap,moltcaptcha,a2a,ratelimit}/

## 设计方案 (design)

L3 鉴权决策层，按 docs/architecture.md 规范：
- 前置关卡（Fail Fast）
- 依赖 L2 (domain) + 标准库 + 必要 L4 接口
- 不直接访问 L1 存储
