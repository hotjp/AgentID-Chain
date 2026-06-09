// Package integration — 集成测试模板（//go:build integration）。
//
// 集成测试（Integration Test）规范：
//   - 仅在指定 build tag 下编译：`//go:build integration`
//   - 需要真实 PostgreSQL / Redis（通过 testcontainers 或本地实例）
//   - 每个测试独立事务 / 独立 schema / 测试完清空数据
//   - 慢（> 100ms / case），CI 中分阶段跑
//
// 运行：
//   go test -tags=integration -count=1 ./internal/storage/...
//
// Makefile target：
//   make test-integration
package integration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/testutil/testcontainers"
)

// TestMain 集成测试入口；启动一次容器，对所有 test 复用。
func TestMain(m *testing.M) {
	// 启动 PG / Redis（如需）
	// pg := testcontainers.NewPostgresContainer(...)
	// redis := testcontainers.NewRedisContainer(...)
	// defer pg.Terminate(context.Background())
	// defer redis.Terminate(context.Background())

	os.Exit(m.Run())
}

// ExampleTest_Postgres 集成测试模板：连真实 PG，建表，CRUD。
func ExampleTest_Postgres(t *testing.T) {
	// 1. 启动容器
	pg, err := testcontainers.NewPostgresContainer(t, testcontainers.PostgresOpts{
		DBName:  "test",
		User:    "test",
		Password: "test",
	})
	if err != nil {
		t.Fatalf("start pg: %v", err)
	}
	defer pg.Terminate(context.Background())
	dsn := pg.DSN()

	// 2. 连库
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// 3. 建测试 schema
	setupSchema(t, db)
	t.Cleanup(func() { teardownSchema(t, db) })

	// 4. 跑测试
	t.Run("insert", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO test_users (id, name) VALUES (1, 'alice')`)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	})

	t.Run("query", func(t *testing.T) {
		var name string
		err := db.QueryRowContext(ctx, `SELECT name FROM test_users WHERE id = 1`).Scan(&name)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if name != "alice" {
			t.Errorf("name = %q, want alice", name)
		}
	})
}

func setupSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_users (
			id INT PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
}

func teardownSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, _ = db.Exec(`DROP TABLE IF EXISTS test_users`)
}
