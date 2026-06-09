// Package cache MiniRedis 实现（仅供测试使用）。
package cache

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// miniredisCache Cache 的 MiniRedis 实现。
//
// 用法：
//
//	mr, _ := miniredis.Run()
//	defer mr.Close()
//	c := cache.NewMiniredis(mr)
//	cache.Set(ctx, "k", []byte("v"), time.Minute)
//
// 适合：单元测试 / CI 环境（无需起真 Redis）
type miniredisCache struct {
	server *miniredis.Miniredis
	mu     sync.Mutex // 保护并发 Set/SetTTL
}

// NewMiniredis 返回基于 miniredis 的 Cache 实现。
func NewMiniredis(mr *miniredis.Miniredis) Cache {
	return &miniredisCache{server: mr}
}

func (m *miniredisCache) Get(_ context.Context, key string) ([]byte, error) {
	if !m.server.Exists(key) {
		return nil, ErrMiss
	}
	v, err := m.server.Get(key)
	if err != nil {
		return nil, err
	}
	return []byte(v), nil
}

func (m *miniredisCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.server.Set(key, string(value))
	if ttl > 0 {
		m.server.SetTTL(key, ttl)
	}
	return nil
}

func (m *miniredisCache) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		m.server.Del(k)
	}
	return nil
}

func (m *miniredisCache) Exists(_ context.Context, key string) (bool, error) {
	return m.server.Exists(key), nil
}

func (m *miniredisCache) Expire(_ context.Context, key string, ttl time.Duration) error {
	m.server.SetTTL(key, ttl)
	return nil
}

func (m *miniredisCache) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	// miniredis 没有原子 Incr；用 mutex 保护（测试场景足够）
	m.mu.Lock()
	defer m.mu.Unlock()
	v, err := m.server.Get(key)
	if err != nil {
		m.server.Set(key, "1")
		return 1, nil
	}
	cur, _ := strconv.ParseInt(v, 10, 64)
	next := cur + 1
	m.server.Set(key, strconv.FormatInt(next, 10))
	return next, nil
}

func (m *miniredisCache) StoreOnce(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if m.server.Exists(key) {
		return ErrMiss // key 已存在 — 用 ErrMiss 表达"未设置"
	}
	m.server.Set(key, string(value))
	if ttl > 0 {
		m.server.SetTTL(key, ttl)
	}
	return nil
}

func (m *miniredisCache) Close() error {
	m.server.Close()
	return nil
}
