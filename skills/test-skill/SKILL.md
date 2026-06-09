# test

> 跑 AgentID-Chain 测试套件

## 📋 描述

执行单元测试 / 集成测试 / 覆盖率检查。

**适用场景**：

- 提交前自检
- CI 本地复现
- 验证修复

## 🔧 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `scope` | string | | `unit` (默认) / `integration` / `e2e` / `all` |
| `package` | string | | 指定包路径 |
| `race` | bool | | 启用 race detector（默认 false） |
| `coverage` | bool | | 输出覆盖率（默认 false） |

## 📤 返回

```json
{
  "passed": 245,
  "failed": 0,
  "skipped": 3,
  "duration_s": 12.4,
  "coverage_pct": 73.5
}
```

## 🛠️ 实现

```python
def run_tests(scope="unit", package=None, race=False, coverage=False):
    args = {"scope": scope, "race": race, "coverage": coverage}
    if package: args["package"] = package
    return call_tool("test", args)
```

## 📚 常用模式

```bash
# 单元 + race
go test -race -count=1 ./internal/...

# 集成（需 docker）
go test -tags=integration -count=1 ./tests/integration/...

# 覆盖率
go test -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1
```
