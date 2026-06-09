# PR 流程

> Pull Request 提交、Review、合并的完整流程

## 📋 流程图

```
创建分支
  ↓
开发 + 测试
  ↓
pre-commit / lint / test
  ↓
git commit (Conventional Commits)
  ↓
git push
  ↓
创建 PR (使用模板)
  ↓
CI 通过 (build / test / lint / security)
  ↓
Code Review (至少 1 个 approver)
  ↓
处理 Review 意见
  ↓
Squash and Merge
  ↓
删除分支
```

## 🛠️ 创建 PR

### 1. 推送分支

```bash
git push origin feat/my-feature
```

### 2. 在 GitHub 创建 PR

URL: `https://github.com/agentid-chain/agentid-chain/compare/main...feat/my-feature`

### 3. 填写 PR 模板

```markdown
## 概要
<!-- 1-3 句话描述这次 PR 解决了什么 -->

## 关联 Issue
<!-- Closes #123, Fixes #456 -->

## 改动类型
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Refactor
- [ ] Documentation
- [ ] Performance
- [ ] Test
- [ ] CI / Build

## 测试
- [ ] 单元测试已添加/更新
- [ ] 集成测试已添加/更新
- [ ] 手动测试已通过
- [ ] Benchmark 已运行（perf 类）

## 检查清单
- [ ] 代码符合 [style.md](style.md)
- [ ] 公开 API 有 godoc
- [ ] 错误处理规范
- [ ] 无 hardcoded 密钥
- [ ] CHANGELOG 已更新（如适用）
- [ ] ADR 已更新（如架构变更）

## 截图 / 日志
<!-- 如适用 -->

## 影响范围
<!-- 哪些模块 / 服务 / 配置受影响？ -->
```

## ✅ Review 标准

### Reviewer 责任

- 24h 内首次响应
- 关注：架构、性能、安全、测试、可维护性
- 评论要具体（指明行号、给出建议）

### Review 评论前缀

| 前缀 | 含义 | 作者行动 |
|------|------|---------|
| `nit:` | 小问题（格式 / 命名） | 可选 |
| `suggestion:` | 建议（非阻塞） | 可选 |
| `question:` | 提问 | 必须回答 |
| `issue:` | 问题（阻塞） | 必须修复 |
| `blocking:` | 阻塞 | 必须修复 |

## 🔧 处理 Review

```bash
# 1. 拉取最新 main
git checkout main
git pull

# 2. 切回 PR 分支，rebase
git checkout feat/my-feature
git rebase main

# 3. 推送（force-with-lease）
git push --force-with-lease

# 4. 等待 CI 重跑
```

## 🔀 合并

### 策略

项目使用 **Squash and Merge**：
- 多个 commit 合并为 1 个
- 保留 PR 标题作为 commit subject
- 保留 PR 描述作为 commit body

### 流程

1. 所有 CI 通过 ✅
2. 至少 1 个 Approve ✅
3. 无未解决 conversation ✅
4. 分支无冲突 ✅
5. Maintainer 点击 Squash and Merge

### 合并后

- 删除源分支
- 关闭关联 Issue（如未自动关闭）

## 🚨 特殊情况

### Breaking Change

需在 PR 描述中明确标注 `BREAKING CHANGE:`：

```
feat(storage)!: rename Backend interface to StorageBackend

BREAKING CHANGE: 
- `storage.Backend` → `storage.StorageBackend`
- Migration: 替换所有引用并重新编译
```

版本号需 bump major（v2.0.0 → v3.0.0）。

### Hotfix

紧急修复（生产故障）：

1. 从 `main` 创建 `hotfix/fix-xxx` 分支
2. 修复 + 测试
3. 快速 review（1 个 approver 即可）
4. 合并到 main
5. 立即发布新版本
6. 创建 Post-Mortem

## 📊 状态检查

### CI 必须通过

- [ ] `build.yml` — 编译
- [ ] `test.yml` — 单元 / 集成测试
- [ ] `lint.yml` — golangci-lint
- [ ] `coverage-check.yml` — 覆盖率 ≥ 70%
- [ ] `security.yml` — gosec / govulncheck
- [ ] `docker-build.yml` — Docker 构建

### 状态徽章

PR 顶部会显示：
- ✅ All checks passed
- ❌ X checks failed

## 🏆 优秀 PR 示例

参考历史优秀 PR（待添加）：

## 📞 遇到问题？

- Slack: `#agentid-dev`
- Email: dev@agentid-chain.example.com

## 📚 相关

- [开发流程](development.md)
- [代码规范](style.md)
- [Conventional Commits](https://www.conventionalcommits.org/)
