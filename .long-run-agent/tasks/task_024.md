# task_024

## 描述

[P22] 发布与运维

## 需求 (requirements)

P21 完成

## 验收标准 (acceptance)

- 语义化版本（SemVer 2.0）发布流程
- CHANGELOG.md 自动生成（git-cliff 或类似）
- 发布脚本（scripts/release.sh）：版本号 bump / tag / 构建 / 签名 / 上传
- 镜像仓库配置（GHCR、Aliyun ACR）
- Helm Chart 完整（values.yaml + templates + Chart.yaml）
- ArgoCD / Flux 兼容的 GitOps 部署
- 发布前自检（test / lint / security / coverage）
- SBOM 集成（syft）
- Cosign 签名（key + keyless）
- 通知机制（Slack / Email）

## 交付物 (deliverables)

- CHANGELOG.md
- scripts/release.sh
- scripts/pre-release-check.sh
- .github/workflows/release.yml
- deploy/helm/agentid-chain/ (完整 chart)
- deploy/gitops/ (ArgoCD 应用)
- .github/ISSUE_TEMPLATE/release.md
- docs/operations/release-process.md
