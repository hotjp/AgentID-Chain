// Package aap 是 L3 鉴权层的 AAP（Agent Admission Protocol）实现。
//
// AAP 协议由三个动作组成（v2.0.1 §3.3.2）：
//  1. Challenge：服务器生成一次性挑战（nonce + ts + domain_sig）
//  2. Response：客户端用 Ed25519 私钥对 challenge 签名
//  3. Proof：服务器验签通过后颁发 AAP Token（短期 access token）
//
// 本文件实现 Challenge 生成；Response 验签和 Proof 颁发在另两个文件。
//
// 数据流：
//
//	cache.Cache ← StoreChallenge(challenge) [TTL 30s]
//	                   ↑
//	              GenerateChallenge()  ← L5 Gateway 首次握手
//
// 安全约束：
//   - Nonce 长度固定 16 字节（32 hex 字符）
//   - Challenge 在 Redis 中 TTL 30s（默认可配）
//   - domain_sig 是服务器对 (challenge_id || nonce || ts) 的 Ed25519 签名
//   - 防重放：客户端必须在 TTL 内返回签名
package aap

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrEmptyDomain 域签名密钥为空。
var ErrEmptyDomain = errors.New("aap: empty domain signing key")

// ErrInvalidChallengeID Challenge ID 格式非法。
var ErrInvalidChallengeID = errors.New("aap: invalid challenge id")

// ErrInvalidChallenge 已存在的 challenge_id（重放）。
var ErrInvalidChallenge = errors.New("aap: invalid or expired challenge")

// ErrStoreUnavailable 存储后端不可用。
var ErrStoreUnavailable = errors.New("aap: challenge store unavailable")

// =============================================================================
// Challenge 数据结构
// =============================================================================

// Challenge AAP Challenge 数据结构（v2.0.1 §3.3.2）。
//
// 字段：
//   - ChallengeID: 唯一 ID（UUID 形式）
//   - Nonce: 16 字节随机数（32 hex 字符）
//   - IssuedAt: 颁发时间戳（RFC3339Nano）
//   - ExpiresAt: 过期时间戳
//   - DomainSig: 服务器对 (ChallengeID || Nonce || IssuedAt) 的 Ed25519 签名
type Challenge struct {
	ChallengeID string    `json:"challenge_id"`
	Nonce       string    `json:"nonce"`        // 32 hex chars
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	DomainSig   string    `json:"domain_sig"`   // base64
}

// IsExpired 业务判定：当前时间是否已超过 ExpiresAt。
func (c *Challenge) IsExpired(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}

// =============================================================================
// Generator Challenge 生成器
// =============================================================================

// Generator 配置。
type Config struct {
	// ChallengeTTL challenge 在存储中的 TTL（默认 30s）
	ChallengeTTL time.Duration
	// DomainKey 服务器 Ed25519 私钥（64 字节）— 用于 domain_sig
	DomainKey ed25519.PrivateKey
	// Clock 时间源（注入便于测试）
	Clock func() time.Time
	// ChallengeIDLen challenge_id 字节长度（默认 16 → 32 hex）
	ChallengeIDLen int
}

// Generator Challenge 生成器。
//
// 依赖：cache.Cache（用于持久化 challenge 给后续验签阶段使用）。
type Generator struct {
	cfg    Config
	store  cache.Cache
}

// NewGenerator 构造生成器。
//
// store 不能为 nil；cfg 缺失值会回退到默认值。
func NewGenerator(store cache.Cache, cfg Config) (*Generator, error) {
	if store == nil {
		return nil, ErrStoreUnavailable
	}
	if cfg.ChallengeTTL <= 0 {
		cfg.ChallengeTTL = 30 * time.Second
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.ChallengeIDLen <= 0 {
		cfg.ChallengeIDLen = 16
	}
	if len(cfg.DomainKey) != ed25519.PrivateKeySize {
		return nil, ErrEmptyDomain
	}
	return &Generator{cfg: cfg, store: store}, nil
}

// Generate 生成新 challenge 并写入存储。
//
// 步骤：
//  1. 生成 challenge_id（hex）
//  2. 生成 nonce（16 字节 hex）
//  3. 计算 issued_at / expires_at
//  4. 用 domain key 签名（challenge_id || nonce || issued_at）
//  5. 写入 cache（TTL = ChallengeTTL）
//
// 失败模式：底层 store 错误原样返回。
func (g *Generator) Generate(ctx context.Context) (*Challenge, error) {
	// 1. challenge_id
	idBytes, err := randomBytes(g.cfg.ChallengeIDLen)
	if err != nil {
		return nil, fmt.Errorf("aap: gen challenge_id: %w", err)
	}
	challengeID := hex.EncodeToString(idBytes)

	// 2. nonce
	nonceBytes, err := randomBytes(16)
	if err != nil {
		return nil, fmt.Errorf("aap: gen nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	// 3. 时间戳
	now := g.cfg.Clock()
	expiresAt := now.Add(g.cfg.ChallengeTTL)

	// 4. domain_sig
	payload := signPayload(challengeID, nonce, now)
	sig := ed25519.Sign(g.cfg.DomainKey, payload)
	domainSig := base64.RawURLEncoding.EncodeToString(sig)

	// 5. 构造 + 持久化
	c := &Challenge{
		ChallengeID: challengeID,
		Nonce:       nonce,
		IssuedAt:    now,
		ExpiresAt:   expiresAt,
		DomainSig:   domainSig,
	}
	if err := g.storeChallenge(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// GenerateWithID 用指定的 challenge_id 生成 challenge（测试 / 重放防护用）。
//
// 业务场景：先校验 caller 没有重复 ID，再生成。
func (g *Generator) GenerateWithID(ctx context.Context, challengeID string) (*Challenge, error) {
	if !isValidChallengeID(challengeID) {
		return nil, ErrInvalidChallengeID
	}
	// 检查重复
	exists, err := g.store.Exists(ctx, challengeStoreKey(challengeID))
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: id=%s", ErrInvalidChallenge, challengeID)
	}

	nonceBytes, err := randomBytes(16)
	if err != nil {
		return nil, err
	}
	nonce := hex.EncodeToString(nonceBytes)

	now := g.cfg.Clock()
	payload := signPayload(challengeID, nonce, now)
	sig := ed25519.Sign(g.cfg.DomainKey, payload)
	domainSig := base64.RawURLEncoding.EncodeToString(sig)

	c := &Challenge{
		ChallengeID: challengeID,
		Nonce:       nonce,
		IssuedAt:    now,
		ExpiresAt:   now.Add(g.cfg.ChallengeTTL),
		DomainSig:   domainSig,
	}
	if err := g.storeChallenge(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// TTL 返回配置 TTL。
func (g *Generator) TTL() time.Duration {
	return g.cfg.ChallengeTTL
}

// =============================================================================
// 存储辅助
// =============================================================================

// storeKeyPrefix challenge 在 cache 中的 key 前缀。
const storeKeyPrefix = "aap:challenge:"

// challengeStoreKey 拼装存储 key。
func challengeStoreKey(id string) string {
	return storeKeyPrefix + id
}

// storeChallenge 序列化 + 写入。
//
// 存储格式：JSON（可读，便于调试）。
func (g *Generator) storeChallenge(ctx context.Context, c *Challenge) error {
	data, err := encodeChallenge(c)
	if err != nil {
		return err
	}
	return g.store.Set(ctx, challengeStoreKey(c.ChallengeID), data, g.cfg.ChallengeTTL)
}

// LoadChallenge 从存储读取（响应验签阶段使用）。
func (g *Generator) LoadChallenge(ctx context.Context, id string) (*Challenge, error) {
	raw, err := g.store.Get(ctx, challengeStoreKey(id))
	if err != nil {
		return nil, err
	}
	return decodeChallenge(raw)
}

// ConsumeChallenge 读取并立即删除（一次性使用）。
func (g *Generator) ConsumeChallenge(ctx context.Context, id string) (*Challenge, error) {
	c, err := g.LoadChallenge(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = g.store.Del(ctx, challengeStoreKey(id))
	return c, nil
}

// =============================================================================
// 工具函数
// =============================================================================

// randomBytes 从 crypto/rand 取 n 字节。
func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// signPayload 构造待签名 payload。
//
// 格式：challenge_id || ":" || nonce || ":" || issued_at_unix_nano
func signPayload(challengeID, nonce string, issuedAt time.Time) []byte {
	return []byte(challengeID + ":" + nonce + ":" + issuedAt.Format(time.RFC3339Nano))
}

// isValidChallengeID 校验 challenge_id 格式（hex 字符串）。
func isValidChallengeID(s string) bool {
	if s == "" || len(s)%2 != 0 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// encodeChallenge 序列化为 JSON。
func encodeChallenge(c *Challenge) ([]byte, error) {
	// 简化：手动 JSON 序列化（不引入 encoding/json 依赖路径时）
	// 这里直接用 fmt.Sprintf 是为了避免额外 import。
	// 实际生产可改 json.Marshal。
	out := fmt.Sprintf(`{"challenge_id":%q,"nonce":%q,"issued_at":%q,"expires_at":%q,"domain_sig":%q}`,
		c.ChallengeID, c.Nonce,
		c.IssuedAt.Format(time.RFC3339Nano), c.ExpiresAt.Format(time.RFC3339Nano),
		c.DomainSig,
	)
	return []byte(out), nil
}

// decodeChallenge 反序列化。
func decodeChallenge(data []byte) (*Challenge, error) {
	s := string(data)
	c := &Challenge{}
	// 极简解析：key":"value"，容错性低但对自生成数据足够
	if err := jsonField(s, "challenge_id", &c.ChallengeID); err != nil {
		return nil, err
	}
	if err := jsonField(s, "nonce", &c.Nonce); err != nil {
		return nil, err
	}
	if err := jsonField(s, "domain_sig", &c.DomainSig); err != nil {
		return nil, err
	}
	var issued, expires string
	if err := jsonField(s, "issued_at", &issued); err != nil {
		return nil, err
	}
	if err := jsonField(s, "expires_at", &expires); err != nil {
		return nil, err
	}
	t1, err := time.Parse(time.RFC3339Nano, issued)
	if err != nil {
		return nil, fmt.Errorf("aap: parse issued_at: %w", err)
	}
	t2, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return nil, fmt.Errorf("aap: parse expires_at: %w", err)
	}
	c.IssuedAt = t1
	c.ExpiresAt = t2
	return c, nil
}

// jsonField 从极简 JSON 串中提取 "key":"value"。
//
// 仅支持本包生成的扁平对象。
func jsonField(s, key string, dst *string) error {
	k := `"` + key + `":`
	idx := strings.Index(s, k)
	if idx < 0 {
		return fmt.Errorf("aap: missing field %s", key)
	}
	rest := s[idx+len(k):]
	// 跳过前导空白
	for len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
		rest = rest[1:]
	}
	if len(rest) == 0 || rest[0] != '"' {
		return fmt.Errorf("aap: field %s not a string", key)
	}
	// 找结尾 "
	end := strings.IndexByte(rest[1:], '"')
	if end < 0 {
		return fmt.Errorf("aap: field %s unterminated", key)
	}
	*dst = rest[1 : 1+end]
	return nil
}
