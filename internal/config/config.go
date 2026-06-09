// Package config 提供 AgentID-Chain 全局配置加载与默认值管理。
//
// 加载顺序（与 docs/AgentID-Chain-技术文档-v2.0.1.md §5.3 对齐）：
//  1. 内置 defaults（New()）
//  2. YAML 文件（--config / -c 指定；可选）
//  3. 环境变量（AGENTID_ 前缀，区分大小写映射，例 AGENTID_DB_DSN → cfg.Storage.DB.DSN）
//  4. 命令行 flag（最高优先级）
//
// 所有路径必须以本包内 Koanf 实例操作为准，禁止外部持有 Koanf 实例。
package config

import (
	"time"
)

// Config 根配置（与 backend.yaml schema 一致；参考 docs §5.2）
type Config struct {
	// 服务基本信息
	Service ServiceConfig `koanf:"service"`
	// 日志
	Log LogConfig `koanf:"log"`
	// 存储后端
	Backend BackendConfig `koanf:"backend"`
	// L1 Storage
	Storage StorageConfig `koanf:"storage"`
	// L3 Authz
	Authz AuthzConfig `koanf:"authz"`
	// 链上
	Chain ChainConfig `koanf:"chain"`
	// A2A
	A2A A2AConfig `koanf:"a2a"`
	// AAP
	AAP AAPConfig `koanf:"aap"`
	// MoltCaptcha
	MoltCaptcha MoltCaptchaConfig `koanf:"moltcaptcha"`
	// 限流
	RateLimit RateLimitConfig `koanf:"ratelimit"`
	// 观测
	Telemetry TelemetryConfig `koanf:"telemetry"`
}

// ServiceConfig 服务基础信息。
type ServiceConfig struct {
	Name     string `koanf:"name"`
	Version  string `koanf:"version"`
	Role     string `koanf:"role"`      // gateway|auth-center|tag-sense|mcp|migration|cli
	HTTPAddr string `koanf:"http_addr"` // 业务 HTTP 端口
	GRPCAddr string `koanf:"grpc_addr"` // gRPC 端口（通常与 HTTP 同端口）
}

// LogConfig 日志配置。
type LogConfig struct {
	Level  string `koanf:"level"`  // debug|info|warn|error
	Format string `koanf:"format"` // json|text
	Source bool   `koanf:"source"` // 是否带 source 字段
}

// BackendConfig 后端模式配置。
type BackendConfig struct {
	Type string `koanf:"type"` // local|onchain|hybrid
}

// StorageConfig L1 存储总配置。
type StorageConfig struct {
	DB         DBConfig         `koanf:"db"`
	Redis      RedisConfig      `koanf:"redis"`
	Outbox     OutboxConfig     `koanf:"outbox"`
	Audit      AuditConfig      `koanf:"audit"`
	Revocation RevocationConfig `koanf:"revocation"`
}

// DBConfig PostgreSQL 连接池配置。
type DBConfig struct {
	Driver      string        `koanf:"driver"` // 仅 postgres (v2.0.1 强制)
	DSN         string        `koanf:"dsn"`
	MaxOpen     int           `koanf:"max_open"`
	MaxIdle     int           `koanf:"max_idle"`
	MaxLifetime time.Duration `koanf:"max_lifetime"`
}

// RedisConfig Redis 客户端配置。
type RedisConfig struct {
	Addr     string        `koanf:"addr"`
	Password string        `koanf:"password"`
	DB       int           `koanf:"db"`
	Timeout  time.Duration `koanf:"timeout"`
}

// OutboxConfig 领域事件 → Redis Stream 转发配置。
type OutboxConfig struct {
	Enabled   bool          `koanf:"enabled"`
	StreamKey string        `koanf:"stream_key"`
	MaxLen    int64         `koanf:"max_len"`
	GroupName string        `koanf:"group_name"`
	BlockTime time.Duration `koanf:"block_time"`
}

// AuditConfig 审计日志保留与批处理配置。
type AuditConfig struct {
	RetentionDays int `koanf:"retention_days"`
	BatchSize     int `koanf:"batch_size"`
}

// RevocationConfig A2A Token 撤销列表配置。
type RevocationConfig struct {
	Enabled        bool          `koanf:"enabled"`
	CheckOnRequest bool          `koanf:"check_on_request"`
	CacheTTL       time.Duration `koanf:"cache_ttl"`
}

// AuthzConfig L3 鉴权总配置。
type AuthzConfig struct {
	RBAC         RBACConfig `koanf:"rbac"`
	DefaultLevel uint8      `koanf:"default_level"`
	StrictMode   bool       `koanf:"strict_mode"` // 严格模式：所有 API 必须带 AAP 令牌
}

// RBACConfig 基于位掩码的 RBAC 配置。
type RBACConfig struct {
	// 等级 → 最大权限位掩码（默认由 domain.LevelType.DefaultMaxPermissions 计算）
	LevelMaxPermissions map[string]uint64 `koanf:"level_max_permissions"`
}

// ChainConfig 多链适配器配置。
type ChainConfig struct {
	Driver             string        `koanf:"driver"` // fisco|polygon|bsc|mock
	RPCURL             string        `koanf:"rpc_url"`
	ContractAddress    string        `koanf:"contract_address"`
	OperatorPrivateKey string        `koanf:"operator_private_key"`
	GasLimit           uint64        `koanf:"gas_limit"`
	ConfirmationBlocks uint64        `koanf:"confirmation_blocks"`
	RequestTimeout     time.Duration `koanf:"request_timeout"`
}

// A2AConfig A2A Token 签发与互认配置。
type A2AConfig struct {
	Issuer                    string        `koanf:"issuer"`
	SigningKeyID              string        `koanf:"signing_key_id"`
	SigningPrivateKey         string        `koanf:"signing_private_key"` // base64 Ed25519
	JWKSRefreshInterval       time.Duration `koanf:"jwks_refresh_interval"`
	TokenTTL                  time.Duration `koanf:"token_ttl"`
	MaxNegotiationsPerHour    int           `koanf:"max_negotiations_per_hour"`
	AllowedPermissionOverride bool          `koanf:"allowed_permission_override"`
	AuditAllNegotiations      bool          `koanf:"audit_all_negotiations"`
}

// AAPConfig Agent Admission Protocol 配置。
type AAPConfig struct {
	ChallengeTTL       time.Duration `koanf:"challenge_ttl"`
	ResponseMaxTTL     time.Duration `koanf:"response_max_ttl"`
	DomainPrivateKey   string        `koanf:"domain_private_key"`
	DomainPublicKeyJWK string        `koanf:"domain_public_key_jwk"`
	RequireHTTPS       bool          `koanf:"require_https"`
}

// MoltCaptchaConfig MoltCaptcha SMHL 反向 CAPTCHA 配置。
type MoltCaptchaConfig struct {
	Difficulty string        `koanf:"difficulty"` // easy|medium|hard|extreme
	TopicPool  []string      `koanf:"topic_pool"`
	TTL        time.Duration `koanf:"ttl"`
}

// RateLimitConfig 速率限制配置。
type RateLimitConfig struct {
	PerIP    string `koanf:"per_ip"`    // 10/min
	PerAgent string `koanf:"per_agent"` // 100/min
	Enabled  bool   `koanf:"enabled"`
}

// TelemetryConfig 观测配置（OTel / Metrics / Pprof）。
type TelemetryConfig struct {
	ServiceName  string  `koanf:"service_name"`
	OTELEndpoint string  `koanf:"otel_endpoint"`
	MetricsAddr  string  `koanf:"metrics_addr"`
	PProfAddr    string  `koanf:"pprof_addr"`
	SampleRate   float64 `koanf:"sample_rate"`
}

// New 返回带默认值的 Config。
func New() *Config {
	return &Config{
		Service: ServiceConfig{
			Name:     "agentid-chain",
			Version:  "dev",
			Role:     "gateway",
			HTTPAddr: ":8080",
			GRPCAddr: ":8080",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
			Source: false,
		},
		Backend: BackendConfig{
			Type: "local",
		},
		Storage: StorageConfig{
			DB: DBConfig{
				Driver:      "postgres",
				DSN:         "postgres://agentid:agentid_dev@localhost:5432/agentid?sslmode=disable",
				MaxOpen:     25,
				MaxIdle:     10,
				MaxLifetime: 5 * time.Minute,
			},
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
				Timeout:  3 * time.Second,
			},
			Outbox: OutboxConfig{
				Enabled:   true,
				StreamKey: "agentid:outbox",
				MaxLen:    100_000,
				GroupName: "agentid-consumers",
				BlockTime: 5 * time.Second,
			},
			Audit: AuditConfig{
				RetentionDays: 365,
				BatchSize:     100,
			},
			Revocation: RevocationConfig{
				Enabled:        true,
				CheckOnRequest: true,
				CacheTTL:       30 * time.Second,
			},
		},
		Authz: AuthzConfig{
			DefaultLevel: 1,
			StrictMode:   true,
		},
		Chain: ChainConfig{
			Driver:             "mock",
			RPCURL:             "http://localhost:8545",
			GasLimit:           300_000,
			ConfirmationBlocks: 1,
			RequestTimeout:     10 * time.Second,
		},
		A2A: A2AConfig{
			Issuer:                    "agentid-chain",
			SigningKeyID:              "agentid-gateway-2026",
			JWKSRefreshInterval:       time.Hour,
			TokenTTL:                  time.Hour,
			MaxNegotiationsPerHour:    100,
			AllowedPermissionOverride: false,
			AuditAllNegotiations:      true,
		},
		AAP: AAPConfig{
			ChallengeTTL:   5 * time.Minute,
			ResponseMaxTTL: 10 * time.Minute,
			RequireHTTPS:   false,
		},
		MoltCaptcha: MoltCaptchaConfig{
			Difficulty: "medium",
			TopicPool: []string{
				"verification", "authenticity", "digital trust",
				"cryptography", "identity", "algorithms",
				"neural networks", "computation", "binary",
				"protocols", "encryption", "tokens", "agents",
				"automation", "circuits", "logic gates", "recursion",
				"entropy", "hashing", "signatures",
			},
			TTL: 30 * time.Second,
		},
		RateLimit: RateLimitConfig{
			PerIP:    "10/min",
			PerAgent: "100/min",
			Enabled:  true,
		},
		Telemetry: TelemetryConfig{
			ServiceName: "agentid-chain",
			MetricsAddr: ":9090",
			PProfAddr:   ":6060",
			SampleRate:  1.0,
		},
	}
}
