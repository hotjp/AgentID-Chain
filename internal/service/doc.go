// Package service 是 AgentID-Chain 的 L4 业务编排层。
//
// 5 层架构约束（与 docs/architecture.md 一致）：
//   - L4 Service 通过 interface 注入插件
//   - **禁止直接 import 任何插件包**（plugins/* / internal/captcha/* / internal/authz/*）
//   - 业务工作流：Register / Upgrade / Batch / Revoke / Unregister / RotateKey
//   - 事务边界由本层把控（一个用例一个事务）
//   - 异常映射：L1/L2 错误 → 业务错误（HTTP/gRPC code 标签）
//
// 包结构：
//
//	internal/service/         ← 本包
//	├── doc.go                ← 本文件
//	├── interfaces.go         ← 插件接口（ChainAdapter / CaptchaEngine / AuditNotifier）
//	├── errors.go             ← 业务错误（与 L1/L2 错误做映射）
//	├── agent_service.go      ← Register/Upgrade/Ban/Unregister 业务逻辑
//	├── batch_service.go      ← 批量操作
//	└── token_service.go      ← A2A Token 业务
//
// 设计目标：
//   - 上层 L5 调 L4；L4 调 L3（鉴权）+ L1（存储）+ L4 注入的接口（外延）
//   - 业务用例易于独立测试（依赖全部 interface 化）
package service
