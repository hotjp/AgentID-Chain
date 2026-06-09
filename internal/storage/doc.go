// Package storage 提供 AgentID-Chain 的 L1 存储抽象层。
//
// 5 层架构约束（与 docs/architecture.md 一致）：
//   - L1 Storage 依赖 L2 Domain 的实体类型
//   - 禁止 L4 Service 直接 import 任何具体实现（必须通过 interface）
//   - 允许 import 的依赖：标准库、ent、pgx/v5、go-redis/v9
//
// 包结构：
//
//	internal/storage/        ← 本包
//	├── client.go             ← StorageClient 统一接口
//	├── doc.go                ← 本文件
//	├── postgres/             ← PostgreSQL 实现 (P3.1 接入)
//	├── redis/                ← Redis 实现 (P3.2 接入)
//	├── outbox/               ← Outbox 模式转发 (P3.3 接入)
//	└── audit/                ← 审计日志 (P3.4 接入)
//
// 设计目标：
//   - 业务层（L2-L5）不感知后端类型
//   - 多实现可插拔（PG/Redis/链上 mock）
//   - 事务边界清晰
package storage
