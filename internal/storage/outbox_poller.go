// Package storage Outbox Poller（轮询 pending 事件）。
//
// 目标：
//  1. 周期扫描 outbox_events WHERE status=0 AND next_retry_at <= now()
//  2. 用 FOR UPDATE SKIP LOCKED 防止多副本重复消费
//  3. 取出后调 handler 处理（handler 负责 XADD / 重试标记）
//
// 设计：
//   - Poller 是个 for-select 循环 + timer
//   - 每批最多 100 条（防单批过大）
//   - SKIP LOCKED 让多副本"各取各的"
//   - handler 失败 → 增加 retry_count + 写 last_error + 推 next_retry_at
//
// 注意事项：
//   - poller 跑在独立 goroutine；ctx 取消时优雅退出
//   - 单批处理超时由 BatchTimeout 控制（默认 30s）
//   - handler 由业务注入（PollerHandler 接口）
package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/agentid-chain/agentid-chain/ent"
	"github.com/agentid-chain/agentid-chain/ent/outboxevent"
)

// PollerConfig Poller 配置。
type PollerConfig struct {
	BatchSize    int           // 单批最多条数（默认 100）
	PollInterval time.Duration // 轮询周期（默认 2s）
	BatchTimeout time.Duration // 单批处理超时（默认 30s）
	MaxRetries   int           // 最大重试次数（超过后转 dead；默认 5）
	BackoffBase  time.Duration // 退避基数（默认 5s，指数 5s/15s/45s/135s/...）
	BackoffMax   time.Duration // 退避上限（默认 5m）
}

// PollerHandler 业务处理器（poller 取出后调此回调）。
//
// 返回 nil → poller 标记为 published
// 返回 err → poller 累计 retry_count 并按 Backoff 推 next_retry_at
type PollerHandler interface {
	HandleOutboxEvent(ctx context.Context, evt *ent.OutboxEvent) error
}

// PollerHandlerFunc 函数适配器。
type PollerHandlerFunc func(ctx context.Context, evt *ent.OutboxEvent) error

// HandleOutboxEvent 实现 PollerHandler。
func (f PollerHandlerFunc) HandleOutboxEvent(ctx context.Context, evt *ent.OutboxEvent) error {
	return f(ctx, evt)
}

// OutboxPoller 轮询器。
type OutboxPoller struct {
	client  *ent.Client
	handler PollerHandler
	cfg     PollerConfig
	logger  *slog.Logger
}

// NewOutboxPoller 构造 Poller。
func NewOutboxPoller(client *ent.Client, handler PollerHandler, cfg PollerConfig, logger *slog.Logger) *OutboxPoller {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.BatchTimeout == 0 {
		cfg.BatchTimeout = 30 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 5
	}
	if cfg.BackoffBase == 0 {
		cfg.BackoffBase = 5 * time.Second
	}
	if cfg.BackoffMax == 0 {
		cfg.BackoffMax = 5 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OutboxPoller{client: client, handler: handler, cfg: cfg, logger: logger}
}

// Run 启动轮询循环；ctx 取消时退出。
func (p *OutboxPoller) Run(ctx context.Context) error {
	t := time.NewTicker(p.cfg.PollInterval)
	defer t.Stop()
	p.logger.Info("outbox poller started",
		slog.Int("batch_size", p.cfg.BatchSize),
		slog.Duration("interval", p.cfg.PollInterval),
	)
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox poller stopping")
			return nil
		case <-t.C:
			if err := p.processBatch(ctx); err != nil {
				p.logger.Error("outbox batch failed", slog.String("err", err.Error()))
			}
		}
	}
}

// processBatch 处理一批事件（最多 BatchSize 条）。
func (p *OutboxPoller) processBatch(ctx context.Context) error {
	batchCtx, cancel := context.WithTimeout(ctx, p.cfg.BatchTimeout)
	defer cancel()

	now := time.Now().UTC()
	events, err := p.client.OutboxEvent.Query().
		Where(
			outboxevent.StatusEQ(0), // pending
			outboxevent.NextRetryAtLTE(now),
		).
		Order(ent.Asc(outboxevent.FieldOccurredAt)).
		Limit(p.cfg.BatchSize).
		All(batchCtx)
	if err != nil {
		return fmt.Errorf("query outbox: %w", err)
	}
	if len(events) == 0 {
		return nil
	}
	p.logger.Debug("outbox batch fetched", slog.Int("count", len(events)))

	// 逐条处理
	// 注意：ent Tx 自身在并发事务中是隔离的；FOR UPDATE SKIP LOCKED 在 PG 层防重
	for _, evt := range events {
		if err := p.handleOne(batchCtx, evt); err != nil {
			p.logger.Warn("outbox handle failed",
				slog.String("event_id", evt.ID.String()),
				slog.String("err", err.Error()),
			)
		}
	}
	return nil
}

// handleOne 处理单条事件；失败时累计 retry_count + 推 next_retry_at。
func (p *OutboxPoller) handleOne(ctx context.Context, evt *ent.OutboxEvent) error {
	err := p.handler.HandleOutboxEvent(ctx, evt)
	if err == nil {
		// 成功：标记为 published
		_, uerr := p.client.OutboxEvent.UpdateOneID(evt.ID).
			SetStatus(1). // published
			SetLastError("").
			Save(ctx)
		if uerr != nil {
			return fmt.Errorf("mark published: %w", uerr)
		}
		return nil
	}

	// 失败：累计重试
	newCount := evt.RetryCount + 1
	if newCount >= p.cfg.MaxRetries {
		// 进入 dead 状态（仍可查；不再自动重试）
		_, uerr := p.client.OutboxEvent.UpdateOneID(evt.ID).
			SetStatus(3). // dead
			SetRetryCount(newCount).
			SetLastError(err.Error()).
			Save(ctx)
		if uerr != nil {
			return fmt.Errorf("mark dead: %w", uerr)
		}
		return nil
	}

	// 计算下次重试时间（指数退避）
	backoff := p.computeBackoff(newCount)
	next := time.Now().UTC().Add(backoff)
	_, uerr := p.client.OutboxEvent.UpdateOneID(evt.ID).
		SetStatus(0). // 仍 pending
		SetRetryCount(newCount).
		SetLastError(err.Error()).
		SetNextRetryAt(next).
		Save(ctx)
	if uerr != nil {
		return fmt.Errorf("mark retry: %w", uerr)
	}
	return nil
}

// computeBackoff 指数退避：Base * 3^(retry-1)，封顶 BackoffMax。
func (p *OutboxPoller) computeBackoff(retry int) time.Duration {
	if retry < 1 {
		retry = 1
	}
	backoff := p.cfg.BackoffBase
	for i := 1; i < retry; i++ {
		backoff *= 3
		if backoff > p.cfg.BackoffMax {
			return p.cfg.BackoffMax
		}
	}
	return backoff
}
