# 本地开发

> 单机开发环境的搭建与日常使用

## 📋 准备

| 依赖 | 版本 | 安装 |
|------|------|------|
| Go | 1.26.1+ | [go.dev](https://go.dev/dl/) |
| Docker | 24+ | [docker.com](https://www.docker.com/) |
| Docker Compose | v2+ | 随 Docker |
| PostgreSQL Client | 15+ | (可选) `psql` |
| Redis Client | 7+ | (可选) `redis-cli` |
| k6 | latest | (可选) 压测 |
| grpcurl | latest | (可选) 调试 Connect |

## 🚀 快速启动

```bash
# 1. 启动基础设施
docker-compose -f docker-compose.dev.yml up -d postgres redis

# 2. 启动业务服务（独立终端）
go run ./cmd/agentid serve

# 3. 验证
curl http://localhost:8080/live
```

## 📁 配置文件

### 默认位置
- 项目根: `configs/app.yaml` (开发)
- 用户: `~/.agentid/config.yaml` (用户偏好)

### 关键配置

```yaml
env: dev
log:
  level: debug
  format: text  # 本地用 text 友好
storage:
  backend: local
  local:
    dsn: postgres://devuser:devpass@localhost:5432/agentid?sslmode=disable
authz:
  jwt_signing_key: "dev-only-do-not-use-in-prod-32bytes!!"
cache:
  redis:
    addr: localhost:6379
```

## 🛠️ 常用命令

### 启动服务

```bash
# API Gateway
go run ./cmd/agentid serve --config configs/app.yaml

# Auth Center（独立）
go run ./cmd/auth-center serve --config auth-center2/configs/config.yaml

# Tag Sense（独立）
go run ./cmd/tag-sense serve --config tag-sense/config.yaml
```

### 编译二进制

```bash
make build
# 或
go build -o bin/agentid ./cmd/agentid
```

### 运行测试

```bash
# 单元测试
go test ./... -short

# 集成测试（需 PG/Redis）
go test ./... -count=1

# 竞态检测
go test ./... -race -count=1

# 基准
go test ./... -bench=. -benchmem

# 覆盖率
make coverage  # 或 bash scripts/check_coverage.sh
```

### 数据库操作

```bash
# 进入 PG
psql postgres://devuser:devpass@localhost:5432/agentid

# 列出表
\dt

# 查看 agents
SELECT * FROM agents LIMIT 10;

# 应用迁移
go run ./cmd/migration-tool up

# 回滚
go run ./cmd/migration-tool down --steps 1
```

### Redis 操作

```bash
# 进入 Redis
redis-cli -h localhost -p 6379

# 查看所有 key
KEYS *

# 查看 nonce
GET aap:nonce:0190a3b4-7c8d-7def-9abc-def012345678

# 清空
FLUSHDB  # 慎用！
```

### 调试

```bash
# 启用 pprof
curl http://localhost:6060/debug/pprof/  # 浏览器
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 查看 trace
go tool pprof http://localhost:6060/debug/pprof/trace?seconds=5

# 查看 goroutine
curl http://localhost:6060/debug/pprof/goroutine?debug=2 | head -50
```

### Lint

```bash
# golangci-lint
make lint
# 或
golangci-lint run ./...

# pre-commit
pre-commit run --all-files
```

## 🔄 开发循环

```
修改代码
   ↓
go build  # 编译
   ↓
go test ./... -short  # 单元测试
   ↓
go run ./cmd/agentid serve  # 运行
   ↓
curl /healthz  # 验证
   ↓
git add . && git commit -m "..."
```

## 🔌 端口冲突

```bash
# 查找占用 8080 的进程
lsof -i :8080

# 杀掉
lsof -ti :8080 | xargs kill -9
```

## 🐛 常见问题

### "connection refused" to PG

```bash
# 检查容器
docker ps | grep postgres
docker logs dev-postgres

# 重启
docker-compose -f docker-compose.dev.yml restart postgres
```

### 编译失败

```bash
# 清理缓存
go clean -cache
go mod tidy
go mod download
```

### 测试失败

```bash
# 检查 testcontainers 是否能拉取镜像
docker pull postgres:15
docker pull redis:7
```

## 📚 相关

- [部署](deployment.md)
- [配置参考](configuration.md)
- [故障排查](troubleshooting.md)
