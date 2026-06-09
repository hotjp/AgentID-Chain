# Redis Pipeline 性能优化 (P18.10)

> 目标：批量操作性能提升 5x
> 实现：`internal/cache/redis_pipeline.go`

## 1. 原理

### 1.1 单命令 vs Pipeline

```
单命令：       client → server    → client → server    → client
                   SET k1 v1          GET k1             DEL k1
                   1 RTT              1 RTT              1 RTT = 3 RTT

Pipeline：     client → server → client
                   SET k1 v1; GET k1; DEL k1
                   (1 RTT = 1 网络往返)
```

### 1.2 提升幅度

| 场景 | 单命令 | Pipeline | 提升 |
|------|--------|----------|------|
| 100 个 GET（局域网，1ms RTT）| 100ms | ~1.5ms | **66x** |
| 100 个 GET（云，5ms RTT） | 500ms | ~6ms | **83x** |
| 100 个 SET（局域网） | 100ms | ~2ms | **50x** |
| 混合读写 | 200ms | ~3ms | **66x** |

## 2. API 一览

| 方法 | 用途 | 命令数 |
|------|------|--------|
| `MGet` | 批量读 | 1 (MGET) |
| `MSet` | 批量写（无 TTL）| 1 (MSET) |
| `MSetWithTTL` | 批量写（带 TTL）| N (SET) |
| `MDel` | 批量删 | 1 (DEL) |
| `MExists` | 批量存在 | N (EXISTS) |
| `MIncrBy` | 批量自增 | N (INCRBY + EXPIRE) |

## 3. 用法示例

### 3.1 批量读（最高效：MGET）

```go
pipe := cache.NewPipeline(c)
keys := []string{"agent:uuid-1", "agent:uuid-2", "agent:uuid-3"}
vals, err := pipe.MGet(ctx, keys)
// 1 RTT
```

### 3.2 批量写带 TTL

```go
items := []cache.SetItem{
    {Key: "agent:1", Value: data1, TTL: 5 * time.Minute},
    {Key: "agent:2", Value: data2, TTL: 5 * time.Minute},
}
err := pipe.MSetWithTTL(ctx, items)
// N 个 SET 合并为 1 RTT
```

### 3.3 批量自增（限流场景）

```go
items := []cache.IncrItem{
    {Key: "rl:ip:1.2.3.4", Delta: 1, NewTTL: time.Minute},
    {Key: "rl:ip:5.6.7.8", Delta: 1, NewTTL: time.Minute},
}
counts, err := pipe.MIncrBy(ctx, items)
// 同时设置 TTL，避免 key 永远存在
```

## 4. 与 L3 Authz 集成

### 4.1 AAP challenge 存储（当前实现）

```go
// 之前：每个 challenge 单独 Set
g.store.Set(ctx, key1, val1, ttl)
g.store.Set(ctx, key2, val2, ttl)
g.store.Set(ctx, key3, val3, ttl)
// 3 RTT

// 之后：批量
pipe := cache.NewPipeline(g.store)
pipe.MSetWithTTL(ctx, []cache.SetItem{...})
// 1 RTT
```

### 4.2 Rate Limit 批量自增

```go
// 之前：每个 IP 单独 Incr + Expire（2 RTT）
g.store.Incr(ctx, "rl:ip:"+ip, time.Minute)
g.store.Expire(ctx, "rl:ip:"+ip, time.Minute)

// 之后：1 RTT
pipe.MIncrBy(ctx, []cache.IncrItem{{Key: "rl:ip:"+ip, Delta: 1, NewTTL: time.Minute}})
```

## 5. 基准测试

```bash
go test -bench=BenchmarkPipeline -benchmem -run=^$ ./internal/cache/ -benchtime=1s
```

## 6. 注意事项

### 6.1 不适合 Pipeline 的场景

- **单个 key 操作** —— 增加 pipeline 装配开销反而更慢
- **需要中间结果** —— pipeline 不支持链式（用 Lua script）

### 6.2 Pipeline 大小限制

| 建议 | 说明 |
|------|------|
| < 1000 commands/batch | 过大增加 server 端处理时间 |
| < 1 MB total payload | Redis 协议限制 |
| 不要混用事务 | 用 TxPipeline 替代 |

### 6.3 错误处理

```go
_, err := pipe.Exec(ctx)
if err != nil {
    // 整个 pipeline 失败（含 redis.Nil 错误）
}
// 单独命令错误：cmds[i].Err()
```

## 7. 进阶：Lua 脚本

比 Pipeline 更强的能力：

```lua
-- 限流 + 设置 TTL（原子）
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
```

通过 `pipe.rdb.Eval(ctx, script, keys, args...)` 调用。

## 8. 引用

- Redis Pipeline 文档：https://redis.io/docs/manual/pipelining/
- go-redis v9 Pipeline：https://redis.uptrace.dev/guide/go-redis-pipelines.html
