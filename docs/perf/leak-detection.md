# Go 内存泄漏检测实战 (P18.9 补充)

> 配套：`docs/PROFILING.md` 第 3 节
> 工具：`runtime/pprof` + `go tool pprof` + 实验性 `goleak`

## 1. 内存泄漏的常见征兆

| 症状 | 检测方式 |
|------|---------|
| `go_memstats_heap_inuse_bytes` 持续增长 | Prometheus 告警 |
| RSS 持续增长 | `ps aux` 监控 |
| GC 频率增加但释放内存少 | `GODEBUG=gctrace=1` |
| 重启后立刻恢复 | diff 前后 heap profile |

## 2. 三种 heap profile 视角

### 2.1 `inuse_space` —— 当前占用

```bash
go tool pprof -inuse_space heap.prof
(pprof) top 10
(pprof) list myFunc
```

适用：**长期存活对象泄漏**（缓存未清、goroutine 卡住、defer 漏写）。

### 2.2 `alloc_space` —— 累计分配

```bash
go tool pprof -alloc_space heap.prof
(pprof) top 10
```

适用：**短命对象频繁创建**（JSON 重复序列化、临时切片）。

### 2.3 `alloc_objects` —— 分配次数

```bash
go tool pprof -alloc_objects heap.prof
```

适用：**小对象高频分配**（指针、大 map 元素）。

## 3. 泄漏检测流程

### 3.1 步骤 1：触发可疑操作

```bash
# 模拟用户行为
hey -n 10000 -c 50 http://localhost:8080/v1/agents/register
```

### 3.2 步骤 2：抓 3 次 heap profile（间隔 1 分钟）

```bash
for i in 1 2 3; do
  curl -o heap-$i.prof "http://localhost:6060/debug/pprof/heap?gc=1"
  sleep 60
done
```

### 3.3 步骤 3：对比相邻时间点

```bash
# 第一次 vs 第二次
go tool pprof -base heap-1.prof heap-2.prof
(pprof) top 20 -cum

# 第二次 vs 第三次
go tool pprof -base heap-2.prof heap-3.prof
(pprof) top 20 -cum
```

如果两次都出现相同的增长函数 → **高度怀疑泄漏**。

### 3.4 步骤 4：定位代码

```bash
(pprof) list candidateFunc
(pprof) peek candidateFunc
```

## 4. 常见泄漏模式

### 4.1 切片引用未释放

```go
// 泄漏：s 一直保留着 largeBuf
var s [][]byte
func Add() {
    buf := make([]byte, 1<<20)
    s = append(s, buf[:100])  // 整个 buf 都不会被 GC
}

// 修复
var s [][]byte
func Add() {
    buf := make([]byte, 1<<20)
    s = append(s, append([]byte(nil), buf[:100]...))  // 复制
}
```

### 4.2 channel 发送阻塞

```go
// 泄漏：ch 永远没人接收
func Leak() {
    ch := make(chan int)
    go func() { ch <- 1 }()  // 永久阻塞
}

// 修复
func Safe() {
    ch := make(chan int, 1)  // 缓冲
    ch <- 1
}
```

### 4.3 Context 未传递

```go
// 泄漏：ctx cancel 后 goroutine 不退出
func Slow(ctx context.Context) {
    go func() {
        for {
            doWork()  // 没有 select ctx.Done()
        }
    }()
}

// 修复
func Safe(ctx context.Context) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            default:
                doWork()
            }
        }
    }()
}
```

### 4.4 defer 顺序

```go
// 泄漏：defer 顺序反了
func Close(conn, body) {
    defer body.Close()  // 先关 body 才能释放连接
    defer conn.Close()  // 后关 conn
}

// 反例：先关 conn → body 永远关不上
```

### 4.5 sync.Pool 不当使用

```go
// 误用：pool 中对象长期存活
var pool = sync.Pool{New: func() any { return &Big{Data: make([]byte, 1<<20)} }}

pool.Put(&Big{})  // Data 不会被 GC

// 正确：使用前清空
func Use() {
    b := pool.Get().(*Big)
    defer func() {
        b.Data = b.Data[:0]  // 清空引用
        pool.Put(b)
    }()
}
```

## 5. 自动泄漏检测

### 5.1 goleak 库

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}

func TestNoLeak(t *testing.T) {
    defer goleak.VerifyNone(t)
    // ... 测试代码
}
```

### 5.2 持续监控脚本

```bash
#!/usr/bin/env bash
# scripts/leak-monitor.sh
# 每分钟采样 goroutine 数 + heap，告警异常增长
while true; do
  GOROUTINES=$(curl -s http://localhost:6060/debug/pprof/goroutine?debug=1 | grep -c "^goroutine ")
  HEAP=$(curl -s http://localhost:6060/debug/pprof/heap?debug=1 | grep "HeapAlloc" | awk '{print $2}')
  echo "[$(date +%H:%M:%S)] goroutines=$GOROUTINES heap=$HEAP"
  sleep 60
done
```

## 6. 案例：AAP challenge 内存增长

### 现象
- 压测 1 分钟后，heap 从 100MB 增长到 800MB
- 不重启则持续增长

### 排查
```bash
curl -o before.prof http://localhost:6060/debug/pprof/heap?gc=1
# 压测 1 分钟
hey -n 10000 -c 100 http://localhost:6060/debug/pprof/profile
curl -o after.prof http://localhost:6060/debug/pprof/heap?gc=1

go tool pprof -base before.prof after.prof
(pprof) top 20
```

### 根因
`encodeChallenge` 使用 `fmt.Sprintf` + `json.Unmarshal` 循环创建大量临时字符串。

### 修复
- 替换为 `json.Marshal`/`json.Unmarshal`（用 sync.Pool 缓存 `bytes.Buffer`）
- 移除 `fmt.Sprintf` 中的字符串拼接

### 验证
- 修复后再次压测 10 分钟
- heap 在 200MB 左右稳定
- GC 频率从每秒 5 次降到每秒 1 次

## 7. 指标

| 指标 | 健康值 | 告警阈值 |
|------|--------|---------|
| `go_memstats_heap_inuse_bytes` | < 500MB | > 1GB |
| `go_goroutines` | < 5000 | > 10000 |
| `rate(go_gc_duration_seconds_sum[5m])` | < 0.01 | > 0.05 |
| `go_gc_count` per minute | < 10 | > 60 |

## 8. 引用

- Go 内存模型：https://go.dev/ref/mem
- `runtime/pprof`: https://pkg.go.dev/runtime/pprof
- `goleak`: https://github.com/uber-go/goleak
