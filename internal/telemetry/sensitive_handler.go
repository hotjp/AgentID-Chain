// Package telemetry: 敏感字段脱敏 Handler (P17.12)。
//
// 包装任何 slog.Handler，自动对以下字段进行脱敏：
//   - 字段名匹配：password / passwd / secret / token / api_key / access_token /
//                 refresh_token / authorization / cookie / session / private_key
//                 / dsn / credential
//   - 字段名包含：secret / key / token / auth
//   - 嵌套 Group 中的字段递归脱敏
//   - 字符串值匹配：Ed25519 base64、HMAC hex、JWT (eyJ...)、DSN 凭据
//
// 脱敏规则：
//   - 长度 ≤ 4：完全替换为 "***"
//   - 长度 > 4：保留首尾各 1 字符 + "***"
//   - 例如 "abcdefgh" → "a***h"
//
// 用法：
//
//	base := slog.NewJSONHandler(os.Stdout, opts)
//	logger := slog.New(telemetry.NewSensitiveHandler(base, nil))
package telemetry

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

// SensitiveConfig 脱敏配置。
type SensitiveConfig struct {
	// ExtraKeys 额外需要脱敏的字段名（大小写不敏感）。
	ExtraKeys []string
	// ExtraPatterns 额外的敏感值模式（正则）。
	ExtraPatterns []string
	// Disabled 完全禁用脱敏（不推荐）。
	Disabled bool
	// Replacement 替换字符串（默认 "***"）。
	Replacement string
	// PreserveLength 是否保留长度（"***" → 等长星号）。
	PreserveLength bool
}

// DefaultSensitiveConfig 返回默认配置。
func DefaultSensitiveConfig() SensitiveConfig {
	return SensitiveConfig{
		Replacement: "***",
	}
}

// sensitivePatterns 内置敏感值模式。
var sensitivePatterns = []*regexp.Regexp{
	// Ed25519 private key (base64 64B+)
	regexp.MustCompile(`\b[A-Za-z0-9+/]{86}==\b`),
	// JWT (eyJ... .eyJ... .signature)
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`),
	// DSN with embedded credentials
	regexp.MustCompile(`(?i)(postgres|postgresql|mysql|mongodb|redis|amqp)://[^:]+:[^@]+@`),
	// AWS access key
	regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`),
	// GitHub PAT
	regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`),
	// Slack token
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,72}\b`),
	// Stripe key
	regexp.MustCompile(`\bsk_live_[A-Za-z0-9]{24,}\b`),
	// PEM private key block
	regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY(?: BLOCK)?-----`),
}

// sensitiveKeyPrefixes 内置敏感字段名前缀。
var sensitiveKeyPrefixes = []string{
	"password", "passwd", "pwd",
	"secret", "token", "api_key", "apikey",
	"access_token", "refresh_token", "auth_token",
	"authorization", "cookie", "session",
	"private_key", "domain_secret", "hmac",
	"dsn", "credential", "credentials",
}

// sensitiveKeyContains 字段名包含即脱敏。
var sensitiveKeyContains = []string{
	"secret", "private", "key", "token", "auth", "password",
}

// SensitiveHandler 包装另一 slog.Handler 做脱敏。
type SensitiveHandler struct {
	inner       slog.Handler
	cfg         SensitiveConfig
	extraKeys   map[string]struct{}
	extraRegex  []*regexp.Regexp
	patterns    []*regexp.Regexp
}

// NewSensitiveHandler 构造脱敏 Handler。
func NewSensitiveHandler(inner slog.Handler, cfg *SensitiveConfig) *SensitiveHandler {
	if inner == nil {
		inner = slog.Default().Handler()
	}
	c := DefaultSensitiveConfig()
	if cfg != nil {
		c = *cfg
	}
	h := &SensitiveHandler{
		inner:      inner,
		cfg:        c,
		extraKeys:  make(map[string]struct{}, len(c.ExtraKeys)),
		patterns:   sensitivePatterns,
		extraRegex: make([]*regexp.Regexp, 0, len(c.ExtraPatterns)),
	}
	for _, k := range c.ExtraKeys {
		h.extraKeys[strings.ToLower(k)] = struct{}{}
	}
	for _, p := range c.ExtraPatterns {
		if re, err := regexp.Compile(p); err == nil {
			h.extraRegex = append(h.extraRegex, re)
		}
	}
	return h
}

// Enabled 透传。
func (h *SensitiveHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle 脱敏 attr 后转发。
func (h *SensitiveHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.cfg.Disabled {
		return h.inner.Handle(ctx, r)
	}
	cloned := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		cloned.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, cloned)
}

// WithAttrs 递归脱敏新 attrs。
func (h *SensitiveHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		redacted = append(redacted, h.redactAttr(a))
	}
	return &SensitiveHandler{
		inner:      h.inner.WithAttrs(redacted),
		cfg:        h.cfg,
		extraKeys:  h.extraKeys,
		patterns:   h.patterns,
		extraRegex: h.extraRegex,
	}
}

// WithGroup 透传但保留脱敏能力。
func (h *SensitiveHandler) WithGroup(name string) slog.Handler {
	return &SensitiveHandler{
		inner:      h.inner.WithGroup(name),
		cfg:        h.cfg,
		extraKeys:  h.extraKeys,
		patterns:   h.patterns,
		extraRegex: h.extraRegex,
	}
}

// redactAttr 对单个 attr 脱敏。
func (h *SensitiveHandler) redactAttr(a slog.Attr) slog.Attr {
	// 1. 嵌套 group — 始终递归进入（结构优先于 key 判定）
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		redacted := make([]slog.Attr, 0, len(attrs))
		for _, ga := range attrs {
			redacted = append(redacted, h.redactAttr(ga))
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(redacted...)}
	}
	// 2. 敏感 key → 整串替换
	if h.isSensitiveKey(a.Key) {
		return h.redactValue(a)
	}
	// 3. 字符串值模式匹配（key 不敏感但 value 可能是 JWT/DSN 等）
	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, h.redactString(a.Value.String()))
	}
	return a
}

// redactValue 替换值为脱敏串。
//   - 字符串：按模式脱敏（JWT、DSN 等）
//   - 敏感 key 的字符串：整串替换（按 PreserveLength 决定形式）
//   - 非字符串：替换为 "***"
func (h *SensitiveHandler) redactValue(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindString {
		s := a.Value.String()
		if h.cfg.PreserveLength {
			return slog.String(a.Key, maskMiddle(s, h.cfg))
		}
		return slog.String(a.Key, h.cfg.Replacement)
	}
	return slog.String(a.Key, h.cfg.Replacement)
}

// redactString 字符串脱敏（保留首尾 + 模式替换）。
func (h *SensitiveHandler) redactString(s string) string {
	if s == "" {
		return s
	}
	out := s
	// 内置模式
	for _, re := range h.patterns {
		out = re.ReplaceAllStringFunc(out, func(match string) string {
			return maskMiddle(match, h.cfg)
		})
	}
	// 额外模式
	for _, re := range h.extraRegex {
		out = re.ReplaceAllStringFunc(out, func(match string) string {
			return maskMiddle(match, h.cfg)
		})
	}
	return out
}

func (h *SensitiveHandler) isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	if _, ok := h.extraKeys[lower]; ok {
		return true
	}
	for _, p := range sensitiveKeyPrefixes {
		if lower == p || strings.HasPrefix(lower, p) {
			return true
		}
	}
	for _, c := range sensitiveKeyContains {
		if strings.Contains(lower, c) {
			return true
		}
	}
	return false
}

// maskMiddle 保留首尾各 1 字符 + ***；长度 ≤4 全部 ***。
func maskMiddle(s string, cfg SensitiveConfig) string {
	if len(s) <= 4 {
		if cfg.PreserveLength {
			return strings.Repeat("*", len(s))
		}
		return cfg.Replacement
	}
	if cfg.PreserveLength {
		// 保留首尾各 1 字符，中间填充等长 *
		return string(s[0]) + strings.Repeat("*", len(s)-2) + string(s[len(s)-1])
	}
	return string(s[0]) + cfg.Replacement + string(s[len(s)-1])
}
