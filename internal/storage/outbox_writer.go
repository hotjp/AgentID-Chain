// Package storage Outbox 写入器（事务内同步发件箱）。
//
// 目标：
//  1. 业务事务内同时写业务表 + outbox_events
//  2. status=pending 由 poller 异步消费 → 转 downstream（Redis Stream）
//  3. 失败幂等：idempotency_key 唯一约束
//
// 设计：
//   - WriteOutbox(ctx, tx, evt) — tx 是 ent.Tx 句柄（抽象为 TxExecutor）
//   - 字段全部由调用方填充；写入器不生成 ID（保持"业务决定 ID"原则）
//   - 不在写入器做 schema 校验（ent 自身已保证）
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentid-chain/agentid-chain/ent"
)

// OutboxStatusPending 0=pending（待 poller 消费）。
const OutboxStatusPending uint8 = 0

// OutboxEvent 业务层 outbox 事件（独立于 ent 类型，避免泄漏 ent 到 service 层）。
type OutboxEvent struct {
	AggregateType  string         // 聚合根类型（agent / user / permission）
	AggregateID    string         // 聚合根 ID
	EventType      string         // 事件类型（agent.registered / agent.banned）
	Payload        map[string]any // 业务 JSON
	IdempotencyKey string         // 幂等键（建议 "<aggregate_type>:<event_type>:<aggregate_id>:<version>"）
}

// TxExecutor 事务执行器抽象（与 TxRunner 区分）。
//
// ent.Tx 满足此接口（其 OutboxEvent.Create 在事务内运行）。
type TxExecutor interface {
	OutboxEvent() *ent.OutboxEventClient
}

// entTxAdapter 把 *ent.Tx 适配为 TxExecutor。
//
// 实际业务层无需手动调用；ent.Tx 自身满足 TxExecutor。
type entTxAdapter struct {
	tx *ent.Tx
}

// OutboxEvent 返回 ent client 的 OutboxEvent 子 client。
func (a *entTxAdapter) OutboxEvent() *ent.OutboxEventClient {
	return a.tx.OutboxEvent
}

// FromEntTx 把 *ent.Tx 包成 TxExecutor。
func FromEntTx(tx *ent.Tx) TxExecutor {
	return &entTxAdapter{tx: tx}
}

// WriteOutbox 在事务内写一条 outbox_event。
//
// 调用方场景：
//
//	tx, _ := client.Tx(ctx)
//	defer tx.Commit()
//	storage.WriteOutbox(ctx, storage.FromEntTx(tx), storage.OutboxEvent{...})
//
// 失败模式：
//   - 字段校验失败 → ErrInvalidInput
//   - ent 写入失败 → 原样透传
//   - 唯一约束冲突（idempotency_key 重复）→ 原样透传
func WriteOutbox(ctx context.Context, exec TxExecutor, evt OutboxEvent) error {
	if err := validateOutboxEvent(evt); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	// payload 二次校验：marshal 必须成功（map 不会失败，但保留兜底）
	if _, err := json.Marshal(evt.Payload); err != nil {
		return fmt.Errorf("%w: payload marshal: %w", ErrInvalidInput, err)
	}

	now := time.Now().UTC()
	_, err := exec.OutboxEvent().Create().
		SetAggregateType(evt.AggregateType).
		SetAggregateID(evt.AggregateID).
		SetEventType(evt.EventType).
		SetPayload(evt.Payload).
		SetOccurredAt(now).
		SetIdempotencyKey(evt.IdempotencyKey).
		SetStatus(OutboxStatusPending).
		SetRetryCount(0).
		SetNextRetryAt(now).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("storage: outbox write: %w", err)
	}
	return nil
}

// validateOutboxEvent 字段校验。
func validateOutboxEvent(evt OutboxEvent) error {
	switch {
	case evt.AggregateType == "":
		return errors.New("aggregate_type is empty")
	case evt.AggregateID == "":
		return errors.New("aggregate_id is empty")
	case evt.EventType == "":
		return errors.New("event_type is empty")
	case evt.IdempotencyKey == "":
		return errors.New("idempotency_key is empty")
	case evt.Payload == nil:
		return errors.New("payload is nil")
	}
	return nil
}

// ErrInvalidInput 输入字段不合法。
var ErrInvalidInput = errors.New("storage: invalid input")
