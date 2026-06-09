# test 示例

## 跑单元测试 + 覆盖率

```python
result = call_tool("test", {"scope": "unit", "coverage": True})
print(f"Coverage: {result['coverage_pct']}%")
```

## 跑某个包

```python
call_tool("test", {"package": "./internal/service/...", "race": True})
```
