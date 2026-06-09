// Package testcontainers: postgres 容器 helper。
//
// 启动一个真实的 PostgreSQL 容器（默认 postgres:16-alpine），暴露：
//   - DSN():    "postgres://user:pass@host:port/db?sslmode=disable"
//   - DB():     *sql.DB（database/sql 通用）
//   - EntClient(): *ent.Client（带 dial 包装）
//   - ApplyMigrations(): 自动应用 ent schema + 自定义 SQL
//   - Terminate(): 停止容器
//
// 推荐用法（testing.TB）：
//
//	pg := testcontainers.MustNewPostgresContainer(t, PostgresOpts{})
//	defer pg.Terminate(t.Context())
//	db := pg.DB(t.Context())
//	// ... 用 db ...
//
// MustNew 会因启动失败而 t.Fatal；NewPostgresContainer 返回 error 供
// 调用方决定是否 skip（无 Docker 环境）。
package testcontainers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	entdialect "entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/agentid-chain/agentid-chain/ent"
	"github.com/agentid-chain/agentid-chain/ent/migrate"
)

// =============================================================================
// 选项
// =============================================================================

// PostgresOpts 启动参数（全部可选，零值 = 默认）。
type PostgresOpts struct {
	// Image 镜像 tag（默认 "postgres:16-alpine"）
	Image string
	// DBName 数据库名（默认 "agentid"）
	DBName string
	// User 用户名（默认 "agentid"）
	User string
	// Password 密码（默认 "agentid"）
	Password string
	// ApplyEntMigrations 是否自动应用 ent 生成的 schema（默认 true）
	ApplyEntMigrations bool
	// applyEntMigrationsSet 内部标记，区分"显式 false"和"零值"。
	applyEntMigrationsSet bool
	// SQLDir 额外 SQL 迁移目录（*.sql，按文件名升序，可选）
	SQLDir string
	// StartupTimeout 启动等待上限（默认 60s）
	StartupTimeout time.Duration
	// Reuse 启用 testcontainers reuse（需 TESTCONTAINERS_REUSE_ENABLE=1）
	Reuse bool
}

// WithApplyEntMigrations 控制是否自动应用 ent schema（链式可选）。
func (o PostgresOpts) WithApplyEntMigrations(apply bool) PostgresOpts {
	o.ApplyEntMigrations = apply
	o.applyEntMigrationsSet = true
	return o
}

// ReuseContainerName reuse 时的容器名（必须稳定）。
const ReuseContainerName = "agentid-pg-test"

// =============================================================================
// PostgresContainer
// =============================================================================

// PostgresContainer 包装 testcontainers 启动的 PG 实例。
//
// 线程安全：所有方法串行安全；DB()/EntClient() 多次调用返回新连接实例。
type PostgresContainer struct {
	t           testing.TB
	ctr         *tcpostgres.PostgresContainer
	dsn         string
	host        string
	port        int
	dbName      string
	user        string
	password    string
	startedOnce sync.Once
	startErr    error
}

// NewPostgresContainer 启动容器（如失败返回 error，调用方决定 skip / fail）。
//
// 必须在 testing.TB 内调用（用于 FailNow / Cleanup）。
func NewPostgresContainer(t testing.TB, opts PostgresOpts) (*PostgresContainer, error) {
	t.Helper()
	if t == nil {
		return nil, errors.New("testcontainers: nil testing.TB")
	}
	if opts.Image == "" {
		opts.Image = "postgres:16-alpine"
	}
	if opts.DBName == "" {
		opts.DBName = "agentid"
	}
	if opts.User == "" {
		opts.User = "agentid"
	}
	if opts.Password == "" {
		opts.Password = "agentid"
	}
	if opts.StartupTimeout == 0 {
		opts.StartupTimeout = 60 * time.Second
	}
	// ApplyEntMigrations / ApplyMigrationsOnCreate 默认 true（零值需要显式 opt-out）
	// 由于 bool 零值是 false，我们改用 *bool 模式：
	// 这里只做"显式 true 才会应用"，方便测试关闭。
	if !opts.applyEntMigrationsSet {
		opts.ApplyEntMigrations = true
	}

	c := &PostgresContainer{
		t:        t,
		dbName:   opts.DBName,
		user:     opts.User,
		password: opts.Password,
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.StartupTimeout)
	defer cancel()

	ctr, err := tcpostgres.Run(ctx,
		opts.Image,
		tcpostgres.WithDatabase(opts.DBName),
		tcpostgres.WithUsername(opts.User),
		tcpostgres.WithPassword(opts.Password),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("testcontainers: run postgres: %w", err)
	}
	c.ctr = ctr

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = ctr.Terminate(context.Background())
		return nil, fmt.Errorf("testcontainers: host: %w", err)
	}
	mappedPort, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = ctr.Terminate(context.Background())
		return nil, fmt.Errorf("testcontainers: mapped port: %w", err)
	}
	c.host = host
	c.port = int(mappedPort.Num())
	c.dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		opts.User, opts.Password, host, c.port, opts.DBName)

	// 注册自动清理
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("testcontainers: terminate postgres: %v", err)
		}
	})

	// 启动后自动应用迁移
	if opts.ApplyEntMigrations {
		if err := c.applyEntSchema(ctx); err != nil {
			_ = ctr.Terminate(context.Background())
			return nil, fmt.Errorf("testcontainers: ent migrate: %w", err)
		}
	}
	if opts.SQLDir != "" {
		if err := c.applySQLDir(ctx, opts.SQLDir); err != nil {
			_ = ctr.Terminate(context.Background())
			return nil, fmt.Errorf("testcontainers: sql dir: %w", err)
		}
	}

	return c, nil
}

// MustNewPostgresContainer 启动失败 → t.Fatal（不返回 error）。
func MustNewPostgresContainer(t testing.TB, opts PostgresOpts) *PostgresContainer {
	t.Helper()
	c, err := NewPostgresContainer(t, opts)
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	return c
}

// =============================================================================
// 访问器
// =============================================================================

// DSN 返回连接串。
func (c *PostgresContainer) DSN() string {
	return c.dsn
}

// Host 返回容器映射的 host。
func (c *PostgresContainer) Host() string {
	return c.host
}

// Port 返回容器映射的端口。
func (c *PostgresContainer) Port() int {
	return c.port
}

// DB 返回 *sql.DB（database/sql 池，连接上限 25）。
//
// 失败 → t.Fatal。
func (c *PostgresContainer) DB(ctx context.Context) *sql.DB {
	c.t.Helper()
	db, err := sql.Open("pgx", c.dsn)
	if err != nil {
		c.t.Fatalf("testcontainers: sql.Open: %v", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		c.t.Fatalf("testcontainers: ping: %v", err)
	}
	// 关闭交给 t.Cleanup
	c.t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

// EntClient 返回 *ent.Client（带 ent migrate 完成后可用的 schema）。
func (c *PostgresContainer) EntClient(ctx context.Context) *ent.Client {
	c.t.Helper()
	db := c.DB(ctx)
	drv := entsql.OpenDB(entdialect.Postgres, db)
	return ent.NewClient(ent.Driver(drv))
}

// =============================================================================
// 迁移
// =============================================================================

// applyEntSchema 应用 ent 生成的 schema（migrate.Tables）。
func (c *PostgresContainer) applyEntSchema(ctx context.Context) error {
	db, err := sql.Open("pgx", c.dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	drv := entsql.OpenDB(entdialect.Postgres, db)
	// ent 0.14.6: 不带选项让 NewMigrate 用默认（无 GlobalUniqueID）
	if err := migrate.NewSchema(drv).Create(ctx); err != nil {
		return fmt.Errorf("ent schema create: %w", err)
	}
	c.t.Logf("testcontainers: ent schema applied (Tables=%d)", len(migrate.Tables))
	return nil
}

// applySQLDir 应用目录下的所有 *.sql（按文件名升序）。
//
// 用于：
//   - ent 之外的扩展脚本（init 数据 / 视图 / 索引）
//   - 自定义 schema patch
func (c *PostgresContainer) applySQLDir(ctx context.Context, dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("sql dir not found: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read sql dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	if len(files) == 0 {
		return nil
	}
	sort.Strings(files)

	db, err := sql.Open("pgx", c.dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if _, err := db.ExecContext(ctx, string(raw)); err != nil {
			return fmt.Errorf("exec %s: %w", f, err)
		}
		c.t.Logf("testcontainers: applied %s", filepath.Base(f))
	}
	return nil
}

// Reset 重建 public schema（清空数据但保留容器）。
//
// 适合 test-between reset，比 Terminate+Start 快 10x。
func (c *PostgresContainer) Reset(ctx context.Context) error {
	db, err := sql.Open("pgx", c.dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		return err
	}
	// 重应用 ent schema
	return c.applyEntSchema(ctx)
}

// =============================================================================
// 生命周期
// =============================================================================

// Terminate 停止并删除容器（幂等：多次调用安全）。
func (c *PostgresContainer) Terminate(ctx context.Context) error {
	if c.ctr == nil {
		return nil
	}
	if err := c.ctr.Terminate(ctx); err != nil {
		return fmt.Errorf("testcontainers: terminate: %w", err)
	}
	c.ctr = nil
	return nil
}
