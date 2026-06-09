# 发布流程

> 从代码到生产的完整流程

## 🎯 发布模型

采用 **语义化版本** (SemVer 2.0) + **GitHub Flow**：

```
main 分支
   ↓
PR (含 Conventional Commits)
   ↓
merge → main
   ↓
打 tag (vX.Y.Z)
   ↓
GitHub Actions 自动发布
   ↓
GitOps 同步到生产
```

## 📋 版本号规则

格式：`vMAJOR.MINOR.PATCH` (如 `v2.0.1`)

| 类型 | 何时 bump | 示例 |
|------|---------|------|
| **MAJOR** | 破坏性变更（API 不兼容） | v2.0.0 → v3.0.0 |
| **MINOR** | 新功能（向后兼容） | v2.0.0 → v2.1.0 |
| **PATCH** | Bug 修复 | v2.0.1 → v2.0.2 |
| **预发布** | RC / Beta | v2.1.0-rc.1 |

## 🚀 发布流程

### 1. 准备

```bash
# 1.1 拉取最新 main
git checkout main
git pull origin main

# 1.2 验证工作区干净
git status

# 1.3 运行发布前自检
./scripts/pre-release-check.sh
```

期望输出：`✅ 可以发布`

### 2. 选择版本类型

```bash
# 补丁（bug 修复）
./scripts/release.sh patch

# 次版本（新功能）
./scripts/release.sh minor

# 主版本（破坏性变更）
./scripts/release.sh major

# 指定版本
./scripts/release.sh v2.0.2

# 演练（不实际修改）
./scripts/release.sh patch --dry-run
```

### 3. 脚本自动执行

```
1. 验证环境（main 分支 / 工作区干净）
2. 自检（test / lint / build）
3. 解析版本号
4. 更新 CHANGELOG.md
5. 提交 + tag
6. 构建多平台二进制
7. 生成 SBOM
8. Cosign 签名
9. 推送 commit + tag
```

### 4. GitHub Actions 自动执行

`.github/workflows/release.yml` 触发后：

```
1. test (Test & Lint)
   - go test ./... -race
   - 覆盖率检查
   - golangci-lint
2. build (多平台二进制)
   - linux/amd64, linux/arm64
   - darwin/amd64, darwin/arm64
   - windows/amd64
   - SBOM 生成（syft）
3. docker (镜像)
   - 4 个镜像：agentid, gateway, auth, tag-sense
   - 多平台（linux/amd64, linux/arm64）
   - Cosign 签名（keyless OIDC）
   - SBOM 证明
4. helm (Chart 打包)
   - helm lint
   - helm package
5. release (GitHub Release)
   - 收集所有 artifact
   - 自动生成 release notes
   - 上传 .tgz, .spdx.json
   - Slack 通知
```

### 5. GitOps 部署

ArgoCD 检测到 Helm Chart tag 变化：

```
- 同步到 staging（自动）
- 同步到 production（手动 approve）
```

## 🔍 发布前检查清单

- [ ] 所有 PR 已合并
- [ ] `main` 分支 CI 全部通过
- [ ] 文档已更新
- [ ] CHANGELOG.md 已更新
- [ ] 性能回归测试通过（如适用）
- [ ] 安全扫描通过
- [ ] 镜像扫描通过
- [ ] 升级路径测试（如 MAJOR）

## ⚠️ Hotfix 流程

紧急修复（生产故障）：

```bash
# 1. 从 main 创建 hotfix 分支
git checkout -b hotfix/fix-xxx main

# 2. 修复 + 测试
git commit -m "fix(...): ..."

# 3. 推送 + PR
git push origin hotfix/fix-xxx
# 在 GitHub 创建 PR（快速 review）

# 4. 合并后立即打 patch tag
./scripts/release.sh patch

# 5. GitHub Actions 自动发布
# 6. ArgoCD 同步到生产
```

## 🔄 回滚

```bash
# Helm 回滚到上一版本
helm rollback agentid-chain

# 或部署指定版本
helm install agentid-chain deploy/helm/agentid-chain/ \
  --version 2.0.0
```

## 📊 发布后验证

```bash
# 1. 健康检查
curl https://api.agentid-chain.example.com/live

# 2. 指标验证（版本号）
curl https://api.agentid-chain.example.com:9090/metrics | grep version

# 3. 注册测试 agent
go run ./cmd/agentid register --owner smoketest --level test

# 4. 查看 30min 监控
open https://grafana/d/agentid-chain-overview

# 5. 错误率 < 0.1%
open https://prometheus/alerts
```

## 🛠️ 工具

| 工具 | 用途 |
|------|------|
| `scripts/release.sh` | 本地发布 |
| `scripts/pre-release-check.sh` | 发布前自检 |
| `.github/workflows/release.yml` | CI 自动发布 |
| `deploy/gitops/` | GitOps 部署 |
| `helm` | Chart 打包 |

## 🔐 密钥管理

发布所需 secrets（在 GitHub Settings → Secrets）：

| Secret | 用途 |
|--------|------|
| `GITHUB_TOKEN` | 自动提供 |
| `SLACK_BOT_TOKEN` | Slack 通知 |
| `SLACK_CHANNEL_ID` | 通知频道 |
| `CODECOV_TOKEN` | 覆盖率上报（如使用） |
| `ALCHEMY_KEY` | 链上 RPC（仅 hybrid） |

## 📚 相关

- [Conventional Commits](https://www.conventionalcommits.org/)
- [SemVer](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [Helm 文档](https://helm.sh/docs/)
- [ArgoCD 文档](https://argo-cd.readthedocs.io/)
