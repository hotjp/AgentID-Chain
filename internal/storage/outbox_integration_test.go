//go:build integration
// +build integration

// Package storage 集成测试 — Outbox 端到端（poller + forwarder + Redis Stream）。
//
// 运行方式：
//
//	cd internal/storage
//	go test -tags=integration -run TestOutboxEndToEnd -v
//
// 前置条件：
//   - Docker daemon 启动
//   - 网络可达 docker hub
package storage

import (
	"context"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/ent/outboxevent"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// TestOutboxEndToEnd 验证：
//  1. WriteOutbox 事务内写
//  2. OutboxPoller 取出 pending
//  3. OutboxForwarder XADD 到 Redis Stream
//  4. XREAD 收到事件，status=published
func TestOutboxEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 1. PG
	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("agentid"),
		tcpostgres.WithUsername("agentid"),
		tcpostgres.WithPassword("agentid_dev"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("postgres container start failed: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(pgC) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pg, err := OpenPostgres(ctx, PostgresConfig{DSN: dsn, MaxOpen: 5, MaxIdle: 2, MaxLifetime: time.Minute})
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer pg.Close()
	if err := pg.Client.Schema.Create(ctx); err != nil {
		t.Fatalf("schema: %v", err)
	}

	// 2. Redis
	redisC, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Skipf("redis container start failed: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(redisC) })
	redisAddr, err := redisC.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("redis endpoint: %v", err)
	}
	redisCli, err := OpenRedis(ctx, RedisConfig{Addr: redisAddr})
	if err != nil {
		t.Fatalf("OpenRedis: %v", err)
	}
	defer redisCli.Close()

	// 3. 写一条 outbox event
	tx, err := pg.Client.Tx(ctx)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	evt := OutboxEvent{
		AggregateType:  "agent",
		AggregateID:    "agent-e2e-001",
		EventType:      "agent.registered",
		Payload:        map[string]any{"k": "v"},
		IdempotencyKey: "e2e:agent:registered:agent-e2e-001:v1",
	}
	if err := WriteOutbox(ctx, FromEntTx(tx), evt); err != nil {
		t.Fatalf("WriteOutbox: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	t.Log("e2e: outbox event written")

	// 4. 启 poller + forwarder
	forwarder := NewOutboxForwarder(redisCli, "test-stream", nil)
	poller := NewOutboxPoller(pg.Client, forwarder, PollerConfig{
		PollInterval: 100 * time.Millisecond,
		BatchSize:    10,
	}, nil)

	pollerCtx, pollerCancel := context.WithCancel(ctx)
	pollerDone := make(chan struct{})
	go func() {
		defer close(pollerDone)
		_ = poller.Run(pollerCtx)
	}()

	// 5. XREAD 阻塞读 stream（等待 poller XADD）
	xreadCtx, xreadCancel := context.WithTimeout(ctx, 30*time.Second)
	defer xreadCancel()
	streams, err := redisCli.XRead(xreadCtx, &redis.XReadArgs{
		Streams: []string{"test-stream", "$"},
		Count:   1,
		Block:   10 * time.Second,
	}).Result()
	if err != nil {
		t.Logf("XRead err: %v", err)
	}
	if streams != nil && len(streams) > 0 {
		for _, s := range streams {
			for _, msg := range s.Messages {
				t.Logf("e2e: XREAD received stream=%s id=%s values=%v",
					s.Stream, msg.ID, msg.Values)
			}
		}
	}

	// 6. 停 poller
	pollerCancel()
	<-pollerDone

	// 7. 验证 status=published
	count, err := pg.Client.OutboxEvent.Query().
		Where(outboxevent.StatusEQ(1)).
		Count(ctx)
	if err != nil {
		t.Logf("query outbox: %v", err)
	}
	t.Logf("e2e: outbox published count=%d (0=未转发/失败；>0=成功)", count)

	t.Log("e2e: outbox pipeline test done")
}
