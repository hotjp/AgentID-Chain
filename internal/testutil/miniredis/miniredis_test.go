package miniredis

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewMiniRedis(t *testing.T) {
	mr, addr := NewMiniRedis(t)
	if addr == "" {
		t.Fatal("addr should not be empty")
	}
	if mr == nil {
		t.Fatal("miniredis instance should not be nil")
	}
	// 验证能 PING
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	pong, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	if pong != "PONG" {
		t.Errorf("ping = %q, want PONG", pong)
	}
}

func TestNewMiniRedisWithClient(t *testing.T) {
	mr, client := NewMiniRedisWithClient(t)
	if mr == nil {
		t.Fatal("mr should not be nil")
	}
	if client == nil {
		t.Fatal("client should not be nil")
	}
	// 写一次
	if err := client.Set(context.Background(), "key", "value", time.Minute).Err(); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	// 读一次
	got, err := client.Get(context.Background(), "key").Result()
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got != "value" {
		t.Errorf("got = %q, want value", got)
	}
}

func TestFlushAll(t *testing.T) {
	mr, client := NewMiniRedisWithClient(t)
	ctx := context.Background()

	// 写入数据
	if err := client.Set(ctx, "k1", "v1", 0).Err(); err != nil {
		t.Fatal(err)
	}
	// 清空
	FlushAll(t, mr)
	// 验证已清空
	_, err := client.Get(ctx, "k1").Result()
	if err != redis.Nil {
		t.Errorf("expected redis.Nil after flush, got %v", err)
	}
}

func TestFastForward(t *testing.T) {
	mr, client := NewMiniRedisWithClient(t)
	ctx := context.Background()

	// 写入 1s 过期
	if err := client.Set(ctx, "temp", "data", time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	// 立刻读
	if _, err := client.Get(ctx, "temp").Result(); err != nil {
		t.Fatalf("get should succeed before expiry: %v", err)
	}
	// 快进 2s
	FastForward(t, mr, 2*time.Second)
	// 应该过期
	if _, err := client.Get(ctx, "temp").Result(); err != redis.Nil {
		t.Errorf("expected redis.Nil after fast-forward, got %v", err)
	}
}
