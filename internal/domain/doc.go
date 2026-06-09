// Package domain 是 AgentID-Chain 的 L2 领域层。
//
// 5 层架构约束（与 docs/architecture.md 一致）：
//   - L2 Domain 是依赖最纯净的一层
//   - **禁止 import 任何第三方包**（仅标准库）
//   - 仅定义实体、状态机、领域事件，不涉及任何技术实现细节
//   - 上层（L3/L4/L5）通过本包暴露的接口/类型进行依赖
//   - 下层（L1 Storage）的实现必须能够转换为本包的实体类型
//
// 包结构：
//
//	internal/domain/        ← 本包
//	├── doc.go              ← 本文件
//	├── agent.go            ← Agent 实体 + 状态机
//	├── state.go            ← AgentState 枚举 + 转换规则
//	├── event.go            ← 领域事件（注册/封禁/升级）
//	└── errors.go           ← 领域错误（与 L1 错误做映射）
//
// 设计目标：
//   - 业务语义集中：所有"什么是 Agent / Agent 怎么变"在这里定义
//   - 持久化无关：Agent 实体不依赖数据库/链
//   - 事件驱动：状态变化通过 DomainEvent 发布，由 L1 Outbox 转发
package domain
