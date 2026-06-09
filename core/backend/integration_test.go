//go:build integration

// Package backend integration tests (P8.13)。
//
// 用 testcontainers 启真实 Redis 作为 cache，验证 LocalBackend 缓存路径
// 端到端可用（虽然 LocalBackend 当前缓存仅作 key 存在性标记，但接口完整）。
//
// 运行：
//
//	go test -tags=integration ./core/backend/...
package backend

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	tc "github.com/agentid-chain/agentid-chain/internal/testutil/testcontainers"
)

func TestIntegration_LocalBackend_WithRedisCache(t *testing.T) {
	rdb, err := tc.NewRedisContainer(t, tc.RedisOpts{})
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Terminate(context.Background()) })

	cache := newRedisCacheAdapter(rdb.Client(context.Background()))

	pers := NewMemoryPersistence()
	be, _ := NewLocalBackend(pers, cache, LocalConfig{CacheTTL: 5 * time.Second})

	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	info, err := be.GetAgentInfo(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != cred.UUID {
		t.Errorf("uuid = %q", info.UUID)
	}

	if err := be.UpdateAgentLevel(context.Background(), cred.UUID, 2, "integration"); err != nil {
		t.Fatal(err)
	}
	info, _ = be.GetAgentInfo(context.Background(), cred.UUID)
	if info.Level != 2 {
		t.Errorf("level = %d, want 2", info.Level)
	}
}

// redisCacheAdapter 把 *redis.Client 适配到 backend.Cache 接口。
type redisCacheAdapter struct {
	rdb *redis.Client
}

func newRedisCacheAdapter(rdb *redis.Client) *redisCacheAdapter {
	return &redisCacheAdapter{rdb: rdb}
}

func (r *redisCacheAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	v, err := r.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return v, err
}

func (r *redisCacheAdapter) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return r.rdb.Set(ctx, key, val, ttl).Err()
}

func (r *redisCacheAdapter) Del(ctx context.Context, key string) error {
	return r.rdb.Del(ctx, key).Err()
}
