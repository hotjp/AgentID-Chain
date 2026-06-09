// Package storage Outbox Forwarder（XADD → Redis Stream）。
//
// 目标：
//  1. 作为 PollerHandler，把 outbox_event 写入 Redis Stream
//  2. 使用 XADD 写入 stream；stream 名为 "agentid:events"
//  3. 失败由 poller 负责 retry（指数退避已在 poller 内部处理）
//
// 设计：
//   - Forwarder 实现 PollerHandler 接口
//   - XADD * MAXLEN ~ 1000 (近似截断，避免 stream 无限增长)
//   - payload 走 JSON bytes
//
// 注意事项：
//   - Redis 不可用 → poller 会重试（视作业务错误）
//   - Stream key 在多处复用（建议常量 OutboxStreamKey）
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/agentid-chain/agentid-chain/ent"
	"github.com/redis/go-redis/v9"
)

// OutboxStreamKey 默认 stream 名称。
const OutboxStreamKey = "agentid:events"

// OutboxStreamMaxLen 近似截断上限（~1000 条；XADD MAXLEN ~ 性能更优）。
const OutboxStreamMaxLen int64 = 1000

// OutboxForwarder 把 outbox 事件 XADD 到 Redis Stream。
type OutboxForwarder struct {
	client *redis.Client
	stream string
	logger *slog.Logger
}

// NewOutboxForwarder 构造 Forwarder。
//
// stream 为空时使用 OutboxStreamKey。
func NewOutboxForwarder(client *redis.Client, stream string, logger *slog.Logger) *OutboxForwarder {
	if stream == "" {
		stream = OutboxStreamKey
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OutboxForwarder{client: client, stream: stream, logger: logger}
}

// HandleOutboxEvent 实现 PollerHandler。
//
// 把 evt 序列化为 stream 字段并 XADD；返回 error 时 poller 会重试。
func (f *OutboxForwarder) HandleOutboxEvent(ctx context.Context, evt *ent.OutboxEvent) error {
	if f.client == nil {
		return errors.New("outbox forwarder: redis client is nil")
	}
	if evt == nil {
		return errors.New("outbox forwarder: evt is nil")
	}

	// 序列化 payload
	payloadBytes, err := json.Marshal(evt.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// XADD 字段：固定 schema（id / aggregate_type / aggregate_id / event_type / payload / occurred_at / idempotency_key）
	values := map[string]any{
		"id":              evt.ID.String(),
		"aggregate_type":  evt.AggregateType,
		"aggregate_id":    evt.AggregateID,
		"event_type":      evt.EventType,
		"payload":         string(payloadBytes),
		"occurred_at":     evt.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000Z07:00"),
		"idempotency_key": evt.IdempotencyKey,
	}

	// XADD stream MAXLEN ~ 1000 * fields
	args := &redis.XAddArgs{
		Stream: f.stream,
		MaxLen: OutboxStreamMaxLen,
		Approx: true,
		Values: values,
	}
	id, err := f.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("xadd %s: %w", f.stream, err)
	}
	f.logger.Debug("outbox forwarded",
		slog.String("event_id", evt.ID.String()),
		slog.String("event_type", evt.EventType),
		slog.String("stream_id", id),
	)
	return nil
}
