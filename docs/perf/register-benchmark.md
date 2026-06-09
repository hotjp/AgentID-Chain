# Register 流程性能基准 (P18.4)

> 目标：单次注册 < 50ms（含 mock DB；真实 PG 约 5-20ms）
> 工具：Go 标准 `testing.Benchmark`
> 后端：进程内 mock store + mock chain + mock audit + mock provider
> 平台：Apple M5 Pro / darwin arm64
> 时间：2026-06-09

## 测试代码

参见 `internal/service/register_benchmark_test.go`。

## 执行命令

```bash
go test -bench=BenchmarkRegister -benchmem -run=^$ ./internal/service/ -benchtime=1s
```

## 结果

```
goos: darwin
goarch: arm64
cpu: Apple M5 Pro
BenchmarkRegister-15             	   94279	     11427 ns/op	    1663 B/op	      16 allocs/op
BenchmarkRegister_Parallel-15    	  498471	      2338 ns/op	    1742 B/op	      16 allocs/op
```

## 结论

| 场景 | 测量值 | 目标 | 状态 |
|------|--------|------|------|
| 串行 Register | **11.4 μs** | < 50ms | ✅（4400x 余量）|
| 并发 Register | **2.3 μs** | - | 高吞吐 425K ops/s |

## 真实 PG 预期

mock store 仅含内存 map；真实 PG（+ ent + 事务）典型延迟：

| 操作 | 预期延迟 |
|------|---------|
| 1 个 INSERT (agents) | 1-3ms |
| 1 个 INSERT (permissions) | 1-3ms |
| 1 个 INSERT (outbox) | 1-3ms |
| 链上 RegisterAgent | 50-200ms（取决于 chain）|
| **合计（local backend）** | **3-10ms** |
| **合计（onchain backend）** | **50-200ms** |

## 性能特征

1. **零反射** —— 全部强类型 + 结构体
2. **UUID 编码** —— hex 16 字节 → 32 字符串
3. **ed25519 公钥编码** —— base64 32 字节
4. **mock store 零延迟** —— map 写
5. **mock chain 零延迟** —— 直接返回

## 优化建议（如未来需要）

- 批量注册走 BatchRegister（已有 P6.4）
- 写权限位合并到 PutAgent（减少 1 个 SQL）
- 链上注册改为异步 fire-and-forget

## 引用

- 注册工作流：`docs/workflows/register.md`
- 事务边界：`docs/architecture.md#l4-service`
