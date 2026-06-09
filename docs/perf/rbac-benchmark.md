# RBAC 权限校验性能基准 (P18.3)

> 目标：单次权限校验 < 1μs（核心热路径，每秒需处理 100K+ 请求）
> 工具：Go 标准 `testing.Benchmark`
> 平台：Apple M5 Pro / darwin arm64
> 时间：2026-06-09

## 测试代码

参见 `internal/authz/rbac/benchmark_test.go`。

## 执行命令

```bash
go test -bench=Benchmark -benchmem -run=^$ ./internal/authz/rbac/ -benchtime=1s
```

## 结果

```
goos: darwin
goarch: arm64
pkg: github.com/agentid-chain/agentid-chain/internal/authz/rbac
cpu: Apple M5 Pro
BenchmarkCheck-15             	151958091	         8.047 ns/op	       0 B/op	       0 allocs/op
BenchmarkCheckMask-15         	158027491	         7.735 ns/op	       0 B/op	       0 allocs/op
BenchmarkMustCheck-15         	154261357	         7.939 ns/op	       0 B/op	       0 allocs/op
BenchmarkHasAny-15            	172112060	         6.848 ns/op	       0 B/op	       0 allocs/op
BenchmarkCheck_Parallel-15    	23369149	        51.29 ns/op	       0 B/op	       0 allocs/op
BenchmarkAllowedBits-15       	14134690	        84.93 ns/op	     512 B/op	       1 allocs/op
```

## 结论

| 操作 | 测量值 | 目标 | 状态 |
|------|--------|------|------|
| `Check(perm)` | **8.0 ns** | < 1μs | ✅（125x 余量）|
| `CheckMask(perms)` | **7.7 ns** | < 1μs | ✅ |
| `MustCheck(perm)` | **7.9 ns** | < 1μs | ✅ |
| `HasAny(perms)` | **6.8 ns** | < 1μs | ✅ |
| 并发 Check | **51 ns** | - | 良好 |
| `AllowedBits` | 85 ns | - | 含分配，可缓存 |

## 性能特征

1. **零分配**（除 `AllowedBits` 需返回切片）—— 完全在 CPU 寄存器完成
2. **位运算** —— `perm & allowedMask != 0` 单指令
3. **map 查找** —— 1 次 hashmap O(1) 查 level → mask
4. **无锁读** —— `RWMutex` 读锁可重入（实际写很少）

## 优化建议（如未来需要）

- 预计算常用 level 的 mask 到 atomic.Value（避免 map 查）
- `AllowedBits` 结果可缓存（变化时失效）
- 如果权限位增多（>32），考虑 SIMD

## 引用

- 权限模型：`docs/authz-model.md`
- 等级配置：`configs/agent_level.yaml`
