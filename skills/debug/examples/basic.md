# debug 示例

## 抓 30s CPU profile

```python
result = call_tool("debug", {"action": "pprof_cpu", "duration": "30s"})
print(f"Saved to: {result['output']}")
```

## 按 trace_id 拉日志

```python
result = call_tool("debug", {"action": "capture_trace", "trace_id": "abc123"})
```
