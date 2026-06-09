# Branch 保护策略 (P16.13)

本文档描述 `main` / `develop` 分支的保护规则。

## 🛡️ main 分支（生产）

| 规则 | 状态 | 说明 |
|------|------|------|
| 禁止直接 push | ✅ 必须 | 只能通过 PR merge |
| 必须 2 人 approve | ✅ 必须 | 1 个 core-team + 1 个 code-owner |
| 必须所有 status check 通过 | ✅ 必须 | 见下表 |
| 必须无 unresolved conversation | ✅ 必须 | 所有评论必须 resolve |
| 必须非 draft | ✅ 必须 | 草稿 PR 不允许 merge |
| 禁止 force push | ✅ 必须 | |
| 禁止删除分支 | ✅ 必须 | |
| 线性历史（squash 或 rebase） | ✅ 必须 | |
| 包含 sign-off | ⚪ 可选 | DCO |

### 必需的 status check

| Check | 来源 | 阻断 merge |
|-------|------|-----------|
| `build / build (ubuntu-latest)` | build.yml | ✅ |
| `build / build (macos-latest)` | build.yml | ✅ |
| `build / build (windows-latest)` | build.yml | ✅ |
| `build / lint` | build.yml | ✅ |
| `build / gofmt` | build.yml | ✅ |
| `lint / golangci-lint` | lint.yml | ✅ |
| `lint / govulncheck` | lint.yml | ✅ |
| `lint / actionlint` | lint.yml | ✅ |
| `test / unit + race` | test.yml | ✅ |
| `test / unit + coverage` | test.yml | ✅ |
| `test / integration (PG + Redis)` | test.yml | ✅ |
| `coverage-check / coverage check ≥ 70%` | coverage-check.yml | ✅ |
| `docker-build / build (gateway)` | docker-build.yml | ✅ |
| `docker-build / build (cli)` | docker-build.yml | ✅ |
| `docker-build / build (migration)` | docker-build.yml | ✅ |
| `docker-build / build (mock-chain)` | docker-build.yml | ✅ |
| `security / govulncheck` | security.yml | ⚪ 警告（不阻断） |
| `security / Trivy (filesystem)` | security.yml | ⚪ 警告 |
| `security / CodeQL` | security.yml | ⚪ 警告 |

## 🛡️ develop 分支（集成分支）

| 规则 | 状态 | 说明 |
|------|------|------|
| 禁止直接 push | ✅ 必须 | 只能通过 PR merge |
| 必须 1 人 approve | ✅ 必须 | core-team 成员 |
| 必须所有 status check 通过 | ⚪ 部分 | 阻断：build / lint / test / coverage / docker-build |

## 🚀 feature/* 分支

| 规则 | 状态 | 说明 |
|------|------|------|
| 命名规范 | ✅ 强制 | `feature/<short-desc>` 或 `fix/<short-desc>` |
| 必须基于最新 main | ✅ 推荐 | merge 前 rebase |
| 必须通过 build + lint + test | ✅ 必须 | status check |
| 24h 内无更新自动关闭 | ⚪ 启用 | stale bot |

## 🤖 自动合并条件

满足以下全部条件时启用 auto-merge：

- [x] 全部 status check 通过
- [x] 至少 1 个 core-team 成员 approve
- [x] PR 标题符合 Conventional Commits
- [x] 关联了 issue（Closes #xxx）
- [x] 改动行数 < 500

## 🔧 配置方法

### GitHub UI

1. 进入 `Settings` → `Branches` → `Add rule`
2. Branch name pattern: `main`
3. 勾选上述规则
4. 添加 required status checks
5. 保存

### GitHub API

```bash
curl -X PUT \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Accept: application/vnd.github.london-preview+json" \
  https://api.github.com/repos/agentid-chain/agentid-chain/branches/main/protection \
  -d '{
    "required_status_checks": {
      "strict": true,
      "contexts": [
        "build / build (ubuntu-latest)",
        "test / unit + race",
        "coverage-check / coverage check ≥ 70%"
      ]
    },
    "enforce_admins": true,
    "required_pull_request_reviews": {
      "dismiss_stale_reviews": true,
      "require_code_owner_reviews": true,
      "required_approving_review_count": 2,
      "require_last_push_approval": true
    },
    "restrictions": null,
    "required_linear_history": true,
    "allow_force_pushes": false,
    "allow_deletions": false,
    "block_creations": false,
    "required_conversation_resolution": true,
    "lock_branch": false,
    "allow_fork_syncing": false
  }'
```

### Terraform（推荐）

使用 `github_branch_protection` resource：

```hcl
resource "github_branch_protection" "main" {
  repository_id = github_repository.agentid_chain.id
  pattern       = "main"

  required_status_checks {
    strict = true
    contexts = [
      "build / build (ubuntu-latest)",
      "test / unit + race",
      "coverage-check / coverage check ≥ 70%"
    ]
  }

  required_pull_request_reviews {
    dismiss_stale_reviews           = true
    require_code_owner_reviews      = true
    required_approving_review_count = 2
    require_last_push_approval      = true
  }

  enforce_admins          = true
  required_linear_history = true
  allows_deletions        = false
  allows_force_pushes     = false
  blocks_creations        = false
  requires_conversation_resolution = true
  restrict_pushes {
    blocks_creations = false
  }
}
```

## 📈 监控

每周 LRA Constitution 验证会检查：

- [ ] main 分支有保护规则
- [ ] 必需的 status check 全部存在
- [ ] CODEOWNERS 至少有 3 个 team
- [ ] 最近 7 天没有 force push
- [ ] 最近 7 天没有绕过 review 的 merge
