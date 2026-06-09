# debug

> 调试 AgentID-Chain 运行时问题

## 📋 描述

打开调试模式、抓取 trace、定位瓶颈。

**适用场景**：

- 复现 bug
- 性能分析
- 死锁排查

## 🔧 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `action` | string | ✅ | `enable_debug` / `capture_trace` / `pprof_cpu` / `pprof_heap` / `pprof_goroutine` |
| `duration` | string | | pprof 时长（默认 30s） |
| `trace_id` | string | | capture_trace 时指定 |
| `output` | string | | 输出文件路径 |

## 📤 返回

```json
{
  "action": "pprof_cpu",
  "output": "/tmp/cpu.prof",
  "duration_s": 30,
  "url": "http://localhost:6060/debug/pprof/profile?seconds=30"
}
```

## 🛠️ 实现

```python
def debug_run(action, duration="30s", trace_id=None, output=None):
    args = {"action": action, "duration": duration}
    if trace_id: args["trace_id"] = trace_id
    if output: args["output"] = output
    return call_tool("debug", args)
```

## 📚 常用分析

```bash
# CPU profile
go tool pprof -http=:8083 http://localhost:6060/debug/pprof/profile?seconds=30

# Heap
go tool pprof -http=:8083 http://localhost:6060/debug/pprof/heap

# Goroutine（死锁）
curl http://localhost:6060/debug/pprof/goroutine?debug=2
```
