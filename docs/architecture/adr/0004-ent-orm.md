# ADR-0004: ent ORM 作为 L1-Storage 主 ORM

## 状态

✅ Accepted（2024-03-09）

## 上下文

AgentID-Chain 需要一个 L1-Storage 层的 ORM / 数据访问层，要求：

- **类型安全**：编译期校验 SQL/查询（避免运行时 SQL 错误）
- **Schema-as-Code**：用代码定义 schema，自动生成迁移
- **图谱关系**：Agent ↔ Owner ↔ Tag 多对多关系
- **代码生成**：减少样板，提升可维护性
- **PG 兼容性**：支持 PostgreSQL 16+ 特性（JSONB / GIN 索引 / 部分索引）
- **活跃维护**：企业级可用，长期支持

候选方案：

1. **ent** (Facebook 出品) — schema-as-code + 代码生成 + 图关系
2. **GORM** — 全功能 ORM，社区大
3. **sqlc** — SQL → 类型安全 Go 代码，无 ORM
4. **直接 pgx** — 不使用 ORM，手写 SQL

## 决策

我们采用 **ent** 作为 L1-Storage 主 ORM（底层驱动使用 `pgx/v5`）。

理由：

- ✅ **Schema-as-Code**：`internal/storage/schema/` 集中定义，符合项目"声明式规范"
- ✅ **图谱关系**：Agent ↔ Tag 多对多天然支持
- ✅ **自动迁移**：`ent migrate` 命令可生成 DDL
- ✅ **类型安全**：所有查询编译期校验
- ✅ **可扩展**：通过 `entc.Generation` 自定义生成
- ✅ **PG 友好**：底层 `pgx` 驱动，支持 JSONB / 数组 / 部分索引

## 后果

### 正面

- ✅ 减少 SQL 拼写错误，编译期拦截
- ✅ Schema 变更即迁移，减少不一致
- ✅ L2 域与 L1 存储解耦：通过 ent.Client 接口注入
- ✅ 性能：ent 生成的查询与手写 pgx 性能相当

### 负面

- ❌ 学习曲线：需熟悉 ent DSL
- ❌ 黑盒查询：复杂查询需 `entgql` 或手写 SQL
- ❌ 锁定较深：未来若换 ORM 迁移成本高
- ❌ 某些 PG 高级特性（窗口函数 / CTE）需走 `Raw` 通道

### 中性

- 🔄 生成的代码占用仓库空间（已 `.gitignore` 强制 generate）
- 🔄 entc 升级需谨慎（每次升级有 breaking change 风险）

## 替代方案

| 方案 | 优点 | 缺点 | 否决理由 |
|------|------|------|---------|
| **ent**（已选） | 类型安全 + 图关系 + 代码生成 | 学习曲线 | — |
| GORM | 社区大、上手快 | 反射多、性能较弱；Hooks 难调试 | 不符合"显式依赖注入"原则 |
| sqlc | 零运行时开销；SQL 显式 | 不支持 schema 关系；需手写 JOIN | L2 图谱关系表达繁琐 |
| 直接 pgx | 完全控制 | 大量样板；无类型安全 | 与"类型安全"原则冲突 |

## 实施细节

### Schema 定义

```go
// internal/storage/schema/agent.go
package schema

import (
    "entgo.io/ent"
    "entgo.io/ent/schema/field"
    "entgo.io/ent/schema/edge"
)

type Agent struct {
    ent.Schema
}

func (Agent) Fields() []ent.Field {
    return []ent.Field{
        field.String("id").MaxLen(26).Unique(),  // ULID
        field.String("owner").MaxLen(64),
        field.Enum("level").Values("test", "prod"),
        field.Enum("status").Values("active", "revoked"),
        field.Time("created_at"),
    }
}

func (Agent) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("owner_ref", Owner.Type).Ref("agents").Unique(),
        edge.To("tags", Tag.Type),
    }
}
```

### 代码生成

```bash
go generate ./internal/storage/schema/...
```

### 迁移

```bash
# 生成迁移
go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/storage/schema/...

# 应用迁移
atlas migrate apply --dir file://migrations
```

## 参考

- [ent 官方文档](https://entgo.io/docs/getting-started)
- [ent 性能基准](https://entgo.io/docs/performance)
- 项目内 ADR-0001（存储后端混合架构）
- 项目内 `docs/architecture/storage.md`
- 相关 Issue: #L1-1
