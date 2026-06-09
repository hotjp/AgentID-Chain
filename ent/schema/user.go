// Package schema 定义 ent ORM 的全部 schema。
//
// User 表示 AgentID-Chain 系统中的"用户"（Agent 的所有者 / 操作者）。
//
// 字段含义：
//   - id        UUIDv7 主键（时间排序，全局唯一）
//   - email     唯一邮箱（登录 / 找回 / 通知）
//   - role_did  角色 DID（"did:agentid:role:admin" / "did:agentid:role:user"）
//   - created_at / updated_at 标准审计字段
//
// 设计要点：
//   - email 走 unique 约束（DB 层 + ent 层双重防护）
//   - role_did 用 string 而非 enum：DID 是开放命名空间，未来加新角色不需 schema 迁移
//   - updated_at 通过 ent.Hook 自动维护（见 ent/hook/user_hook.go）
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			Comment("UUIDv7 主键；时间排序、全局唯一"),

		field.String("email").
			NotEmpty().
			Unique().
			MaxLen(320). // RFC 5321 max email length
			Comment("用户邮箱；唯一约束，登录/找回/通知"),

		field.String("role_did").
			NotEmpty().
			MaxLen(256).
			Comment("角色 DID（did:agentid:role:admin / user 等）"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间（DB 默认 now()，由 ent 维护）"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间（ent hook 自动维护）"),
	}
}

// Edges of the User.
//
// User 与 Agent 是 1:N 关系：一个 User 可拥有多个 Agent。
// 反向边将在 P3.2 Agent schema 中通过 edges.From("owner", User.Type) 定义。
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// owned_agents edge 由 Agent schema 的 From("owner", User.Type) 反向声明
	}
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		// 预留：审计 mixin、soft-delete mixin 等
	}
}
