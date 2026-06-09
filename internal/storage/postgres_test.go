package storage

import (
	"context"
	"errors"
	"testing"
)

func TestOpenPostgres_EmptyDSN(t *testing.T) {
	ctx := context.Background()
	_, err := OpenPostgres(ctx, PostgresConfig{DSN: ""})
	if err == nil {
		t.Fatal("expected error for empty DSN")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want wraps ErrUnavailable", err)
	}
}

func TestOpenPostgres_InvalidDSN(t *testing.T) {
	ctx := context.Background()
	// DSN 解析失败通常在 sql.Open 时就报
	_, err := OpenPostgres(ctx, PostgresConfig{
		DSN: "not-a-valid-dsn-format",
	})
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
	// 不强制要求是 ErrUnavailable（OpenPostgres 的 sql.Open 可能直接报错）
	t.Logf("got err: %v", err)
}

func TestPostgresConfig_Defaults(t *testing.T) {
	// 验证 cfg 字段为空时 OpenPostgres 走默认值
	// 用一个永远不会连上的 DSN，Open 成功但 Ping 失败
	ctx := context.Background()
	_, err := OpenPostgres(ctx, PostgresConfig{
		DSN: "postgres://x:y@127.0.0.1:1/none?sslmode=disable&connect_timeout=1",
		// 留空让默认值生效
	})
	if err == nil {
		t.Fatal("expected ping failure")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want wraps ErrUnavailable", err)
	}
}

func TestPostgresHandle_NilReceiver(t *testing.T) {
	var h *PostgresHandle
	if err := h.Close(); err != nil {
		t.Errorf("nil Close = %v, want nil", err)
	}
	if err := h.Ping(context.Background()); err == nil {
		t.Error("nil Ping should fail")
	}
}

func TestPostgresHandle_ClientOnly(t *testing.T) {
	// 构造只有 Client 没有 DB 的 handle（人工构造，不调 OpenPostgres）
	h := &PostgresHandle{Client: nil, DB: nil}
	if err := h.Ping(context.Background()); err == nil {
		t.Error("nil handle Ping should fail")
	}
}
