# rollback 示例

## 回滚应用（演练）

```python
result = call_tool("rollback", {"scope": "app", "dry_run": True})
print(f"Would revert to revision {result['to_revision']}")
```

## 实际回滚 DB

```python
call_tool("rollback", {"scope": "db", "revision": 1})
```
