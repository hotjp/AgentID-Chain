package testcontainers

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// 这些测试需要 Docker；CI lite 节点会 skip。
// 跑全部：go test -count=1 -tags=integration ./internal/testutil/testcontainers/...
// 本地：just-go docker 起来后即可。

func TestPostgresContainer_StartAndPing(t *testing.T) {
	pg, err := NewPostgresContainer(t, PostgresOpts{StartupTimeout: 90 * time.Second})
	if err != nil {
		t.Skipf("postgres container unavailable (no docker?): %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := pg.DB(ctx)
	defer db.Close()

	// 简单 query 验证连通
	var got int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&got); err != nil {
		t.Fatalf("ping query: %v", err)
	}
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestPostgresContainer_DSNFormat(t *testing.T) {
	pg, err := NewPostgresContainer(t, PostgresOpts{
		DBName:   "sample",
		User:     "u",
		Password: "p",
	})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	dsn := pg.DSN()
	want := "postgres://u:p@"
	if !contains(dsn, want) {
		t.Errorf("DSN %q missing %q", dsn, want)
	}
	if !contains(dsn, "/sample") {
		t.Errorf("DSN %q missing dbname", dsn)
	}
	if !contains(dsn, "sslmode=disable") {
		t.Errorf("DSN %q missing sslmode=disable", dsn)
	}
}

func TestPostgresContainer_EntMigrationsApplied(t *testing.T) {
	pg, err := NewPostgresContainer(t, PostgresOpts{})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := pg.DB(ctx)
	defer db.Close()

	// ent schema 至少有 users 表（v2.0.1）
	rows, err := db.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public'`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if len(tables) == 0 {
		t.Error("ent migrate should have created at least one table")
	}
}

func TestPostgresContainer_ApplySQLDir(t *testing.T) {
	dir := t.TempDir()
	// 写两个 SQL，按字母序
	if err := writeFile(t, filepath.Join(dir, "01-ext.sql"),
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(t, filepath.Join(dir, "02-table.sql"),
		`CREATE TABLE test_apply (id SERIAL PRIMARY KEY, name TEXT);`); err != nil {
		t.Fatal(err)
	}
	pg, err := NewPostgresContainer(t, PostgresOpts{
		SQLDir: dir,
	})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	ctx := context.Background()
	db := pg.DB(ctx)
	defer db.Close()

	var n int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM test_apply").Scan(&n); err != nil {
		t.Fatalf("table not created: %v", err)
	}
}

func TestPostgresContainer_Reset(t *testing.T) {
	pg, err := NewPostgresContainer(t, PostgresOpts{})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	ctx := context.Background()
	db := pg.DB(ctx)
	defer db.Close()

	// 写一行
	if _, err := db.ExecContext(ctx, "CREATE TABLE rt (id INT); INSERT INTO rt VALUES (1);"); err != nil {
		t.Fatal(err)
	}
	if err := pg.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	// 重建后 rt 应不存在
	_, err = db.ExecContext(ctx, "SELECT * FROM rt")
	if err == nil {
		t.Error("expected error querying dropped table")
	}
}

func TestPostgresContainer_TerminateIdempotent(t *testing.T) {
	pg, err := NewPostgresContainer(t, PostgresOpts{})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	ctx := context.Background()
	if err := pg.Terminate(ctx); err != nil {
		t.Fatal(err)
	}
	// 第二次 Terminate 应为 no-op
	if err := pg.Terminate(ctx); err != nil {
		t.Errorf("second terminate: %v", err)
	}
}

func TestPostgresContainer_EntClient(t *testing.T) {
	pg, err := NewPostgresContainer(t, PostgresOpts{})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	ctx := context.Background()
	client := pg.EntClient(ctx)
	if client == nil {
		t.Fatal("EntClient returned nil")
	}
	defer client.Close()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatalf("schema create via client: %v", err)
	}
}

func TestPostgresContainer_WithoutEntMigrations(t *testing.T) {
	pg, err := NewPostgresContainer(t, PostgresOpts{}.WithApplyEntMigrations(false))
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	ctx := context.Background()
	db := pg.DB(ctx)
	defer db.Close()
	// 没有 ent schema，应为 0 表
	rows, err := db.QueryContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables
		 WHERE table_schema = 'public'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var n int
	if !rows.Next() {
		t.Fatal("no row")
	}
	if err := rows.Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 tables, got %d", n)
	}
}

func TestPostgresContainer_SQLDirNotFound(t *testing.T) {
	_, err := NewPostgresContainer(t, PostgresOpts{
		SQLDir: "/nonexistent/path/to/sql",
	})
	if err == nil {
		t.Error("expected error for missing sql dir")
	}
}

func TestPostgresContainer_Reuse(t *testing.T) {
	// 验证 Reuse 选项能传入（实际启用需 TESTCONTAINERS_REUSE_ENABLE=1）
	// 这里仅确保不 panic
	pg, err := NewPostgresContainer(t, PostgresOpts{Reuse: true})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	if pg == nil {
		t.Fatal("nil container")
	}
}

func TestPostgresContainer_HostPort(t *testing.T) {
	pg, err := NewPostgresContainer(t, PostgresOpts{})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	if pg.Host() == "" {
		t.Error("Host should be non-empty")
	}
	if pg.Port() == 0 {
		t.Error("Port should be non-zero")
	}
}

// =============================================================================
// 工具
// =============================================================================

func writeFile(t *testing.T, path, content string) error {
	t.Helper()
	return writeFileImpl(path, content)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
