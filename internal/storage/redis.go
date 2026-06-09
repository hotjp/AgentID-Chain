// Package storage Redis 客户端封装（裸 *redis.Client）。
//
// 与 internal/cache/redis.go 的区别：
//   - cache.Cache 抽象：L3 Authz 用，提供 Get/Set/Del/Incr/StoreOnce 等高层 API
//   - 本文件 *redis.Client：L1 Storage 用，提供 stream / pub-sub / Lua 脚本等底层能力
//
// 用途：
//   - revocation list（SET / GET / DEL — A2A token 撤销）
//   - nonce 存储（业务 nonce 与 cache.StoreOnce 并存；这里是 L1 视角）
//   - outbox forwarder（XADD / XREAD — 事件发布）
//   - 分布式锁（SETNX）
//
// 选型：go-redis/v9 — 官方维护、context-aware、cluster/sentinel 支持完整。
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig Redis 客户端配置（与 internal/config.RedisConfig 字段对齐）。
type RedisConfig struct {
	Addr     string // host:port
	Password string
	DB       int           // 0-15
	Timeout  time.Duration // dial/read/write 默认 3s
	PoolSize int           // 默认 10
}

// OpenRedis 打开 Redis 连接并返回 *redis.Client。
//
// 调用方负责在退出时调用 client.Close()。
//
// 失败模式：
//   - Addr 为空 → 立即返回 ErrUnavailable
//   - Ping 失败 → 关闭后返回 ErrUnavailable（让上层 fail-fast）
//
//nolint:gocritic // 工厂方法保持单参数 cfg
func OpenRedis(ctx context.Context, cfg RedisConfig) (*redis.Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("%w: redis addr is empty", ErrUnavailable)
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		PoolSize:     cfg.PoolSize,
	})

	// 健康检查
	pingCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("%w: redis ping %s: %w", ErrUnavailable, cfg.Addr, err)
	}
	return client, nil
}

// RedisHealthCheck 轻量健康检查。
func RedisHealthCheck(ctx context.Context, client *redis.Client) error {
	if client == nil {
		return errors.New("storage: redis client is nil")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("%w: redis ping: %w", ErrUnavailable, err)
	}
	return nil
}
