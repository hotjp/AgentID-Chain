// Package domain Outbox 收集器（类型安全）。
//
// 业务场景：L4 Service 在事务内既要写业务表，也要发领域事件。
// 本包提供 Collect(ctx, exec, evt) 把领域事件转为 outbox_event 写入事务。
//
// 关键约束：
//   - 零第三方 import（domain 不能 import ent/storage）
//   - 类型安全：通过 DomainEvent 接口/struct 包装，不暴露 ent 类型
//   - 业务层调用形式：Collect(ctx, FromEntTx(tx), registeredEvt)
//
// 为什么不直接 import storage？
//   - domain 是"最纯净的层"；不依赖任何具体实现
//   - 通过 OutboxWriter 接口注入（接口隔离）
//   - 单元测试用 stub 替代；不依赖 ent/PG
package domain

import (
	"context"
	"errors"
	"fmt"
)

// OutboxWriter 事务内 outbox 写入接口（由 L1 Storage 实现）。
//
// domain 依赖此接口；具体实现由 storage.FromEntTx 适配。
type OutboxWriter interface {
	WriteOutboxEvent(ctx context.Context, evt OutboxEnvelope) error
}

// OutboxEnvelope 通用 outbox 信封（domain 视角）。
//
// OutboxEnvelope 把 domain 事件打包为 L1 outbox 需要的字段；
// 实际 outbox 表的 JSON 序列化由 OutboxWriter 内部完成。
type OutboxEnvelope struct {
	AggregateType  string                 // "agent"
	AggregateID    string                 // AgentUUID.String()
	EventType      string                 // event type
	Payload        map[string]interface{} // 业务 JSON
	IdempotencyKey string                 // 唯一幂等键
}

// FromDomainEvent 把任何领域事件转为 OutboxEnvelope。
//
// 用 Go 类型断言（type switch）分派到具体事件类型 → 构造 payload + idempotency_key。
func FromDomainEvent(evt interface{}) (OutboxEnvelope, error) {
	switch e := evt.(type) {
	case *AgentRegisteredEventV1:
		return envelopeFromRegistered(e)
	case *AgentUpgradedEventV1:
		return envelopeFromUpgraded(e)
	case *AgentBannedEventV1:
		return envelopeFromBanned(e)
	case *AgentRevokedEventV1:
		return envelopeFromRevoked(e)
	default:
		return OutboxEnvelope{}, fmt.Errorf("domain: unsupported event type %T", evt)
	}
}

// envelopeFromRegistered 注册事件 → envelope。
func envelopeFromRegistered(e *AgentRegisteredEventV1) (OutboxEnvelope, error) {
	if e == nil {
		return OutboxEnvelope{}, errors.New("domain: nil registered event")
	}
	return OutboxEnvelope{
		AggregateType: "agent",
		AggregateID:   e.AgentUUID.String(),
		EventType:     e.EventType,
		Payload: map[string]interface{}{
			"event_id":     e.EventID,
			"agent_uuid":   e.AgentUUID.String(),
			"owner_did":    e.OwnerDID,
			"level":        uint8(e.Level),
			"operator_did": e.OperatorDID,
			"occurred_at":  e.OccurredAt,
		},
		IdempotencyKey: e.AgentUUID.String() + ":registered:" + e.EventID,
	}, nil
}

// envelopeFromUpgraded 升级事件 → envelope。
func envelopeFromUpgraded(e *AgentUpgradedEventV1) (OutboxEnvelope, error) {
	if e == nil {
		return OutboxEnvelope{}, errors.New("domain: nil upgraded event")
	}
	return OutboxEnvelope{
		AggregateType: "agent",
		AggregateID:   e.AgentUUID.String(),
		EventType:     e.EventType,
		Payload: map[string]interface{}{
			"event_id":     e.EventID,
			"agent_uuid":   e.AgentUUID.String(),
			"old_level":    uint8(e.OldLevel),
			"new_level":    uint8(e.NewLevel),
			"old_perms":    e.OldPerms,
			"new_perms":    e.NewPerms,
			"reason":       e.Reason,
			"operator_did": e.OperatorDID,
			"occurred_at":  e.OccurredAt,
		},
		IdempotencyKey: e.AgentUUID.String() + ":upgraded:" + e.EventID,
	}, nil
}

// envelopeFromBanned 封禁事件 → envelope。
func envelopeFromBanned(e *AgentBannedEventV1) (OutboxEnvelope, error) {
	if e == nil {
		return OutboxEnvelope{}, errors.New("domain: nil banned event")
	}
	payload := map[string]interface{}{
		"event_id":     e.EventID,
		"agent_uuid":   e.AgentUUID.String(),
		"reason":       e.Reason,
		"operator_did": e.OperatorDID,
		"occurred_at":  e.OccurredAt,
	}
	if e.BanExpiresAt != nil {
		payload["ban_expires_at"] = *e.BanExpiresAt
	}
	return OutboxEnvelope{
		AggregateType:  "agent",
		AggregateID:    e.AgentUUID.String(),
		EventType:      e.EventType,
		Payload:        payload,
		IdempotencyKey: e.AgentUUID.String() + ":banned:" + e.EventID,
	}, nil
}

// envelopeFromRevoked 撤销事件 → envelope。
func envelopeFromRevoked(e *AgentRevokedEventV1) (OutboxEnvelope, error) {
	if e == nil {
		return OutboxEnvelope{}, errors.New("domain: nil revoked event")
	}
	return OutboxEnvelope{
		AggregateType: "agent",
		AggregateID:   e.AgentUUID.String(),
		EventType:     e.EventType,
		Payload: map[string]interface{}{
			"event_id":     e.EventID,
			"agent_uuid":   e.AgentUUID.String(),
			"reason":       e.Reason,
			"operator_did": e.OperatorDID,
			"occurred_at":  e.OccurredAt,
		},
		IdempotencyKey: e.AgentUUID.String() + ":revoked:" + e.EventID,
	}, nil
}

// Collect 把领域事件写入 outbox（事务内）。
//
// 调用方场景：
//
//	err := storage.InTransaction(ctx, runner, func(ctx context.Context, _ any) error {
//	    // 1. 写业务表
//	    // 2. 调 Collect
//	    return domain.Collect(ctx, storage.FromEntTx(tx), evt)
//	})
//
// Collect 内部做：
//  1. FromDomainEvent 类型转换（type switch）
//  2. exec.WriteOutboxEvent 写入
//
// 失败模式：writer 报错原样透传；事务会回滚。
func Collect(ctx context.Context, w OutboxWriter, evt interface{}) error {
	if w == nil {
		return errors.New("domain: outbox writer is nil")
	}
	envelope, err := FromDomainEvent(evt)
	if err != nil {
		return err
	}
	return w.WriteOutboxEvent(ctx, envelope)
}
