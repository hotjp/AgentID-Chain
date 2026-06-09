// Package cache Redis 实现。
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig Redis 客户端配置（与 internal/config.RedisConfig 字段对齐）。
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	Timeout  time.Duration
}

// redisCache Cache 的 Redis 实现。
type redisCache struct {
	client *redis.Client
}

// NewRedis 返回基于 go-redis/v9 的 Cache 实现。
//
//nolint:gocritic // 工厂方法，参数类型不宜混合
func NewRedis(cfg RedisConfig) (Cache, error) {
	if cfg.Addr == "" {
		return nil, errors.New("cache: redis addr is empty")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}
	c := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
	})
	// 健康检查
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("cache: redis ping %s: %w", cfg.Addr, err)
	}
	return &redisCache{client: c}, nil
}

func (r *redisCache) Get(ctx context.Context, key string) ([]byte, error) {
	v, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	return v, err
}

func (r *redisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *redisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

func (r *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

func (r *redisCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	// 用 pipeline：INCR + EXPIRE (NX 仅当未设置 TTL)
	pipe := r.client.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

func (r *redisCache) StoreOnce(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// SET key value NX EX ttl：仅当 key 不存在时设置
	ok, err := r.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cache: key %q already exists", key)
	}
	return nil
}

func (r *redisCache) Close() error {
	return r.client.Close()
}
