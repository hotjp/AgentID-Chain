# Release Process

> AgentID-Chain 完整发布流程

## 📋 概览

```
代码冻结 → RC 测试 → 构建签名 → 发布 → 灰度 → 全量
   T-3d       T-2d      T-1d      T      T+1h   T+24h
```

## 🔄 详细流程

### 1. 代码冻结（T-3 天）

- 创建 `release/vX.Y.Z` 分支
- 停止合并非关键 PR
- 仅接受 hotfix

### 2. RC 构建（T-2 天）

```bash
# 创建分支
git checkout main
git pull
git checkout -b release/v2.1.0

# 更新版本
sed -i '' 's/version = "2.0.1"/version = "2.1.0-rc.1"/' internal/version/version.go
sed -i '' 's/version: 2.0.1/version: 2.1.0-rc.1/' deploy/helm/agentid-chain/Chart.yaml

# 提交
git commit -am "chore(release): bump to 2.1.0-rc.1"
```

### 3. 测试（T-1 天）

```bash
# 单元 + 集成 + E2E
make test-all

# 质量门控
make gates

# 覆盖率
go test -coverprofile=cover.out ./...
COVERAGE=$(go tool cover -func=cover.out | tail -1 | awk '{print $3}')
[ "${COVERAGE%.*}" -ge 70 ] || (echo "Coverage $COVERAGE < 70%" && exit 1)
```

### 4. 构建 + 签名（T-1 天）

```bash
# Docker 镜像
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t agentid-chain/agentid-chain:v2.1.0 \
  -t agentid-chain/agentid-chain:latest \
  --push \
  .

# cosign 签名
cosign sign --key cosign.key agentid-chain/agentid-chain:v2.1.0

# SBOM
syft agentid-chain/agentid-chain:v2.1.0 -o spdx-json > sbom.spdx.json

# 二进制
go build -ldflags='-s -w -X main.version=2.1.0 -X main.commit=abc123' \
  -o dist/agentid-linux-amd64 \
  ./cmd/agentid
```

### 5. 发布日（T）

```bash
# 合并 release → main
git checkout main
git merge --no-ff release/v2.1.0
git push origin main

# 打 tag
git tag -s v2.1.0 -m "release v2.1.0"
git push origin v2.1.0

# 触发 GitHub Actions：
#   - 构建并推送 Docker
#   - 创建 GitHub Release
#   - 上传 SBOM 到 release assets
#   - 推送到 Docker Hub
```

### 6. 灰度部署（T+1h → T+24h）

```bash
# 1. 部署到 staging
helm upgrade agentid-chain-staging deploy/helm/agentid-chain \
  --namespace staging \
  --set image.tag=2.1.0

# 2. 监控 30 分钟
# 3. 部署到生产（先 1 集群）
helm upgrade agentid-chain deploy/helm/agentid-chain \
  --namespace agentid \
  --set image.tag=2.1.0

# 4. 监控 1 小时
# 5. 扩量（HA 集群 + 其他 region）
```

### 7. 公告

- 邮件给客户（用 `templates/customer_advisory.md`）
- Slack `#release` 公告
- 官网 changelog

## 🔧 工具脚本

```bash
# 完整 release 流程
./scripts/release.sh v2.1.0

# 仅 dry-run
./scripts/release.sh v2.1.0 --dry-run

# 跳过测试（紧急修复）
./scripts/release.sh v2.1.0 --skip-tests

# 跳过 push
./scripts/release.sh v2.1.0 --skip-push
```

## 📚 详细文档

- [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md) — 必走清单
- [MIGRATION.md](MIGRATION.md) — 升级迁移
- [ROLLBACK.md](ROLLBACK.md) — 紧急回滚
- [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md) — 事故响应
- [postmortem 模板](../templates/postmortem.md) — 复盘
- [customer_advisory 模板](../templates/customer_advisory.md) — 客户公告
