# docker-deploy

> 部署 AgentID-Chain 到 Docker / Docker Compose

## 📋 描述

一键拉起开发或测试环境的 Docker 部署。

**适用场景**：

- 本地开发
- CI 测试
- 演示环境

## 🔧 参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `mode` | string | | `dev` (默认) / `demo` / `test` |
| `pull` | bool | | 是否拉取最新镜像（默认 false） |
| `detached` | bool | | 后台启动（默认 true） |

## 📤 返回

```json
{
  "containers": [
    {"name": "dev-postgres", "status": "running", "port": "5432"},
    {"name": "dev-redis", "status": "running", "port": "6379"},
    {"name": "dev-api-gateway", "status": "running", "port": "8080"}
  ],
  "compose_file": "docker-compose.dev.yml"
}
```

## 🛠️ 实现

```python
def docker_deploy(mode="dev", pull=False, detached=True):
    return call_tool("docker_deploy", {
        "mode": mode, "pull": pull, "detached": detached
    })
```

## 📚 常用命令

```bash
# 查看日志
docker-compose -f docker-compose.dev.yml logs -f api-gateway

# 停止
docker-compose -f docker-compose.dev.yml down

# 清理数据
docker-compose -f docker-compose.dev.yml down -v
```
