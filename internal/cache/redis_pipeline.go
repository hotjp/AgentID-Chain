// Package cache: Redis Pipeline 批量操作 (P18.10)。
//
// Pipeline 原理：
//   - 单个命令：1 RTT（client → server → client）
//   - Pipeline：N 个命令 → 1 RTT（client 一次发送，server 一次返回所有结果）
//   - 典型提升：10-100 倍（高延迟网络环境）
//
// 本文件提供：
//   - Pipeline 批量 Get / Set / Del
//   - MGet / MSet（如果 Redis 版本支持，更高效）
//   - TxPipeline（事务性批量）
//
// 适用场景：
//   - 批量查询（如批量 agent 加载）
//   - 批量写入（如批量权限更新）
//   - 批量删除（如批量撤销）
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// =============================================================================
// Pipeline 封装
// =============================================================================

// Pipeline 批量操作封装。
type Pipeline struct {
	rdb *redis.Client
}

// NewPipeline 创建 Pipeline。
func NewPipeline(c Cache) *Pipeline {
	if pc, ok := c.(*redisCache); ok {
		return &Pipeline{rdb: pc.client}
	}
	return nil
}

// MGet 批量获取（1 RTT）。
//
// 行为：返回的 [][]byte 中，不存在的 key 对应 nil。
// 错误：Redis 整体错误返回（个别 key 不存在不算错）。
func (p *Pipeline) MGet(ctx context.Context, keys []string) ([][]byte, error) {
	if p == nil || p.rdb == nil {
		return nil, errors.New("cache: pipeline not initialized with redis backend")
	}
	if len(keys) == 0 {
		return nil, nil
	}
	// go-redis v9 用 MGet 一次取全部
	res, err := p.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([][]byte, len(res))
	for i, v := range res {
		if v == nil {
			out[i] = nil
			continue
		}
		if s, ok := v.(string); ok {
			out[i] = []byte(s)
		} else if b, ok := v.([]byte); ok {
			out[i] = b
		}
	}
	return out, nil
}

// MSet 批量设置（1 RTT，不带 TTL）。
//
// 注意：MSet 不支持 TTL；如需 TTL 请用 Pipeline + 多次 Set。
func (p *Pipeline) MSet(ctx context.Context, kv map[string][]byte) error {
	if p == nil || p.rdb == nil {
		return errors.New("cache: pipeline not initialized with redis backend")
	}
	if len(kv) == 0 {
		return nil
	}
	values := make([]any, 0, len(kv)*2)
	for k, v := range kv {
		values = append(values, k, v)
	}
	return p.rdb.MSet(ctx, values...).Err()
}

// MSetWithTTL 批量设置（带 TTL，逐个 Set 在 pipeline 中）。
//
// 行为：Pipeline 中每个 Set 携带独立 TTL。
// 性能：N 个 Set 合并为 1 RTT（vs N 个独立 Set = N RTT）。
func (p *Pipeline) MSetWithTTL(ctx context.Context, items []SetItem) error {
	if p == nil || p.rdb == nil {
		return errors.New("cache: pipeline not initialized with redis backend")
	}
	if len(items) == 0 {
		return nil
	}
	pipe := p.rdb.Pipeline()
	for _, item := range items {
		pipe.Set(ctx, item.Key, item.Value, item.TTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// MDel 批量删除（1 RTT）。
func (p *Pipeline) MDel(ctx context.Context, keys []string) (int64, error) {
	if p == nil || p.rdb == nil {
		return 0, errors.New("cache: pipeline not initialized with redis backend")
	}
	if len(keys) == 0 {
		return 0, nil
	}
	return p.rdb.Del(ctx, keys...).Result()
}

// =============================================================================
// MExists 批量存在性检查
// =============================================================================

// MExists 批量检查 key 是否存在。
//
// 返回：bool 切片（与 keys 一一对应）。
func (p *Pipeline) MExists(ctx context.Context, keys []string) ([]bool, error) {
	if p == nil || p.rdb == nil {
		return nil, errors.New("cache: pipeline not initialized with redis backend")
	}
	if len(keys) == 0 {
		return nil, nil
	}
	pipe := p.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(keys))
	for i, k := range keys {
		cmds[i] = pipe.Exists(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	out := make([]bool, len(keys))
	for i, cmd := range cmds {
		out[i] = cmd.Val() > 0
	}
	return out, nil
}

// =============================================================================
// MIncrBy 批量自增
// =============================================================================

// MIncrBy 批量自增（计数器场景）。
//
// 行为：每个 key 原子 +1，返回新值。
// 注意：自增不会重置 TTL。
func (p *Pipeline) MIncrBy(ctx context.Context, items []IncrItem) ([]int64, error) {
	if p == nil || p.rdb == nil {
		return nil, errors.New("cache: pipeline not initialized with redis backend")
	}
	if len(items) == 0 {
		return nil, nil
	}
	pipe := p.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(items))
	for i, item := range items {
		cmds[i] = pipe.IncrBy(ctx, item.Key, item.Delta)
		if item.NewTTL > 0 {
			pipe.Expire(ctx, item.Key, item.NewTTL)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	out := make([]int64, len(items))
	for i, cmd := range cmds {
		out[i] = cmd.Val()
	}
	return out, nil
}

// =============================================================================
// 数据结构
// =============================================================================

// SetItem 单个 Set 项。
type SetItem struct {
	Key   string
	Value []byte
	TTL   time.Duration
}

// IncrItem 单个 Incr 项。
type IncrItem struct {
	Key    string
	Delta  int64
	NewTTL time.Duration // 0 = 不改 TTL
}
