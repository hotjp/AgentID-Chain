//go:build integration
// +build integration

// Package storage 集成测试 — PostgreSQL 真实环境（testcontainers）。
//
// 运行方式：
//
//	cd internal/storage
//	go test -tags=integration -run TestPostgresIntegration -v
//
// 前置条件：
//   - Docker daemon 启动
//   - 网络可达 docker hub（testcontainers 拉 postgres 镜像）
//
// 测试目标：
//  1. OpenPostgres 真实连接 → sql.Open + Ping + ent dialect
//  2. ent schema migration 完整跑通
//  3. CRUD 流程（User / Agent / AuditLog / OutboxEvent）端到端验证
package storage

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestPostgresIntegration 集成测试主入口。
func TestPostgresIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 1. 启 PG 容器
	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("agentid"),
		tcpostgres.WithUsername("agentid"),
		tcpostgres.WithPassword("agentid_dev"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("postgres container start failed (likely no Docker): %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(pgC); err != nil {
			t.Logf("terminate: %v", err)
		}
	})

	// 2. 拿 DSN
	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	t.Logf("postgres up: %s", dsn)

	// 3. OpenPostgres
	h, err := OpenPostgres(ctx, PostgresConfig{
		DSN:         dsn,
		MaxOpen:     5,
		MaxIdle:     2,
		MaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer h.Close()

	// 4. Ping
	if err := h.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	t.Log("postgres integration: OpenPostgres + Ping OK")

	// 5. ent schema migration（建表）
	if err := h.Client.Schema.Create(ctx); err != nil {
		t.Fatalf("schema create: %v", err)
	}
	t.Log("postgres integration: schema migrated")

	// 6. CRUD：User
	email := "integration-test@example.com"
	u, err := h.Client.User.Create().
		SetEmail(email).
		SetRoleDid("did:agentid:role:test").
		Save(ctx)
	if err != nil {
		t.Fatalf("user create: %v", err)
	}
	got, err := h.Client.User.Get(ctx, u.ID)
	if err != nil {
		t.Fatalf("user get: %v", err)
	}
	if got.Email != email {
		t.Fatalf("email mismatch: want=%s got=%s", email, got.Email)
	}
	t.Logf("postgres integration: User CRUD OK (id=%s)", u.ID)

	// 7. CRUD：Agent
	a, err := h.Client.Agent.Create().
		SetOwnerDid("did:agentid:user:" + u.ID.String()).
		SetLevel(3).
		SetPermission(0xFF).
		SetStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("agent create: %v", err)
	}
	t.Logf("postgres integration: Agent created (id=%s)", a.ID)

	// 8. CRUD：AuditLog（关联 agent）
	al, err := h.Client.AuditLog.Create().
		SetAction("register").
		SetOperatorDid("did:agentid:user:admin").
		SetAgent(a).
		Save(ctx)
	if err != nil {
		t.Fatalf("audit log create: %v", err)
	}
	t.Logf("postgres integration: AuditLog created (id=%s)", al.ID)

	// 9. 验证关联：查 agent 的 audit_logs
	logs, err := a.QueryAuditLogs().All(ctx)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("audit_logs count=%d, want 1", len(logs))
	}
	t.Log("postgres integration: agent ↔ audit_log edge OK")

	// 10. CRUD：OutboxEvent
	oe, err := h.Client.OutboxEvent.Create().
		SetAggregateType("agent").
		SetAggregateID(a.ID.String()).
		SetEventType("agent.registered").
		SetPayload(map[string]any{"k": "v"}).
		SetIdempotencyKey("agent:registered:" + a.ID.String() + ":v1").
		SetStatus(OutboxStatusPending).
		SetRetryCount(0).
		Save(ctx)
	if err != nil {
		t.Fatalf("outbox create: %v", err)
	}
	t.Logf("postgres integration: OutboxEvent created (id=%s)", oe.ID)

	// 11. OutboxWriter：事务内写
	if err := testOutboxWriter(ctx, h); err != nil {
		t.Fatalf("outbox writer: %v", err)
	}
	t.Log("postgres integration: OutboxWriter OK")

	t.Log("postgres integration: ALL OK")
}

// testOutboxWriter 验证事务内 outbox 写入。
func testOutboxWriter(ctx context.Context, h *PostgresHandle) error {
	tx, err := h.Client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	evt := OutboxEvent{
		AggregateType:  "agent",
		AggregateID:    "test-agent-001",
		EventType:      "agent.registered",
		Payload:        map[string]any{"k": "v"},
		IdempotencyKey: "test:writer:v1",
	}
	if err := WriteOutbox(ctx, FromEntTx(tx), evt); err != nil {
		return err
	}
	return tx.Commit()
}
