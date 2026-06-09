# SemVer 规范

> AgentID-Chain 遵循 [Semantic Versioning 2.0.0](https://semver.org/)

## 📐 版本号格式

```
MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
```

| 段 | 触发条件 | 示例 |
|----|---------|------|
| **MAJOR** | 不兼容 API 变更 | 1.x.x → 2.0.0 |
| **MINOR** | 向后兼容新功能 | 2.0.x → 2.1.0 |
| **PATCH** | 向后兼容 bug 修复 | 2.1.0 → 2.1.1 |
| **PRERELEASE** | 预发布（alpha/beta/rc） | 2.1.0-rc.1 |
| **BUILD** | 编译元数据 | 2.1.0+20260610 |

## 🔢 当前版本

```
2.0.1
```

下一个 MINOR 版本候选：2.1.0（计划中）

## 📋 升级类别决策表

| 变更 | 版本类型 | 示例 |
|------|---------|------|
| 移除公开 API | **MAJOR** | 移除 `LegacyRegisterAgent` |
| 改 API 签名 | **MAJOR** | `Register(req, ctx)` → `Register(ctx, req)` |
| 改默认值 | **MAJOR** | `pool_size` 25 → 100 |
| 移除配置项 | **MAJOR** | 移除 `legacy_signature_alg` |
| 新增 API 端点 | MINOR | 新增 `/v1/agents/batch` |
| 新增配置项 | MINOR | 新增 `chain.fallback_url` |
| 新增可选字段 | MINOR | `metadata` 增加新键 |
| Bug 修复 | PATCH | 修复 N+1 查询 |
| 文档 / 重构 | PATCH | 内部接口重命名 |
| 安全修复 | PATCH | CVE 修复 |

## 🏷️ 预发布标签

| 标签 | 含义 | 稳定性 |
|------|------|--------|
| `alpha.N` | 内部测试 | ❌ 不稳定 |
| `beta.N` | 公开测试 | ⚠️ 可能变更 |
| `rc.N` | 发布候选 | ✅ 锁定 API |

预发布优先级：`2.1.0-rc.1` < `2.1.0` < `2.2.0-alpha.1`

## 🔧 在代码中引用

### Go

```go
import "github.com/agentid-chain/agentid-chain/internal/version"

fmt.Println(version.String())  // "2.0.1"
fmt.Println(version.Commit)     // "abc123..."
```

### CLI

```bash
$ agentid version
agentid v2.0.1 (commit: abc123, built: 2026-06-10)
```

### Helm

```yaml
image:
  tag: 2.0.1  # 锁定具体版本
```

## ⚠️ 兼容性承诺

- **MAJOR 版本内**：公开 API 向后兼容
- **MINOR 版本内**：行为可观察但不变
- **PATCH 版本内**：行为完全不变

破坏性变更**必须**走 MAJOR 升级 + 至少 1 个 MINOR 弃用期。

## 📚 延伸阅读

- [SemVer 2.0.0 规范](https://semver.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [本项目 RELEASE.md](RELEASE.md)
- [MIGRATION.md](MIGRATION.md)
