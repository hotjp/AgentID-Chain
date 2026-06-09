# 快速开始

> 5 分钟内跑起来

## ⚡ TL;DR

```bash
# 1. 启动基础设施
docker-compose -f docker-compose.dev.yml up -d postgres redis

# 2. 启动服务
go run ./cmd/agentid serve

# 3. 验证
curl http://localhost:8080/live
```

## 📋 准备

确保已安装：

- **Go 1.26.1+** — [go.dev](https://go.dev/dl/)
- **Docker 24+** + **Docker Compose v2+** — [docker.com](https://www.docker.com/)

验证：

```bash
go version
docker --version
docker compose version
```

## 🚀 步骤

### Step 1: 克隆项目

```bash
git clone https://github.com/agentid-chain/agentid-chain.git
cd agentid-chain
```

### Step 2: 启动基础设施

```bash
docker-compose -f docker-compose.dev.yml up -d postgres redis
```

预期输出：
```
[+] Running 3/3
 ✔ Network dev_default         Created
 ✔ Container dev-postgres      Started
 ✔ Container dev-redis         Started
```

验证：
```bash
docker ps
# 应看到 dev-postgres 和 dev-redis
```

### Step 3: 启动业务服务

```bash
go run ./cmd/agentid serve
```

预期输出：
```
INFO  config loaded service=agentid-gateway env=dev
INFO  database connected service=agentid-gateway latency=15ms
INFO  redis connected service=agentid-gateway
INFO  server listening addr=:8080 service=agentid-gateway
INFO  metrics server listening addr=:9090
```

### Step 4: 验证

```bash
# 健康检查
curl http://localhost:8080/live
# → 200 OK

# 准备就绪
curl http://localhost:8080/ready
# → 200 OK

# 注册测试 Agent
curl -X POST http://localhost:8080/v1/agents \
  -H "Content-Type: application/json" \
  -d '{"owner":"alice","level":"test"}'
# → 201 Created + JSON
```

## 🎉 接下来

- 📖 [第一个 Agent](first-agent.md) — 完整注册流程
- 🏛️ [架构总览](../architecture/overview.md) — 了解设计
- 🔌 [API 文档](../api/openapi.md) — 所有端点
- 🛠️ [本地开发](../operations/local-dev.md) — 进阶开发

## 🆘 遇到问题？

| 错误 | 解决 |
|------|------|
| `bind: address already in use` | 杀掉占用 8080 端口的进程：`lsof -ti :8080 \| xargs kill -9` |
| `connection refused` (PG) | 启动容器：`docker-compose -f docker-compose.dev.yml up -d postgres` |
| `permission denied` (Docker) | 加入 `docker` 用户组或使用 `sudo` |
| 编译失败 | `go clean -cache && go mod tidy` |

详见 [故障排查](../operations/troubleshooting.md)
