# UUID v7 性能基准 (P18.1)

> 目标：单次生成 < 1μs/op
> 工具：Go 标准 `testing.Benchmark`
> 平台：Apple M5 Pro / darwin arm64
> 时间：2026-06-09

## 测试代码

参见 `internal/uuid_generator/uuid_test.go::BenchmarkGenerateV7`。

## 执行命令

```bash
go test -bench=BenchmarkGenerateV7 -benchmem -run=^$ ./internal/uuid_generator/ -benchtime=1s
```

## 结果

```
goos: darwin
goarch: arm64
pkg: github.com/agentid-chain/agentid-chain/internal/uuid_generator
cpu: Apple M5 Pro
BenchmarkGenerateV7-15    	 5253096	       205.2 ns/op	      48 B/op	       1 allocs/op
PASS
```

## 结论

| 指标 | 测量值 | 目标 | 状态 |
|------|--------|------|------|
| 延迟 | **205.2 ns/op** | < 1μs (1000ns) | ✅ 达标（5x 余量）|
| 内存 | 48 B/op | < 100 B | ✅ |
| 分配次数 | 1 allocs/op | 0-2 | ✅ |
| 吞吐 | ~4.87M ops/s | - | 高性能 |

## 性能特征

1. **crypto/rand 单次 16 字节** —— 主要成本
2. **atomic 操作** —— 无锁并发安全
3. **hex.Encode** —— 36 字节字符串，无额外分配
4. **批量 1000 优化** —— 见 `BenchmarkBatchGenerate1000`

## 优化建议（如未来需要）

- 使用 `chacha8rand` 替换 crypto/rand（~3x 加速）
- 预分配 hex 缓冲（已实现）
- 池化 Generator（当前每 goroutine 一个）

## 引用

- RFC 9562: https://www.rfc-editor.org/rfc/rfc9562
- ULID 对比：https://github.com/ulid/spec
