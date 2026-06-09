# AAP 握手性能基准 (P18.2)

> 目标：单次 Challenge 生成 + Verify < 5ms
> 工具：Go 标准 `testing.Benchmark`
> 后端：miniredis（进程内 Redis，零网络开销）
> 平台：Apple M5 Pro / darwin arm64
> 时间：2026-06-09

## 测试代码

参见 `internal/authz/aap/benchmark_test.go`。

## 执行命令

```bash
go test -bench=Benchmark -benchmem -run=^$ ./internal/authz/aap/ -benchtime=1s
```

## 结果

```
goos: darwin
goarch: arm64
pkg: github.com/agentid-chain/agentid-chain/internal/authz/aap
cpu: Apple M5 Pro
BenchmarkChallenge-15          	   72936	     14177 ns/op	    2427 B/op	      21 allocs/op
BenchmarkVerify-15             	   22903	     52593 ns/op	    3624 B/op	      38 allocs/op
BenchmarkChallengeVerify-15    	   22915	     52782 ns/op	    3619 B/op	      38 allocs/op
BenchmarkParallelVerify-15     	  137556	      7695 ns/op	    3584 B/op	      38 allocs/op
```

## 结论

| 阶段 | 测量值 | 目标 | 状态 |
|------|--------|------|------|
| Challenge 生成 | **14.2 μs** | < 5ms | ✅（350x 余量）|
| Verify（含生成+签名+验签）| **52.8 μs** | < 5ms | ✅（94x 余量）|
| 并发 Verify | **7.7 μs** | - | 横向扩展良好 |

## 性能特征

1. **crypto/rand** —— 16 字节 nonce + 16 字节 challenge_id
2. **ed25519.Sign** —— domain_sig（32 字节签名）
3. **miniredis 写** —— 1ms 以内
4. **JSON 编码** —— 极简自实现，避免 encoding/json 反射开销

## 优化建议（如未来需要）

- 缓存 Ed25519 PublicKey 解析结果
- 替换 JSON 编码为 msgpack（~30% 加速）
- Redis 客户端连接池调优（P18.11）

## 引用

- AAP 协议规范：`docs/aap-protocol.md`
- RFC 8032 (Ed25519): https://www.rfc-editor.org/rfc/rfc8032
