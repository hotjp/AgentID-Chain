// Package gateway 是 AgentID-Chain 的 L5 接入网关层。
//
// 5 层架构约束（与 docs/architecture.md 一致）：
//   - L5 Gateway 是唯一对外的入口
//   - 职责：TLS 终止 / 路由 / 中间件 / UA 拦截 / 限流前置 / AAP 握手入口
//   - 业务逻辑 0 行；只负责"转发 + 鉴权前置 + 协议转换"
//   - 通过 connect-go (connectrpc.com/connect) 暴露 HTTP/gRPC 入口
//   - 业务实现位于 internal/gateway/handler/*，调用 L4 Service
//
// 包结构：
//
//	internal/gateway/             ← 本包
//	├── doc.go                    ← 本文件
//	├── middleware/               ← 中间件（Recover/RequestID/Metrics/Logging/CORS/UA/AAP/APIKey）
//	├── router/                   ← 路由注册 & 拦截器装配
//	└── handler/                  ← connect-go handler 实现（直接调 L4）
//
// 中间件链顺序（与 docs §3.3 决策顺序对齐）：
//
//	1. Recover           panic 兜底
//	2. RequestID         注入 X-Request-ID
//	3. Logging           slog 结构化
//	4. Metrics           prometheus 自动
//	5. CORS              跨域
//	6. UA-Block          拦截恶意 UA
//	7. APIKey            静态 API Key 认证
//	8. AAP               AAP 协议握手
//	9. A2A-Token         A2A 互认 Token 校验
//	10. RateLimit        限流
//	11. RBAC             L3 RBAC 校验
//	12. Tracing          OTEL span
package gateway
