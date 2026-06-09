# govulncheck 集成 (P17.3)

## 安装

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## 本地运行

```bash
# 检查所有 package
govulncheck ./...

# 仅检查当前 binary
govulncheck -mode=binary ./bin/agentid

# 仅 source 模式（不依赖 build）
govulncheck -mode=source ./...
```

## 输出格式

```text
Vulnerability  #1  GO-2023-XXXX-XXXX
    Affects: github.com/foo/bar@v1.2.3
    Symbol:  foo.Bar.Baz
    Description: ...
    Reference: https://pkg.go.dev/vuln/GO-2023-XXXX-XXXX
```

## CI 集成

已通过 `lint.yml` 集成 `govulncheck`，每周一与每次 push/PR 都会跑。

## 已知漏洞处理流程

1. **确认**：查看 `Reference` 链接（go.dev/vuln/<id>）
2. **升级**：`go get -u <module>@<fixed-version>`
3. **回归**：`make test` + `make test-integration`
4. **提交**：`fix(deps): upgrade <module> to <version> for CVE-XXXX-XXXX`
5. **追踪**：在 issue 中 `Closes #XXX` 关联 CVE

## 当前基线

| 模块 | 版本 | 状态 |
|------|------|------|
| `github.com/golang-jwt/jwt` | v5.2.x | ✅ 无已知漏洞 |
| `golang.org/x/crypto` | v0.x.x | ✅ 跟踪 |
| `github.com/redis/go-redis` | v9.x.x | ✅ |
| `github.com/lestrrat-go/jwx` | v2.x.x | ✅ |

## 豁免

极少数情况下漏洞无法立即修复时，使用 `//go:build ignore_vuln_check` 标记并记录在 issue 中。
