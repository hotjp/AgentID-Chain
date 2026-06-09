// Package authz 是 AgentID-Chain 的 L3 鉴权决策层。
//
// 5 层架构约束（与 docs/architecture.md 一致）：
//   - L3 Authz 是"前置关卡"（Fail Fast）
//   - 接收 L5 网关透传的用户上下文（Subject / Token / Challenge）
//   - 允许 import：L2 (domain) + 标准库 + 必要的 L4 接口
//   - 不直接访问 L1 存储（必须通过 L4 Service 转发）
//
// 包结构：
//
//	internal/authz/                ← 本包
//	├── doc.go                     ← 本文件
//	├── rbac/                      ← 基于位掩码的 RBAC 校验（P3.5）
//	├── aap/                       ← AAP 协议 (Challenge-Response + EdDSA) (P3.6)
//	├── moltcaptcha/               ← MoltCaptcha SMHL 反向 CAPTCHA（P3.7）
//	└── ratelimit/                 ← 限流器（P6 接入）
//
// 决策顺序（与 docs §3.3 一致）：
//
//	┌─ UA / IP 黑名单
//	├─ API Key / mTLS 认证
//	├─ AAP Challenge 验证（首次握手）
//	├─ A2A Token 验证（互认调用）
//	├─ RBAC 位掩码校验
//	└─ Rate Limit
//
// 任何一步失败立即拒绝（Fail Fast），返回 4xx 与标准错误码。
package authz
