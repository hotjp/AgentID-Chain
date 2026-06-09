# Release Checklist

> 每次发布前必走流程

## 📋 预发布（T-3 天）

- [ ] **代码冻结**（main 分支）
- [ ] **RC 构建**：从 main 切 `release/vX.Y.Z` 分支
- [ ] **更新版本号**：`internal/version/version.go` + Helm `Chart.yaml`
- [ ] **更新 CHANGELOG.md**：所有变更归类
- [ ] **跑全部测试**：
  - [ ] `go test -race -count=1 ./...`
  - [ ] `go test -tags=integration ./tests/integration/...`
  - [ ] `go test -tags=e2e ./tests/e2e/...`
- [ ] **跑质量门控**：
  - [ ] `golangci-lint run`
  - [ ] `gosec -severity=high ./...`
  - [ ] `govulncheck ./...`
  - [ ] `go run ./cmd/constitution-gates`
- [ ] **覆盖率 ≥ 70%**：`go test -coverprofile=cover.out ./...`
- [ ] **文档同步**：所有新增/变更 API 已有文档
- [ ] **ADR 同步**：架构决策已记录

## 📦 构建（T-1 天）

- [ ] **Docker 镜像**：
  - [ ] `docker buildx build --platform linux/amd64,linux/arm64 -t agentid-chain:vX.Y.Z .`
  - [ ] cosign 签名：`cosign sign --key cosign.key agentid-chain:vX.Y.Z`
  - [ ] SBOM：`syft . -o spdx-json=sbom.spdx.json`
- [ ] **Helm chart**：`helm package deploy/helm/agentid-chain/`
- [ ] **二进制**：`go build -ldflags='-X main.version=X.Y.Z' -o dist/agentid ./cmd/agentid`

## ✅ 发布日（T）

- [ ] **最终 review**：2 个 reviewer 批准
- [ ] **合并 release 分支 → main**：squash merge
- [ ] **打 tag**：`git tag -s vX.Y.Z -m "release vX.Y.Z"`
- [ ] **推送 tag**：`git push origin vX.Y.Z`
- [ ] **GitHub Release**：基于 tag 自动生成 release notes
- [ ] **Docker Hub 推送**：`docker push agentid-chain/agentid-chain:vX.Y.Z`
- [ ] **Helm repo 推送**：`helm push agentid-chain-X.Y.Z.tgz oci://...`
- [ ] **更新官网 changelog**
- [ ] **发公告邮件给客户**（参考 `templates/customer_advisory.md`）
- [ ] **Slack 通知** `#release` 频道
- [ ] **更新 PagerDuty 维护窗口**（如需）

## 🚀 部署（T+1 小时）

- [ ] **staging 部署验证**：
  - [ ] Helm 升级
  - [ ] 健康检查通过
  - [ ] 跑 30 分钟监控
- [ ] **生产灰度**：
  - [ ] 1 个 region / 1 个集群
  - [ ] 监控 1 小时
  - [ ] 扩量（50% → 100%）
- [ ] **SLO 观察**（发布后 24h）：
  - [ ] 错误率 < 0.1%
  - [ ] P99 延迟 < SLO
  - [ ] 无 P1 事故

## 📝 发布后（T+24h）

- [ ] **关闭 release 分支**
- [ ] **更新 sprint board**：标记 release 完成
- [ ] **复盘**：
  - [ ] 顺利的：什么做得好
  - [ ] 改进的：下次如何更快
  - [ ] 文档：更新 release playbook

## ⚠️ 紧急回滚

如发现 P0/P1 问题：

- [ ] **决策**：2 人同意（PM + Tech Lead）
- [ ] **执行**：`helm rollback agentid-chain <previous-revision>`
- [ ] **通知**：`#incident` 频道
- [ ] **后续**：开 postmortem issue

## 📚 参考

- [RELEASE.md](RELEASE.md) — 完整发布流程
- [MIGRATION.md](MIGRATION.md) — 升级迁移
- [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md) — 事故响应
- [ROLLBACK.md](ROLLBACK.md) — 回滚流程
- [postmortem 模板](../templates/postmortem.md) — 复盘模板
