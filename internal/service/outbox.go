// Package service: Outbox emission 封装（P6.10）。
//
// 设计：在 L4 业务事务中插入 Outbox 事件，由独立 forwarder 异步投递给
// 链 / 审计 / 通知。Outbox 模式保证"业务写 + 事件写"原子性。
//
// 用法（与 Tx 配合）：
//
//	err := service.InTx(ctx, store, func(tx Tx) error {
//	    if err := tx.Identity().PutAgent(ctx, rec); err != nil { return err }
//	    return service.EmitOutbox(ctx, store, evt)  // 同事务内插 outbox 行
//	})
//
// EmitOutbox 写到 storage.OutboxStore.Insert；OutboxForwarder 独立
// goroutine 轮询投递（实现见 internal/storage/outbox_forwarder.go）。
package service

import (
	"context"
	"errors"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/storage"
)

// =============================================================================
// 事件类型
// =============================================================================

// OutboxEventType outbox 事件类型（v2.0.1 §6.4）。
const (
	OutboxEventRegister   = "agent.registered"
	OutboxEventUpgrade    = "agent.upgraded"
	OutboxEventBan        = "agent.banned"
	OutboxEventUnban      = "agent.unbanned"
	OutboxEventRevoke     = "agent.revoked"
	OutboxEventPermission = "agent.permission.changed"
)

// OutboxEvent outbox 载荷。
type OutboxEvent struct {
	ID         string
	Type       string
	UUID       string
	Payload    map[string]any
	OccurredAt time.Time
	// TryCount 重试次数（forwarder 维护）
	// NextRetry 下次重试时间
}

// OutboxStore outbox 存储（最小子集；真实 L1 实现会有完整 retry/state 字段）。
type OutboxStore interface {
	InsertOutbox(ctx context.Context, evt *OutboxEvent) error
}

// =============================================================================
// 发射
// =============================================================================

// ErrOutboxStoreUnavailable 存储后端无 outbox。
var ErrOutboxStoreUnavailable = errors.New("service: outbox store unavailable")

// EmitOutbox 发射事件（必须在 L1 事务内调用以保证原子性）。
//
// 存储约定：key = outbox:<event-id> → JSON；
// payload 序列化为 []byte 由 OutboxStore 负责。
func EmitOutbox(ctx context.Context, store storage.Client, evt *OutboxEvent) error {
	if evt == nil {
		return errors.New("service: nil outbox event")
	}
	if evt.ID == "" {
		return errors.New("service: empty outbox id")
	}
	if evt.Type == "" {
		return errors.New("service: empty outbox type")
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now()
	}

	// 检查 store 是否实现 OutboxStore（生产 PG 应该实现）
	if ob, ok := store.(OutboxStore); ok {
		return ob.InsertOutbox(ctx, evt)
	}
	// 退化：直接 No-op（warn by caller）
	return ErrOutboxStoreUnavailable
}

// MustEmitOutbox 发射失败时 panic（用于关键事件，不允许静默丢失）。
func MustEmitOutbox(ctx context.Context, store storage.Client, evt *OutboxEvent) {
	if err := EmitOutbox(ctx, store, evt); err != nil {
		panic("service: critical outbox emit failed: " + err.Error())
	}
}
