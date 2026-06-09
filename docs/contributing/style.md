# 代码规范

> Go 编码风格 + 项目特殊约定

## 🏛️ 基础规范

遵循 [Effective Go](https://go.dev/doc/effective_go) + [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

## 📦 命名

### 包

```go
// ✅ 简短、小写、单词
package authz
package aap
package rbac

// ❌ 避免
package authentication  // 太长
package auth_center      // 下划线
```

### 文件

```
// ✅ 角色清晰
handler.go        # HTTP handler
service.go        # 业务逻辑
repository.go     # 数据访问
model.go          # 数据模型
errors.go         # 错误定义

// 测试
handler_test.go
service_test.go
benchmark_test.go
```

### 函数

```go
// ✅ 动词开头
func RegisterAgent(...)
func ValidateChallenge(...)
func GetAgentByID(...)

// ❌ 名词（容易混淆）
func Agent(...)     // 啥？
func Data(...)      // 啥数据？
```

### 变量

```go
// ✅ 简洁但有意义
agentID := uuid.New()
challenge := make([]byte, 32)

// ❌ 缩写过度
aid := uuid.New()  // 不清晰
ch := make(...)     // 单词冲突
```

## 🎨 格式

### 使用 gofmt + goimports

```bash
gofmt -w .
goimports -w .
```

### 缩进

- Tab（Go 标准）
- 一行不超过 100 字符

### 函数顺序

```go
// 1. 构造器
func NewService(deps Deps) *Service { ... }

// 2. 公开方法
func (s *Service) PublicMethod() { ... }

// 3. 私有方法
func (s *Service) privateMethod() { ... }

// 4. 接口实现
var _ Interface = (*Service)(nil)
```

## 🛡️ 错误处理

### 使用 error 类型

```go
// ✅
if err != nil {
    return fmt.Errorf("register agent: %w", err)
}

// ❌ panic (除真正不可恢复)
panic("not implemented")  // 仅在 main 启动时检查
```

### 自定义错误

```go
// internal/domain/errors.go
var (
    ErrAgentNotFound = errors.New("agent not found")
    ErrAgentRevoked  = errors.New("agent revoked")
    ErrInvalidLevel  = errors.New("invalid level")
)
```

### Wrap vs New

```go
// ✅ 在 L4/L5 用 %w（保留链路）
return fmt.Errorf("register: %w", err)

// ✅ 在 L2 用 %w
return fmt.Errorf("validate: %w", ErrInvalidLevel)
```

## 🏗️ 架构铁律

### 依赖方向

```
L5 → L3 → L4 → L2 → L1
```

跨层跳跃 ❌

### L2 Domain 零依赖

```go
// ✅ 仅标准库
import (
    "errors"
    "context"
    "time"
    "crypto/sha256"
)

// ❌ 任何 github.com/ 都违反
import (
    "github.com/.../ent"     // ❌
    "gopkg.in/..."           // ❌
)
```

### 接口位置

```go
// ✅ 接口在使用方定义（依赖倒置）
// internal/service/service.go
type AgentRepository interface {
    Save(ctx context.Context, a *Agent) error
    Get(ctx context.Context, id uuid.UUID) (*Agent, error)
}

// L1 实现接口
type pgAgentRepository struct{ ... }
func (r *pgAgentRepository) Save(...) error { ... }
```

## 🧪 测试

### 命名

```go
func TestService_RegisterAgent_Success(t *testing.T) { ... }
func TestService_RegisterAgent_DuplicateUUID(t *testing.T) { ... }
```

### 表格驱动

```go
tests := []struct {
    name    string
    input   Input
    want    Output
    wantErr bool
}{
    {"valid", Input{...}, Output{...}, false},
    {"invalid", Input{...}, Output{}, true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := Service.Process(tt.input)
        // assert
    })
}
```

### 不依赖外部

```go
// ❌ 直接连 PG
db, _ := sql.Open("postgres", "...")

// ✅ 使用 testcontainers 或 mock
repo := mocks.NewMockAgentRepository(t)
```

## 📊 性能

### 避免不必要的分配

```go
// ❌
s := fmt.Sprintf("agent-%s", id)

// ✅（热路径）
var buf [64]byte
n := copy(buf[:], "agent-")
hex.Encode(buf[n:], id[:])
```

### Benchmark 验证

每次性能相关 PR 需附 benchmark 结果。

## 📚 文档

### 公开 API 必须有 godoc

```go
// RegisterAgent registers a new agent with the given owner and level.
// It returns the assigned AgentID or an error if registration fails.
//
// The challenge parameter is the AAP challenge response (base64).
func RegisterAgent(ctx context.Context, owner string, level Level, challenge []byte) (uuid.UUID, error) {
    // ...
}
```

### 复杂逻辑必须有注释

```go
// Why: We use a separate nonce store per public key to prevent
// cross-key replay attacks.
nonceKey := "aap:nonce:" + publicKey
```

## 🛠️ 工具链

| 工具 | 用途 |
|------|------|
| `gofmt` | 格式化 |
| `goimports` | import 排序 |
| `go vet` | 静态检查 |
| `golangci-lint` | 综合 lint |
| `staticcheck` | 高级检查 |
| `gosec` | 安全检查 |
| `mockery` | mock 生成 |
| `pre-commit` | 提交前检查 |

## 📋 Lint 配置

`.golangci.yml`:
```yaml
linters:
  enable:
    - gofmt
    - goimports
    - govet
    - staticcheck
    - gocritic
    - gosec
    - errcheck
    - ineffassign
    - misspell
    - unconvert
    - prealloc
```

## 🏆 Review 检查清单

Reviewer 检查项：

- [ ] 代码符合分层（[5-layer.md](../architecture/5-layer.md)）
- [ ] 错误处理规范（不 panic、不忽略）
- [ ] 命名清晰
- [ ] 测试覆盖（> 70%）
- [ ] 公开 API 有 godoc
- [ ] 无 hardcoded 密钥/密码
- [ ] 性能关键路径有 benchmark
- [ ] CHANGELOG 更新（如适用）
- [ ] ADR 更新（如架构决策变更）
