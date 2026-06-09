# docker-deploy 示例

## 启动开发环境

```python
result = call_tool("docker_deploy", {"mode": "dev"})
for c in result["containers"]:
    print(f"  {c['name']}: {c['status']} (:{c['port']})")
```

## 拉取最新镜像后启动

```python
call_tool("docker_deploy", {"mode": "demo", "pull": True})
```
