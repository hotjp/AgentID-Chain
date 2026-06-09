// Package schema OutboxEvent ent schema。
//
// OutboxEvent 实现"事务性发件箱"（Transactional Outbox）模式：
//  1. 业务事务内同时写业务表 + outbox_events 表
//  2. 单独的 poller 把 outbox_events 转发到下游（Redis Stream / Kafka / ...）
//  3. 失败时 status=pending，retry_count 自增，指数退避
//  4. 成功时 status=published，避免重发
//
// 字段含义（与 architecture.md §九 DSL 一致）：
//   - id                UUIDv7 主键
//   - aggregate_type    聚合根类型（"agent" / "user" / "permission"）
//   - aggregate_id      聚合根 ID
//   - event_type        事件类型（"agent.registered" / "agent.banned" ...）
//   - payload           事件 JSON
//   - occurred_at       发生时间
//   - idempotency_key   唯一幂等键（防 poller 重复发布）
//   - status            0=pending, 1=published, 2=failed, 3=dead
//   - retry_count       已重试次数
//   - last_error        最近一次失败原因
//   - next_retry_at     下次重试时间（用于轮询时的索引扫描）
//
// 设计要点：
//   - payload 走 JSONB 字段（PG 端）
//   - idempotency_key 走 unique 约束（DB 层防重复）
//   - next_retry_at 单独可索引：poller 走 "WHERE status=0 AND next_retry_at <= now()"
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// OutboxEvent holds the schema definition for the OutboxEvent entity.
type OutboxEvent struct {
	ent.Schema
}

// Fields of the OutboxEvent.
func (OutboxEvent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable().
			Comment("UUIDv7 主键"),

		field.String("aggregate_type").
			NotEmpty().
			MaxLen(64).
			Comment("聚合根类型：agent/user/permission/..."),

		field.String("aggregate_id").
			NotEmpty().
			MaxLen(64).
			Comment("聚合根 ID（字符串形式，兼容链上/链下 ID）"),

		field.String("event_type").
			NotEmpty().
			MaxLen(128).
			Comment("事件类型：agent.registered / agent.banned / ..."),

		field.JSON("payload", map[string]any{}).
			Comment("事件 JSON payload"),

		field.Time("occurred_at").
			Default(time.Now).
			Immutable().
			Comment("事件发生时间"),

		field.String("idempotency_key").
			NotEmpty().
			Unique().
			MaxLen(128).
			Comment("幂等键（unique 防重复发布）"),

		field.Uint8("status").
			Range(0, 3). // 0=pending, 1=published, 2=failed, 3=dead
			Default(0).
			Comment("发布状态：0=pending, 1=published, 2=failed, 3=dead"),

		field.Int("retry_count").
			Default(0).
			Comment("已重试次数"),

		field.String("last_error").
			Optional().
			MaxLen(1024).
			Comment("最近一次失败原因（可空）"),

		field.Time("next_retry_at").
			Default(time.Now).
			Comment("下次可重试时间（轮询索引扫描用）"),
	}
}

// Indexes of the OutboxEvent.
//
// 关键索引：
//   - idx_outbox_pending: poller 扫描 (status, next_retry_at) 复合索引
//   - idx_outbox_aggregate: 按聚合根查询历史事件
func (OutboxEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "next_retry_at").
			StorageKey("idx_outbox_pending"),
		index.Fields("aggregate_type", "aggregate_id").
			StorageKey("idx_outbox_aggregate"),
	}
}
