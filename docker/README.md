# AgentID-Chain — Docker 部署说明 (v2.0.1)

> 本目录包含 **三套** docker compose 配置，覆盖从「开发调试」到「企业内网」到「混合链上」的全场景。
>
> 端口规划统一遵循仓库根 `CLAUDE.md` 的 **尾号规则**（`80x` HTTP, `90x` Metrics, `60x` Pprof）。

---

## 📁 文件清单

| 文件 | 用途 | 端口暴露 | 启动 profile |
|------|------|---------|-------------|
| `docker-compose.dev.yml` | **开发模式**：基础设施 + mock-chain + 业务占位 | PG/Redis 暴露宿主机；业务 in `with-gateway` profile | 默认仅基础设施；`--profile with-gateway` 启业务 |
| `docker-compose.local.yml` | **企业内网 / local-only**：强制 `BACKEND_TYPE=local`，CLI 随 gateway 启动 | 业务端口暴露；PG/Redis **不暴露** | 业务默认启；无 profile |
| `docker-compose.hybrid.yml` | **混合模式**：身份走 mock-chain，权限/审计走 PG/Redis | 业务端口暴露；PG/Redis **不暴露** | 业务默认启；无 profile |

---

## 🚀 快速开始

### 1. 基础设施起停（最常用）

```bash
# 起基础设施（postgres + redis）
docker compose -f docker/compose/docker-compose.dev.yml up -d

# 查看状态
docker compose -f docker/compose/docker-compose.dev.yml ps

# 看日志
docker compose -f docker/compose/docker-compose.dev.yml logs -f

# 停
docker compose -f docker/compose/docker-compose.dev.yml down
```

> 首次启动时 `pg_data` 卷为空，`scripts/init-databases.sql` 会自动执行，
> 创建 5 个库：`agentid / apigateway / authcenter / tagsense / agentid_audit`。

### 2. 启用业务服务（with-gateway profile）

```bash
# 在 dev 基础上启用 gateway / auth-center / tag-sense 占位
docker compose -f docker/compose/docker-compose.dev.yml --profile with-gateway up -d
```

### 3. 切到企业内网模式

```bash
# 关掉 dev 套
docker compose -f docker/compose/docker-compose.dev.yml down

# 起 local-only 套（自带业务 + CLI）
docker compose -f docker/compose/docker-compose.local.yml up -d
```

### 4. 切到混合链上模式

```bash
# 关掉 dev 套
docker compose -f docker/compose/docker-compose.dev.yml down

# 起 hybrid 套（自带 mock-chain + 业务）
docker compose -f docker/compose/docker-compose.hybrid.yml up -d
```

---

## 🌐 端口速查表

| 端口 | 服务 | dev | local | hybrid |
|------|------|-----|-------|--------|
| `5432` | PostgreSQL | ✅ 暴露 | ❌ 仅容器 | ❌ 仅容器 |
| `6379` | Redis | ✅ 暴露 | ❌ 仅容器 | ❌ 仅容器 |
| `8545` | mock-chain (P3.8) | ✅ 仅容器 | — | ❌ 仅容器 |
| `8080` | Gateway HTTP/gRPC | profile 启 | ✅ 暴露 | ✅ 暴露 |
| `9090` | Gateway Metrics | profile 启 | ✅ 暴露 | ✅ 暴露 |
| `6060` | Gateway Pprof | profile 启 | ✅ 暴露 | ✅ 暴露 |
| `8081` | Auth Center HTTP | profile 启 | ✅ 暴露 | ✅ 暴露 |
| `9091` | Auth Center Metrics | profile 启 | ✅ 暴露 | ✅ 暴露 |
| `6061` | Auth Center Pprof | profile 启 | ✅ 暴露 | ✅ 暴露 |
| `8082` | Tag Sense HTTP | profile 启 | ✅ 暴露 | ❌ (hybrid 未含) |
| `9092` | Tag Sense Metrics | profile 启 | ✅ 暴露 | ❌ (hybrid 未含) |
| `6062` | Tag Sense Pprof | profile 启 | ✅ 暴露 | ❌ (hybrid 未含) |

> 当前所有业务镜像为 P5 接入前的 **alpine 占位**（`sleep infinity`）。
> 真正镜像 `agentid-chain/{gateway,auth-center,tag-sense}:v2.0.1` 由 P5 阶段构建。

---

## 🔑 环境变量加载

```bash
# 用 .env.example 加载（示例值）
docker compose --env-file .env.example -f docker/compose/docker-compose.hybrid.yml config

# 真正的本地配置
cp .env.example .env
# 编辑 .env，填入真实值（不要提交）
docker compose --env-file .env -f docker/compose/docker-compose.hybrid.yml up -d
```

compose 文件中只引用 `${AGENTID_API_KEY}` 和 `${AGENTID_PRIVATE_KEY}` 两个变量。
其它 `AGENTID_*` 由 Go 进程在运行时通过 koanf/env provider 加载。

---

## 💾 数据备份 / 恢复

### 备份

```bash
# 备份 PG 全库
docker exec dev-postgres pg_dumpall -U agentid > backups/pg-$(date +%Y%m%d).sql

# 备份 redis（持久化卷即备份，appendonly 模式数据安全）
docker run --rm -v agentid_pg_data:/data -v $(pwd)/backups:/backup alpine \
  tar czf /backup/pg-data-$(date +%Y%m%d).tar.gz /data
```

### 恢复

```bash
# 恢复 PG（注意：会先 drop 再 restore）
cat backups/pg-20260101.sql | docker exec -i dev-postgres psql -U agentid

# 恢复 redis（替换 appendonly.aof 后重启 redis）
docker cp backups/appendonly.aof dev-redis:/data/
docker compose -f docker/compose/docker-compose.dev.yml restart redis
```

---

## 🛠️ 调试入口

```bash
# 进 postgres shell
docker exec -it dev-postgres psql -U agentid -d agentid

# 进 redis cli
docker exec -it dev-redis redis-cli

# 看 mock-chain 日志
docker logs -f dev-mock-chain

# 看 init script 执行日志（仅首次启动有效）
docker logs dev-postgres 2>&1 | grep -A5 "PostgreSQL init process complete"
```

---

## 🩺 常见 Troubleshooting

### 1. 端口 `8080` / `9090` 等被占用

```bash
# 查占用
lsof -i :8080

# 解决：杀掉占用进程，或修改 compose 中 ports 映射（如 8080:8080 → 18080:8080）
```

### 2. `dev-postgres` 启动报 "container name already in use"

之前的同名容器没清理。`dev-`/`local-`/`hybrid-` 三个 compose 用了独立前缀以避免冲突。
如果意外冲突，删掉旧容器：

```bash
docker rm -f dev-postgres dev-redis dev-mock-chain
docker compose -f docker/compose/docker-compose.dev.yml up -d
```

### 3. `init-databases.sql` 没生效 / 数据库没建出来

该脚本**只在 `pg_data` 卷为空时**首次启动执行（postgres 镜像官方约定）。
如果卷已有数据，需要清理：

```bash
# ⚠️ 谨慎：会清空所有数据
docker compose -f docker/compose/docker-compose.dev.yml down -v
docker compose -f docker/compose/docker-compose.dev.yml up -d
```

### 4. `mock-chain` 健康检查一直 starting

alpine 镜像下 `wget` 默认没装。已在 P1.6 修复（用 `apk add --no-cache python3` + 内联 http server）。
如果遇到 `wget: can't connect to remote host: Connection refused`，等待 15s（start_period）后再检查。

### 5. healthcheck 用 `localhost` 失败

部分 alpine 镜像 `/etc/hosts` 不含 `localhost` 映射。改用 `127.0.0.1` 即可。
P1.6 的 mock-chain healthcheck 已用 `127.0.0.1:8545` 而不是 `localhost:8545`。

### 6. `with-gateway` profile 没启导致 8080 连不上

这是预期行为。dev compose 默认不启业务服务，避免无业务镜像时启动报错。
要启业务：

```bash
docker compose -f docker/compose/docker-compose.dev.yml --profile with-gateway up -d
```

或者直接用 `local.yml` / `hybrid.yml`（业务默认启）。

---

## 🧹 清理

```bash
# 停 dev 套（保留数据卷）
docker compose -f docker/compose/docker-compose.dev.yml down

# 停 dev 套并清数据卷（⚠️ 数据丢失）
docker compose -f docker/compose/docker-compose.dev.yml down -v

# 一次清干净
make clean-all    # 等价于: docker compose ... down -v + make clean
```

---

## 📋 下一步

- **P3.8** 完成后，`mock-chain` 镜像将替换为 `agentid-chain/mock-chain:v2.0.1`（实现 EVM JSON-RPC 子集）
- **P5** 完成后，`gateway / auth-center / tag-sense` 镜像将替换为真实业务镜像
- **P8** 完成后，将新增 CI 推送镜像到 Docker Hub 的工作流
