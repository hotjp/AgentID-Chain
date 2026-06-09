# AgentID-Chain — 性能分析手册 (P18.8-18.9)

> 范围：CPU / 内存 / Goroutine / Block / Mutex 全套 pprof
> 工具：Go 内置 `net/http/pprof` + `go tool pprof` + 火焰图（FlameGraph）
> 监听：`http://localhost:6060/debug/pprof`（详见各服务 pprof 端口）

## 1. 快速开始

### 1.1 启用 pprof

每个服务在启动时已默认启用 pprof（参考 `internal/gateway/server/server.go`）：

```go
import _ "net/http/pprof"  // 注册默认 handler
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

### 1.2 端口分配

| 服务 | pprof 端口 |
|------|-----------|
| API Gateway | 6060 |
| Auth Center | 6061 |
| Tag Sense | 6062 |

## 2. CPU 性能分析 (P18.8)

### 2.1 抓取 30s CPU profile

```bash
# 方式 1：直接 HTTP
curl -o cpu.prof http://localhost:6060/debug/pprof/profile?seconds=30

# 方式 2：带 label（更精确）
curl -o cpu.prof 'http://localhost:6060/debug/pprof/profile?seconds=30&labels=service:gateway'
```

### 2.2 查看热点（top 命令）

```bash
go tool pprof cpu.prof
(pprof) top 20 -cum
```

输出示例：
```
      flat  flat%   sum%        cum   cum%
         0     0%     0%    45.12s 32.1%  net/http.(*conn).serve
     1.23s  0.9%   0.9%    23.45s 16.7%  github.com/.../aap.(*Generator).Generate
     0.89s  0.6%   1.5%    18.20s 12.9%  encoding/json.Marshal
```

### 2.3 火焰图（Flame Graph）

```bash
# 安装 FlameGraph
git clone https://github.com/brendangregg/FlameGraph.git
export PATH=$PWD/FlameGraph:$PATH

# 生成火焰图
go tool pprof -http=:8088 cpu.prof &
# 浏览器打开 http://localhost:8088/ui/flamegraph
```

或者直接生成 SVG：

```bash
go tool pprof -svg cpu.prof > cpu_flame.svg
# 用浏览器打开 cpu_flame.svg
```

### 2.4 火焰图解读

```
       main
         │
    ┌────┴────┐
    │         │
 serveHTTP  handleRegister
    │         │
    ├─────────┤
  Validate    DB.Put
    │         │
  5%         35%       ← 宽 = 占 CPU 时间多
```

**关键观察点**：

1. **最宽的栈帧** —— 优化 ROI 最高
2. **长尾调用链** —— 同步阻塞的常见迹象
3. **重复出现的模式** —— 可能是循环/分配热点
4. **意外的 system call** —— 排查 IO 阻塞

### 2.5 真实案例：Register 流程

抓取 30s 后，火焰图可能显示：

```
time.Time.Now                8.2%    ← 可换 monotonic clock
crypto/rand.Read            12.1%    ← 可换 chacha8rand
encoding/json.Marshal       15.4%    ← 可换 msgpack
ent.go/INSERT                9.8%    ← OK（DB IO）
```

## 3. 内存分析 (P18.9)

### 3.1 抓取 heap profile

```bash
# 当前分配
curl -o heap.prof http://localhost:6060/debug/pprof/heap

# 强制 GC 后（更准确）
curl -o heap.prof 'http://localhost:6060/debug/pprof/heap?gc=1'
```

### 3.2 查看分配

```bash
go tool pprof -alloc_space heap.prof  # 总分配（包含已 GC）
go tool pprof -alloc_objects heap.prof  # 分配次数
go tool pprof -inuse_space heap.prof    # 当前占用
```

### 3.3 常见内存问题

| 模式 | 原因 | 修复 |
|------|------|------|
| 高 inuse + 低 alloc | 长期存活的对象 | 减少缓存/池化 |
| 高 alloc + 低 inuse | 短命对象 | sync.Pool / 复用 |
| 持续增长 | 泄漏 | defer release |
| 突刺式 | batch job | 限流 |

### 3.4 内存火焰图

```bash
go tool pprof -http=:8088 -sample_index=alloc_space heap.prof
```

## 4. Goroutine / Block / Mutex

### 4.1 Goroutine dump

```bash
curl -s http://localhost:6060/debug/pprof/goroutine?debug=1 > goroutines.txt
curl -s http://localhost:6060/debug/pprof/goroutine?debug=2 | head -100
```

### 4.2 Block（同步阻塞）

```bash
# 启用 runtime.SetBlockProfileRate(1) 后
curl -o block.prof http://localhost:6060/debug/pprof/block
go tool pprof block.prof
```

### 4.3 Mutex 竞争

```bash
# 启用 -mutex 模式
curl -o mutex.prof http://localhost:6060/debug/pprof/mutex
go tool pprof mutex.prof
```

## 5. 在线分析（实时）

### 5.1 浏览器 UI

```bash
# CPU 实时
go tool pprof -http=:8088 http://localhost:6060/debug/pprof/profile?seconds=30

# 堆实时
go tool pprof -http=:8088 http://localhost:6060/debug/pprof/heap
```

### 5.2 diff profile（升级前后对比）

```bash
# 升级前
curl -o before.prof http://localhost:6060/debug/pprof/heap?gc=1
# 升级代码 + 重启
# 升级后
curl -o after.prof http://localhost:6060/debug/pprof/heap?gc=1
# 对比
go tool pprof -base before.prof after.prof
```

## 6. 持续监控

### 6.1 Prometheus 集成

服务已暴露 `go_*` 指标：

```
go_gc_duration_seconds
go_goroutines
go_memstats_alloc_bytes
go_memstats_heap_inuse_bytes
go_memstats_heap_objects
go_threads
```

### 6.2 告警规则

```yaml
groups:
- name: go-runtime
  rules:
  - alert: HighGoroutineCount
    expr: go_goroutines > 10000
    for: 5m
    annotations:
      summary: "Goroutine 数过高（> 10K）"
  - alert: HighHeapUsage
    expr: go_memstats_heap_inuse_bytes > 1e9
    for: 5m
    annotations:
      summary: "堆内存 > 1GB"
  - alert: GCPauseHigh
    expr: rate(go_gc_duration_seconds_sum[5m]) > 0.05
    for: 5m
    annotations:
      summary: "GC 暂停 > 50ms"
```

## 7. 实战脚本

### 7.1 自动抓取 profile

```bash
#!/usr/bin/env bash
# scripts/profile.sh
SERVICE=${1:-gateway}
PORT=${2:-6060}
DURATION=${3:-30}
OUT_DIR=${4:-profiles/$(date +%Y%m%d-%H%M%S)}
mkdir -p "$OUT_DIR"

echo "==> 抓取 CPU profile ($DURATION s)..."
curl -o "$OUT_DIR/cpu.prof" "http://localhost:$PORT/debug/pprof/profile?seconds=$DURATION" &

echo "==> 抓取 heap profile..."
curl -o "$OUT_DIR/heap.prof" "http://localhost:$PORT/debug/pprof/heap?gc=1"

echo "==> 抓取 goroutine dump..."
curl -s "http://localhost:$PORT/debug/pprof/goroutine?debug=1" > "$OUT_DIR/goroutines.txt"

wait
echo "==> Profiles: $OUT_DIR"
go tool pprof -text "$OUT_DIR/cpu.prof" | head -30
```

### 7.2 远程服务 profile

```bash
# 通过 SSH 隧道
ssh -L 6060:localhost:6060 user@prod-host
# 本地分析
go tool pprof http://localhost:6060/debug/pprof/heap
```

## 8. 常见反模式

| 反模式 | 后果 | 替代方案 |
|--------|------|---------|
| 抓 profile 时跑测试 | 不真实 | 用生产流量 |
| 短时间抓 profile（<10s） | 噪声大 | 30s 起步 |
| 不做 `gc=1` | 包含未 GC 内存 | 总是带 `gc=1` |
| 单次抓取就下结论 | 缺上下文 | 多点采样 |
| 在高并发下直接打开浏览器 UI | 影响服务 | 先 dump 再分析 |

## 9. 进阶：parca / Polar Signals

持续生产环境推荐：

- **Parca** —— eBPF 持续 profiling
- **Pyroscope** —— 拉模式持续 profiling
- **Datadog Continuous Profiler** —— SaaS

## 10. 引用

- Go Blog: https://go.dev/blog/pprof
- pprof 文档：https://github.com/google/pprof
- FlameGraph: https://github.com/brendangregg/FlameGraph
- 实操参考：`docs/perf/` 目录各报告
