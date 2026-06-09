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

---

## 🐳 镜像清单（P14 已完成）

| 镜像 | Dockerfile | 用途 | 暴露端口 |
|------|-----------|------|----------|
| `agentid-chain/gateway:<tag>` | `docker/Dockerfile.gateway` | 主网关服务 | 8080 / 9090 / 6060 |
| `agentid-chain/cli:<tag>` | `docker/Dockerfile.cli` | CLI 客户端 | — |
| `agentid-chain/migration:<tag>` | `docker/Dockerfile.migration` | DB 迁移工具（Job） | — |
| `agentid-chain/mock-chain:<tag>` | `docker/Dockerfile.mock-chain` | 模拟链节点 | 8545 / 8546 |

### 镜像 Tag 策略

每个镜像发布时携带三个 tag：

```bash
# 完整语义版本
agentid-chain/gateway:v2.0.1

# 浮动 latest（默认分支）
agentid-chain/gateway:latest

# 短 SHA 追溯
agentid-chain/gateway:sha-abc1234
```

### 单镜像使用示例

```bash
# 拉取
docker pull agentid-chain/gateway:v2.0.1

# 启动 gateway（连本地 PG/Redis）
docker run --rm \
  -p 8080:8080 -p 9090:9090 -p 6060:6060 \
  -e AGENTID_STORAGE_DB_DSN="postgres://user:pass@host.docker.internal:5432/agentid?sslmode=disable" \
  -e AGENTID_STORAGE_REDIS_ADDR="host.docker.internal:6379" \
  -e AGENTID_BACKEND_TYPE=local \
  -v $PWD/configs:/etc/agentid/configs:ro \
  agentid-chain/gateway:v2.0.1

# 一次性 register
docker run --rm \
  agentid-chain/cli:v2.0.1 \
  register --owner alice --public-key pk_xxx

# 跑数据库迁移（CI/CD InitContainer）
docker run --rm \
  -e AGENTID_STORAGE_DB_DSN="postgres://..." \
  agentid-chain/migration:v2.0.1 up
```

### 构建与推送

```bash
# 本地构建（单架构）
make docker-build

# 多架构构建 + 推送（amd64 + arm64）
make docker-buildx

# 推送镜像到仓库
make docker-push

# 用 cosign 签名（防供应链攻击）
make cosign-keygen    # 仅首次
make cosign-sign      # 签名所有镜像
make cosign-verify    # 验证签名

# 清理
make docker-clean
```

### 故障排查

| 问题 | 排查方向 |
|------|----------|
| 容器启动后立即退出 | `docker logs <container>`；检查 `AGENTID_STORAGE_DB_DSN` / `AGENTID_STORAGE_REDIS_ADDR` |
| 健康检查一直不通过 | distroless 镜像无 shell，`docker exec` 不可用；用 `docker inspect` + `docker logs` 排查 |
| 镜像体积过大 | 确认使用 distroless；`docker history <image>` 查看各层 |
| 多架构 push 失败 | 确认 `docker buildx create` 已创建；`make docker-buildx-inspect` |
| 签名验证失败 | `COSIGN_PASSWORD` 不一致；密钥文件未同步；使用 keyless（OIDC）需 `$ACTIONS_ID_TOKEN` |

### 安全

- **基础镜像**：`gcr.io/distroless/static-debian12:nonroot`（无 shell，无包管理器，无 libc）
- **运行用户**：UID 65532 (`nonroot`)，禁止 root 运行
- **CGO**：关闭（避免 glibc 依赖）
- **LDFLAGS**：`-s -w`（去除调试符号与 DWARF）
- **多架构**：linux/amd64 + linux/arm64
- **签名**：cosign keyless（GitHub OIDC）或本地密钥对
- **SBOM**：建议在 CI 中加入 `anchore/sbom-action` 生成 CycloneDX SBOM

### 镜像大小对比

| 镜像 | 基础 | 预期大小 |
|------|------|----------|
| `agentid-chain/gateway` | distroless | ~25-30 MB |
| `agentid-chain/cli` | distroless | ~25-30 MB |
| `agentid-chain/migration` | distroless | ~25-30 MB |
| `agentid-chain/mock-chain` | distroless | ~12-15 MB |

> 远低于业界常见的 80-150 MB 镜像，节省拉取时间与存储成本。
