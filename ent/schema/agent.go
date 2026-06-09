// Package schema Agent ent schema。
//
// Agent 是 AgentID-Chain 的核心实体：
//   - uuid: 全局唯一 ID（UUIDv7，时间排序，链上 / 链下统一表示）
//   - owner_did: 拥有者 DID（"did:agentid:user:xxx"）
//   - level: 等级 0-7（uint8；与 L4 Service 的等级体系对齐）
//   - permission: 权限位掩码（uint64；位运算判断 has/grant/revoke）
//   - status: 状态机（registered / active / banned / unregistered）
//
// 设计要点：
//   - uuid 用 UUIDv7（Default 走 google/uuid 的 New）— 时间排序索引友好
//   - permission 用 uint64 位掩码而非枚举数组：未来加权限位不需要 schema 变更
//   - status 用 uint8 而非 ent enum：跨语言 SDK 序列化更可控
//   - owner_did 用 string 而非外键：保留"链上 Agent"的可能性（owner 不一定是本地 user）
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Agent holds the schema definition for the Agent entity.
type Agent struct {
	ent.Schema
}

// Fields of the Agent.
func (Agent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Unique().
			Immutable().
			Comment("UUIDv7 主键；全局唯一（链上链下统一表示）"),

		field.String("owner_did").
			NotEmpty().
			MaxLen(256).
			Comment("拥有者 DID（did:agentid:user:xxx）；可指向本地 user 或链上身份"),

		field.Uint8("level").
			Range(0, 7).
			Default(0).
			Comment("等级 0-7；L4 Service 等级体系"),

		field.Uint64("permission").
			Default(0).
			Comment("权限位掩码；位运算判断 has/grant/revoke"),

		field.Uint8("status").
			Range(0, 3). // 0=registered, 1=active, 2=banned, 3=unregistered
			Default(0).
			Comment("状态机：0=registered, 1=active, 2=banned, 3=unregistered"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间（ent hook 自动维护）"),
	}
}

// Edges of the Agent.
//
// Agent ↔ AuditLog：1 个 Agent 对应 N 条审计日志（创建 / 升级 / 封禁 / 解封 ...）
// 关系在 P3.3 AuditLog schema 中通过 From("agent", Agent.Type) 反向声明。
func (Agent) Edges() []ent.Edge {
	return []ent.Edge{
		// audit_logs edge 由 AuditLog schema 的 From("agent", Agent.Type) 反向声明
	}
}

// Indexes of the Agent.
//
// 索引策略：
//   - owner_did_idx：按 owner 查询某人的所有 Agent
//   - status_idx：按状态过滤（如查 banned 列表）
func (Agent) Indexes() []ent.Index {
	return []ent.Index{
		// 注：id 已通过 Unique() 自动建索引，无需重复声明
		index.Fields("owner_did").
			StorageKey("idx_agent_owner_did"),
		index.Fields("status").
			StorageKey("idx_agent_status"),
	}
}
