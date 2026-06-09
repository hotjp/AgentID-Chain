package testcontainers

import (
	"context"
	"testing"
	"time"
)

func TestRedisContainer_StartAndPing(t *testing.T) {
	rdb, err := NewRedisContainer(t, RedisOpts{StartupTimeout: 60 * time.Second})
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := rdb.Client(ctx)
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestRedisContainer_AddrFormat(t *testing.T) {
	rdb, err := NewRedisContainer(t, RedisOpts{})
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	addr := rdb.Addr()
	if addr == "" {
		t.Error("addr should be non-empty")
	}
	url := rdb.URL()
	if url[:8] != "redis://" {
		t.Errorf("URL should start with redis://, got %q", url)
	}
}

func TestRedisContainer_CacheLayer(t *testing.T) {
	rdb, err := NewRedisContainer(t, RedisOpts{})
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	ctx := context.Background()
	c := rdb.Cache(ctx)
	if c == nil {
		t.Fatal("Cache returned nil")
	}

	// Set + Get 走完整 round-trip
	if err := c.Set(ctx, "test-key", []byte("hello"), time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(ctx, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestRedisContainer_FlushDB(t *testing.T) {
	rdb, err := NewRedisContainer(t, RedisOpts{})
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	ctx := context.Background()
	c := rdb.Cache(ctx)
	if err := c.Set(ctx, "x", []byte("1"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := rdb.FlushDB(ctx); err != nil {
		t.Fatal(err)
	}
	// FlushDB 后 key 不应存在
	_, err = c.Get(ctx, "x")
	if err == nil {
		t.Error("expected miss after flush")
	}
}

func TestRedisContainer_TerminateIdempotent(t *testing.T) {
	rdb, err := NewRedisContainer(t, RedisOpts{})
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	ctx := context.Background()
	if err := rdb.Terminate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Terminate(ctx); err != nil {
		t.Errorf("second terminate: %v", err)
	}
}
