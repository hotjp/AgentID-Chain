// Package schema AuditLog ent schema。
//
// AuditLog 记录 Agent 生命周期内所有可审计动作（注册 / 升级 / 封禁 / 解封 / 注销）。
//
// 字段含义：
//   - id           UUIDv7 主键
//   - action       动作类型字符串（"register" / "upgrade" / "ban" / "unban" / "unregister"）
//   - reason       业务原因（可空；如"违反策略 X"）
//   - operator_did 操作者 DID（"did:agentid:user:admin123"）
//   - occurred_at  发生时间
//   - agent        关联 Agent（many-to-one via edge "agent"）
//
// 设计要点：
//   - 用 string 而非 enum 存 action：跨服务语义稳定，避免 schema 迁移
//   - reason 选 Optional：register / unregister 通常无原因
//   - operator_did 不强引用 user.id：操作者可能来自链上 / 跨服务（未来 A2A）
//   - agent edge 用 Required：审计日志必属某个 Agent（孤儿日志无业务意义）
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// AuditLog holds the schema definition for the AuditLog entity.
type AuditLog struct {
	ent.Schema
}

// Fields of the AuditLog.
func (AuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			Comment("UUIDv7 主键"),

		field.String("action").
			NotEmpty().
			MaxLen(64).
			Comment("动作类型：register/upgrade/ban/unban/unregister/..."),

		field.String("reason").
			Optional().
			MaxLen(1024).
			Comment("业务原因（可空）"),

		field.String("operator_did").
			NotEmpty().
			MaxLen(256).
			Comment("操作者 DID"),

		field.Time("occurred_at").
			Default(time.Now).
			Immutable().
			Comment("发生时间（DB 默认 now()）"),
	}
}

// Edges of the AuditLog.
//
// agent: many-to-one — 多个 audit log 属于同一个 agent
func (AuditLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("audit_logs").
			Unique().
			Required().
			Comment("归属 Agent（必填）"),
	}
}
