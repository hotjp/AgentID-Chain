// Package testcontainers: redis 容器 helper。
//
// 启动一个真实的 Redis 容器（默认 redis:7-alpine），暴露：
//   - Addr():   "host:port"
//   - URL():    "redis://host:port"
//   - Cache():  cache.Cache（go-redis 客户端 + 我们的 wrapper）
//   - Client(): *redis.Client（go-redis 原始客户端，便于执行 FlushDB / Config 等）
//   - FlushDB(): 清空当前 DB
//   - Terminate(): 停止容器
//
// 推荐用法（testing.TB）：
//
//	rdb := testcontainers.MustNewRedisContainer(t, RedisOpts{})
//	defer rdb.Terminate(t.Context())
//	cache := rdb.Cache(t.Context())
//	// ... 用 cache ...
package testcontainers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/agentid-chain/agentid-chain/internal/cache"
)

// =============================================================================
// 选项
// =============================================================================

// RedisOpts 启动参数（全部可选，零值 = 默认）。
type RedisOpts struct {
	// Image 镜像 tag（默认 "redis:7-alpine"）
	Image string
	// Password AUTH 密码（默认无）
	Password string
	// DB 数据库编号（默认 0）
	DB int
	// StartupTimeout 启动等待上限（默认 30s）
	StartupTimeout time.Duration
	// Reuse 启用 testcontainers reuse（需 TESTCONTAINERS_REUSE_ENABLE=1）
	Reuse bool
}

// =============================================================================
// RedisContainer
// =============================================================================

// RedisContainer 包装 testcontainers 启动的 Redis 实例。
type RedisContainer struct {
	t        testing.TB
	ctr      *tcredis.RedisContainer
	host     string
	port     int
	password string
	db       int
}

// NewRedisContainer 启动容器（如失败返回 error，调用方决定 skip / fail）。
func NewRedisContainer(t testing.TB, opts RedisOpts) (*RedisContainer, error) {
	t.Helper()
	if t == nil {
		return nil, errors.New("testcontainers: nil testing.TB")
	}
	if opts.Image == "" {
		opts.Image = "redis:7-alpine"
	}
	if opts.StartupTimeout == 0 {
		opts.StartupTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.StartupTimeout)
	defer cancel()

	ctr, err := tcredis.Run(ctx, opts.Image)
	if err != nil {
		return nil, fmt.Errorf("testcontainers: run redis: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = ctr.Terminate(context.Background())
		return nil, fmt.Errorf("testcontainers: host: %w", err)
	}
	mappedPort, err := ctr.MappedPort(ctx, "6379/tcp")
	if err != nil {
		_ = ctr.Terminate(context.Background())
		return nil, fmt.Errorf("testcontainers: mapped port: %w", err)
	}

	c := &RedisContainer{
		t:        t,
		ctr:      ctr,
		host:     host,
		port:     int(mappedPort.Num()),
		password: opts.Password,
		db:       opts.DB,
	}

	// 注册自动清理
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("testcontainers: terminate redis: %v", err)
		}
	})

	return c, nil
}

// MustNewRedisContainer 启动失败 → t.Fatal。
func MustNewRedisContainer(t testing.TB, opts RedisOpts) *RedisContainer {
	t.Helper()
	c, err := NewRedisContainer(t, opts)
	if err != nil {
		t.Skipf("redis container unavailable: %v", err)
	}
	return c
}

// =============================================================================
// 访问器
// =============================================================================

// Addr 返回 "host:port"。
func (c *RedisContainer) Addr() string {
	return fmt.Sprintf("%s:%d", c.host, c.port)
}

// URL 返回 "redis://host:port"。
func (c *RedisContainer) URL() string {
	return fmt.Sprintf("redis://%s:%d", c.host, c.port)
}

// Host 返回容器映射的 host。
func (c *RedisContainer) Host() string {
	return c.host
}

// Port 返回容器映射的端口。
func (c *RedisContainer) Port() int {
	return c.port
}

// Client 返回 *redis.Client（go-redis 原始客户端）。
//
// 调用方负责在退出时调用 client.Close()。
func (c *RedisContainer) Client(ctx context.Context) *redis.Client {
	c.t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Addr(),
		Password:     c.password,
		DB:           c.db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		c.t.Fatalf("testcontainers: redis ping: %v", err)
	}
	c.t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// Cache 返回 cache.Cache 接口（包成我们的 wrapper）。
//
// 失败 → t.Fatal。
func (c *RedisContainer) Cache(ctx context.Context) cache.Cache {
	c.t.Helper()
	cache, err := cache.NewRedis(cache.RedisConfig{
		Addr:    c.Addr(),
		DB:      c.db,
		Timeout: 3 * time.Second,
	})
	if err != nil {
		c.t.Fatalf("testcontainers: new redis cache: %v", err)
	}
	return cache
}

// =============================================================================
// 维护
// =============================================================================

// FlushDB 清空当前 DB（适合 test-between reset）。
func (c *RedisContainer) FlushDB(ctx context.Context) error {
	rdb := c.Client(ctx)
	return rdb.FlushDB(ctx).Err()
}

// Terminate 停止并删除容器（幂等：多次调用安全）。
func (c *RedisContainer) Terminate(ctx context.Context) error {
	if c.ctr == nil {
		return nil
	}
	if err := c.ctr.Terminate(ctx); err != nil {
		return fmt.Errorf("testcontainers: terminate: %w", err)
	}
	c.ctr = nil
	return nil
}
