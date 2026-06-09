// Package miniredis 提供 miniredis 嵌入测试辅助（无外部 Redis 依赖）。
//
// 用法：
//
//	func TestSomething(t *testing.T) {
//	    mr, addr := miniredis.NewMiniRedis(t)
//	    defer mr.Close()
//	    // addr 如 "127.0.0.1:6379"；用其创建 redis.Client
//	    client := redis.NewClient(&redis.Options{Addr: addr})
//	}
//
// 适用场景：
//   - 单元测试（fast / no docker）
//   - CI 失败调试（无副作用）
//   - 大量并发测试（避免 6379 端口冲突）
package miniredis

import (
	"testing"
	"time"

	mr "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// NewMiniRedis 启动一个嵌入 Redis（miniredis），返回 server 句柄与监听地址。
// t.Cleanup() 自动关闭。
//
// 注意：miniredis 不支持所有真实 Redis 命令（特别是 stream / cluster），
// 对于 outbox / revocation 测试可能需要 testcontainers/redis。
func NewMiniRedis(t *testing.T) (*mr.Miniredis, string) {
	t.Helper()
	s, err := mr.Run()
	if err != nil {
		t.Fatalf("miniredis: run failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, s.Addr()
}

// NewMiniRedisWithClient 直接返回 redis 客户端（基于 miniredis）。
// t.Cleanup() 自动关闭 server 与 client。
func NewMiniRedisWithClient(t *testing.T) (*mr.Miniredis, *redis.Client) {
	t.Helper()
	s, addr := NewMiniRedis(t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	return s, client
}

// NewMiniRedisWithTime 返回带 FastForward 支持的 miniredis。
// 用于测试 TTL / 过期相关逻辑。
func NewMiniRedisWithTime(t *testing.T) (*mr.Miniredis, string) {
	t.Helper()
	s, addr := NewMiniRedis(t)
	// 默认 miniredis 时间为 real-time；如需 fast-forward，调用 s.FastForward(d)
	return s, addr
}

// FlushAll 清空 miniredis 全部数据。
// 用于：在 subtest 之间清理状态。
func FlushAll(t *testing.T, s *mr.Miniredis) {
	t.Helper()
	s.FlushAll()
}

// SetTime 设置 miniredis 内部时间（用于测试过期）。
func SetTime(t *testing.T, s *mr.Miniredis, ts time.Time) {
	t.Helper()
	s.SetTime(ts)
}

// FastForward 让时间快进 d。
func FastForward(t *testing.T, s *mr.Miniredis, d time.Duration) {
	t.Helper()
	s.FastForward(d)
}
