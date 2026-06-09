// Package cache 提供 L1 缓存层的统一抽象。
//
// 设计目标：
//   - 业务层（L2-L5）只依赖 Cache 接口
//   - 实现可替换：Redis / MiniRedis（测试） / 内存 / Noop
//   - 包含常用方法 + 限流器用的原子自增
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrMiss 缓存未命中（不是错误，是正常分支）。
var ErrMiss = errors.New("cache: miss")

// ErrNotSupported 当前后端不支持该操作。
var ErrNotSupported = errors.New("cache: not supported")

// Cache 缓存接口。
//
// 业务示例：
//   - L4 Service 注入此接口
//   - 实现位于 redis.go (生产) / miniredis.go (测试) / noop.go (禁用)
type Cache interface {
	// Get 获取 key 的值；不存在返回 ErrMiss。
	Get(ctx context.Context, key string) ([]byte, error)

	// Set 设置 key/value，ttl=0 表示不过期。
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Del 删除 1+ 个 key。
	Del(ctx context.Context, keys ...string) error

	// Exists 判断 key 是否存在。
	Exists(ctx context.Context, key string) (bool, error)

	// Expire 设置 key 的 TTL。
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// Incr 原子自增（限流器用）；key 不存在时初始化为 0。
	// 返回自增后的值。
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)

	// Store 写入 Nonce 防重放，ttl 内不允许重复。
	// 语义：若 key 已存在，返回错误；否则写入并设置 ttl。
	StoreOnce(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Close 关闭底层连接。
	Close() error
}
