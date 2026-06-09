// Package testcontainers 提供集成测试基础设施（基于 testcontainers-go）。
//
// 设计动机：
//   - 集成测试需要真实的 PG/Redis，而不是 miniredis / SQLite
//   - 复用 dev 镜像（postgres:16-alpine / redis:7-alpine），保证与生产一致
//   - 一次性启动 + t.Cleanup 自动回收，CI 与本地同源
//
// 子包：
//   - postgres.go — PostgresContainer（启动 / DSN / 迁移 / 清理）
//   - redis.go    — RedisContainer（启动 / Addr / 清理）
//
// 用法示例：
//
//	func TestRepo_Insert(t *testing.T) {
//	    pg := testcontainers.NewPostgresContainer(t)
//	    db := pg.DB(t) // 已自动应用 ent 迁移
//	    repo := repo.New(db)
//	    // ... 测试 ...
//	}
//
// 依赖：testcontainers-go v0.42.0 + Docker 守护进程。
// 在没有 Docker 的环境（CI lite 节点），可用 miniredis / sqlite 替代，但本
// helper 不做 fallback —— 集成测试必须真实。
package testcontainers
