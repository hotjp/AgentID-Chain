package storage

import (
	"context"
	"errors"
	"testing"
)

func TestOpenRedis_EmptyAddr(t *testing.T) {
	ctx := context.Background()
	_, err := OpenRedis(ctx, RedisConfig{Addr: ""})
	if err == nil {
		t.Fatal("expected error for empty addr")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want wraps ErrUnavailable", err)
	}
}

func TestOpenRedis_Defaults(t *testing.T) {
	// 不可达的地址（无端口暴露），验证 Ping 失败时返回 ErrUnavailable
	ctx := context.Background()
	_, err := OpenRedis(ctx, RedisConfig{
		Addr: "127.0.0.1:1", // 不可达
		// 留空让默认值生效
	})
	if err == nil {
		t.Fatal("expected connection failure")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want wraps ErrUnavailable", err)
	}
}

func TestRedisHealthCheck_NilClient(t *testing.T) {
	if err := RedisHealthCheck(context.Background(), nil); err == nil {
		t.Error("nil client healthcheck should fail")
	}
}
