// Package config — 配置校验。
//
// 在 Load 完成后调用 Validate()，确保 Config 满足启动要求。
// 校验失败时返回多重错误（用 errors.Join 合并），便于一次性看到所有问题。
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ValidationError 单条校验错误。
type ValidationError struct {
	Field   string // 配置项路径（如 "storage.db.dsn"）
	Message string // 错误描述
	Cause   error  // 原始错误（可选）
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Field, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// Validate 校验整个 Config，失败时返回多重错误。
func (cfg *Config) Validate() error {
	var errs []error

	// Service
	if err := validateService(&cfg.Service); err != nil {
		errs = append(errs, err)
	}

	// Log
	if err := validateLog(&cfg.Log); err != nil {
		errs = append(errs, err)
	}

	// Backend
	if err := validateBackend(&cfg.Backend); err != nil {
		errs = append(errs, err)
	}

	// Storage
	if err := validateStorage(&cfg.Storage); err != nil {
		errs = append(errs, err)
	}

	// Authz
	if err := validateAuthz(&cfg.Authz); err != nil {
		errs = append(errs, err)
	}

	// Chain (only if backend is onchain/hybrid)
	if cfg.Backend.Type != "local" {
		if err := validateChain(&cfg.Chain); err != nil {
			errs = append(errs, err)
		}
	}

	// Telemetry
	if err := validateTelemetry(&cfg.Telemetry); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func validateService(s *ServiceConfig) error {
	var errs []error
	if strings.TrimSpace(s.Name) == "" {
		errs = append(errs, &ValidationError{Field: "service.name", Message: "required"})
	}
	if !validRole(s.Role) {
		errs = append(errs, &ValidationError{
			Field:   "service.role",
			Message: fmt.Sprintf("invalid role %q (want: gateway|auth-center|tag-sense|mcp|migration|cli)", s.Role),
		})
	}
	if !validAddr(s.HTTPAddr) {
		errs = append(errs, &ValidationError{Field: "service.http_addr", Message: fmt.Sprintf("invalid addr %q", s.HTTPAddr)})
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func validateLog(l *LogConfig) error {
	var errs []error
	if !validLogLevel(l.Level) {
		errs = append(errs, &ValidationError{
			Field:   "log.level",
			Message: fmt.Sprintf("invalid level %q (want: debug|info|warn|error)", l.Level),
		})
	}
	if !validLogFormat(l.Format) {
		errs = append(errs, &ValidationError{
			Field:   "log.format",
			Message: fmt.Sprintf("invalid format %q (want: json|text)", l.Format),
		})
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func validateBackend(b *BackendConfig) error {
	switch b.Type {
	case "local", "onchain", "hybrid":
		return nil
	default:
		return &ValidationError{
			Field:   "backend.type",
			Message: fmt.Sprintf("invalid type %q (want: local|onchain|hybrid)", b.Type),
		}
	}
}

func validateStorage(s *StorageConfig) error {
	var errs []error
	// DB
	if s.DB.Driver != "postgres" {
		errs = append(errs, &ValidationError{
			Field:   "storage.db.driver",
			Message: fmt.Sprintf("invalid driver %q (only postgres supported in v2.0.1)", s.DB.Driver),
		})
	}
	if strings.TrimSpace(s.DB.DSN) == "" {
		errs = append(errs, &ValidationError{Field: "storage.db.dsn", Message: "required"})
	} else if !validPostgresDSN(s.DB.DSN) {
		errs = append(errs, &ValidationError{Field: "storage.db.dsn", Message: "invalid DSN (must be postgres://...)"})
	}
	if s.DB.MaxOpen <= 0 {
		errs = append(errs, &ValidationError{Field: "storage.db.max_open", Message: "must be > 0"})
	}
	if s.DB.MaxIdle < 0 {
		errs = append(errs, &ValidationError{Field: "storage.db.max_idle", Message: "must be >= 0"})
	}
	if s.DB.MaxLifetime < 0 {
		errs = append(errs, &ValidationError{Field: "storage.db.max_lifetime", Message: "must be >= 0"})
	}
	// Redis
	if s.Redis.Addr == "" {
		errs = append(errs, &ValidationError{Field: "storage.redis.addr", Message: "required"})
	}
	if s.Redis.Timeout < 0 {
		errs = append(errs, &ValidationError{Field: "storage.redis.timeout", Message: "must be >= 0"})
	}
	// Outbox
	if s.Outbox.Enabled {
		if s.Outbox.StreamKey == "" {
			errs = append(errs, &ValidationError{Field: "storage.outbox.stream_key", Message: "required when outbox enabled"})
		}
		if s.Outbox.MaxLen <= 0 {
			errs = append(errs, &ValidationError{Field: "storage.outbox.max_len", Message: "must be > 0 when outbox enabled"})
		}
	}
	// Audit
	if s.Audit.RetentionDays <= 0 {
		errs = append(errs, &ValidationError{Field: "storage.audit.retention_days", Message: "must be > 0"})
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func validateAuthz(a *AuthzConfig) error {
	var errs []error
	if a.DefaultLevel < 1 || a.DefaultLevel > 3 {
		errs = append(errs, &ValidationError{
			Field:   "authz.default_level",
			Message: fmt.Sprintf("must be 1-3, got %d", a.DefaultLevel),
		})
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func validateChain(c *ChainConfig) error {
	var errs []error
	switch c.Driver {
	case "fisco", "polygon", "bsc", "mock":
		// valid
	default:
		errs = append(errs, &ValidationError{
			Field:   "chain.driver",
			Message: fmt.Sprintf("invalid driver %q (want: fisco|polygon|bsc|mock)", c.Driver),
		})
	}
	if c.Driver != "mock" && strings.TrimSpace(c.RPCURL) == "" {
		errs = append(errs, &ValidationError{Field: "chain.rpc_url", Message: "required for non-mock drivers"})
	}
	if c.RequestTimeout < 100*time.Millisecond {
		errs = append(errs, &ValidationError{Field: "chain.request_timeout", Message: "must be >= 100ms"})
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func validateTelemetry(t *TelemetryConfig) error {
	if t.SampleRate < 0 || t.SampleRate > 1 {
		return &ValidationError{
			Field:   "telemetry.sample_rate",
			Message: fmt.Sprintf("must be 0.0-1.0, got %v", t.SampleRate),
		}
	}
	return nil
}

// validRole 校验 role 枚举。
func validRole(r string) bool {
	switch r {
	case "gateway", "auth-center", "tag-sense", "mcp", "migration", "cli":
		return true
	}
	return false
}

// validLogLevel 校验 level 枚举。
func validLogLevel(l string) bool {
	switch l {
	case "debug", "info", "warn", "error":
		return true
	}
	return false
}

// validLogFormat 校验 format 枚举。
func validLogFormat(f string) bool {
	switch f {
	case "json", "text":
		return true
	}
	return false
}

// validAddr 校验 ":port" 或 "host:port" 形式。
func validAddr(addr string) bool {
	if addr == "" {
		return false
	}
	// 简单校验：以 : 开头或 host:port 形式
	if !strings.HasPrefix(addr, ":") && !strings.Contains(addr, ":") {
		return false
	}
	// 必须以数字端口结尾
	idx := strings.LastIndex(addr, ":")
	portStr := addr[idx+1:]
	if portStr == "" {
		return false
	}
	for _, c := range portStr {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validPostgresDSN 校验 PostgreSQL DSN（postgres:// 开头，含 host）。
func validPostgresDSN(dsn string) bool {
	u, err := url.Parse(dsn)
	if err != nil {
		return false
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return false
	}
	if u.Host == "" {
		return false
	}
	return true
}

// MustValidate 校验失败时 panic。
// 用于：main 启动时快速失败。
func (cfg *Config) MustValidate() {
	if err := cfg.Validate(); err != nil {
		panic(fmt.Errorf("config: validation failed: %w", err))
	}
}
