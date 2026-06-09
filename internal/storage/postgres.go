// Package storage PostgreSQL 客户端封装。
//
// 封装目标：
//  1. 把 pgx 驱动 + ent dialect 装配成 *ent.Client
//  2. 连接池参数集中管理（MaxOpen / MaxIdle / MaxLifetime）
//  3. 健康检查 — 启动时验证连通性
//  4. 优雅关闭 — 关 client 前先 drain 所有 in-flight tx
//
// 选型说明：
//   - 驱动：pgx/v5（性能优于 lib/pq；ent 官方推荐）
//   - stdlib 适配：pgx/v5/stdlib — 把 pgx 注册为 database/sql driver
//   - ORM：ent v0.14.6（schema-first，code-gen）
//   - 池：database/sql 通用 SetMax{Open,Idle}Conns / SetConnMaxLifetime
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	// pgx stdlib 注册驱动到 database/sql
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/agentid-chain/agentid-chain/ent"
)

// PostgresConfig PostgreSQL 连接配置（与 internal/config.DBConfig 字段对齐）。
type PostgresConfig struct {
	DSN         string        // postgres://user:pass@host:port/db?sslmode=disable
	MaxOpen     int           // 最大打开连接数（默认 25）
	MaxIdle     int           // 最大空闲连接数（默认 5）
	MaxLifetime time.Duration // 连接最长存活时间（默认 30m）
}

// PostgresHandle 把 *ent.Client 与底层 *sql.DB 一起包成结构体，
// 方便上层做 healthcheck / 关闭。
type PostgresHandle struct {
	Client *ent.Client
	DB     *sql.DB
}

// OpenPostgres 打开 PostgreSQL 连接并返回 *PostgresHandle。
//
// 调用方负责在退出时调用 handle.Close()。
//
// 失败模式：
//   - DSN 为空 → 立即返回 ErrUnavailable
//   - sql.Open 失败 → 包装后返回
//   - Ping 失败 → 关闭后返回 ErrUnavailable（让上层 fail-fast）
//
//nolint:gocritic // 工厂方法保持单参数 cfg，调用方可读性更高
func OpenPostgres(ctx context.Context, cfg PostgresConfig) (*PostgresHandle, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("%w: postgres DSN is empty", ErrUnavailable)
	}
	if cfg.MaxOpen == 0 {
		cfg.MaxOpen = 25
	}
	if cfg.MaxIdle == 0 {
		cfg.MaxIdle = 5
	}
	if cfg.MaxLifetime == 0 {
		cfg.MaxLifetime = 30 * time.Minute
	}

	// 1. database/sql 打开连接池
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("storage: postgres sql.Open: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpen)
	db.SetMaxIdleConns(cfg.MaxIdle)
	db.SetConnMaxLifetime(cfg.MaxLifetime)

	// 2. 健康检查（带超时）
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: postgres ping: %w", ErrUnavailable, err)
	}

	// 3. ent dialect 包装
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))

	return &PostgresHandle{Client: client, DB: db}, nil
}

// Close 关闭 ent.Client 与底层 *sql.DB。
//
// 推荐用法：defer handle.Close()
func (h *PostgresHandle) Close() error {
	if h == nil {
		return nil
	}
	var firstErr error
	if h.Client != nil {
		if err := h.Client.Close(); err != nil {
			firstErr = err
		}
	}
	if h.DB != nil {
		if err := h.DB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Ping 轻量健康检查（不写表；走 driver.Ping）。
//
// 用于 /readyz 端点。
func (h *PostgresHandle) Ping(ctx context.Context) error {
	if h == nil || h.DB == nil {
		return errors.New("storage: postgres handle is nil")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := h.DB.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: postgres ping: %w", ErrUnavailable, err)
	}
	return nil
}
