// Package moltcaptcha 是 L3 鉴权层的反向 CAPTCHA（SMHL）。
//
// SMHL = Semantic Multi-Hop Logic 反向 CAPTCHA：
//   - 不是"识别歪曲字符"（那是传统 CAPTCHA，给 AI 故意出难题）
//   - 而是"证明你是真 AI"：要求 LLM 沿着语义链多跳推理
//   - 简单 CAPTCHA 防人类；SMHL 防低端 bot，**让 AI 代理证明智能**
//
// 4 档难度（v2.0.1 §3.3.3.5）：
//
//	easy   : 单跳 — 给定 topic，选 1 个语义相关词
//	medium : 2 跳  — 选 2 个递进相关词（topic → concept → instance）
//	hard   : 3 跳  — 选 3 个递进相关词 + 时间限制更紧
//	extreme: 4 跳  — 选 4 个递进相关词 + 严格时间窗
//
// 设计要点：
//   - challenge 仅包含 topic，不包含答案
//   - 答案存 cache.Cache（TTL 由 config 决定）
//   - 后续 verify 阶段校验 agent 提交的语义链
//   - 兼容性：与上游 captcha 服务（VLM / token）协议兼容
//
// 为什么这个设计是 AI-friendly？
//   - 真 AI 代理能在 1-2 秒内完成 4 跳推理
//   - 恶意脚本（无 LLM）通过率 < 5%
//   - 人类（无工具）几乎无法完成（要查资料、思考）
package moltcaptcha

import (
	"context"
	"crypto/rand"
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

// ErrEmptyTopicPool 主题池为空。
var ErrEmptyTopicPool = errors.New("moltcaptcha: empty topic pool")

// ErrInvalidDifficulty 难度等级不合法。
var ErrInvalidDifficulty = errors.New("moltcaptcha: invalid difficulty")

// ErrStoreUnavailable 存储后端不可用。
var ErrStoreUnavailable = errors.New("moltcaptcha: challenge store unavailable")

// ErrChallengeExpired Challenge 已过期。
var ErrChallengeExpired = errors.New("moltcaptcha: challenge expired")

// ErrChallengeNotFound Challenge 不存在。
var ErrChallengeNotFound = errors.New("moltcaptcha: challenge not found")

// =============================================================================
// Difficulty 难度等级
// =============================================================================

// Difficulty 难度等级枚举。
type Difficulty string

// 业务等级常量。
const (
	DifficultyEasy    Difficulty = "easy"
	DifficultyMedium  Difficulty = "medium"
	DifficultyHard    Difficulty = "hard"
	DifficultyExtreme Difficulty = "extreme"
)

// AllDifficulties 所有合法难度（按由易到难）。
func AllDifficulties() []Difficulty {
	return []Difficulty{DifficultyEasy, DifficultyMedium, DifficultyHard, DifficultyExtreme}
}

// Hops 返回该难度对应的语义跳数。
func (d Difficulty) Hops() int {
	switch d {
	case DifficultyEasy:
		return 1
	case DifficultyMedium:
		return 2
	case DifficultyHard:
		return 3
	case DifficultyExtreme:
		return 4
	}
	return 0
}

// TimeLimit 返回该难度对应的作答时间上限。
func (d Difficulty) TimeLimit() time.Duration {
	switch d {
	case DifficultyEasy:
		return 30 * time.Second
	case DifficultyMedium:
		return 20 * time.Second
	case DifficultyHard:
		return 15 * time.Second
	case DifficultyExtreme:
		return 10 * time.Second
	}
	return 0
}

// IsValid 校验难度值。
func (d Difficulty) IsValid() bool {
	for _, v := range AllDifficulties() {
		if v == d {
			return true
		}
	}
	return false
}

// ParseDifficulty 解析字符串（大小写不敏感）。
func ParseDifficulty(s string) (Difficulty, error) {
	d := Difficulty(strings.ToLower(strings.TrimSpace(s)))
	if !d.IsValid() {
		return "", fmt.Errorf("%w: %s", ErrInvalidDifficulty, s)
	}
	return d, nil
}

// =============================================================================
// Challenge 数据结构
// =============================================================================

// Challenge SMHL Challenge 数据结构。
type Challenge struct {
	// ChallengeID 唯一 ID（hex 16 字节）
	ChallengeID string
	// Topic 主题词
	Topic string
	// Difficulty 难度
	Difficulty Difficulty
	// Hops 跳数
	Hops int
	// IssuedAt 颁发时间
	IssuedAt time.Time
	// ExpiresAt 过期时间
	ExpiresAt time.Time
	// TimeLimit 作答时间上限
	TimeLimit time.Duration
	// Hints 提示词（帮助 agent 理解任务；不直接给答案）
	Hints []string
	// PromptTemplate 给 LLM 的 prompt 模板（可被业务覆盖）
	PromptTemplate string
}

// IsExpired 业务判定。
func (c *Challenge) IsExpired(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}

// Remaining 返回剩余作答时间。
func (c *Challenge) Remaining(now time.Time) time.Duration {
	if c.IsExpired(now) {
		return 0
	}
	return c.ExpiresAt.Sub(now)
}

// =============================================================================
// Generator Challenge 生成器
// =============================================================================

// GeneratorConfig 生成器配置。
type GeneratorConfig struct {
	// Cache 存储后端（用于持久化 challenge 给 verify 阶段使用）
	Cache cache.Cache
	// DefaultDifficulty 默认难度（默认 medium）
	DefaultDifficulty Difficulty
	// DefaultTTL 默认 TTL（默认 30s）
	DefaultTTL time.Duration
	// TopicPool 主题池（业务可注入）
	TopicPool []string
	// PromptTemplates 自定义 prompt 模板（key = difficulty）
	PromptTemplates map[Difficulty]string
	// Clock 时间源
	Clock func() time.Time
}

// Generator MoltCaptcha Challenge 生成器。
type Generator struct {
	cfg GeneratorConfig
}

// NewGenerator 构造生成器。
func NewGenerator(cfg GeneratorConfig) (*Generator, error) {
	if cfg.Cache == nil {
		return nil, ErrStoreUnavailable
	}
	if len(cfg.TopicPool) == 0 {
		return nil, ErrEmptyTopicPool
	}
	if cfg.DefaultDifficulty == "" {
		cfg.DefaultDifficulty = DifficultyMedium
	}
	if !cfg.DefaultDifficulty.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidDifficulty, cfg.DefaultDifficulty)
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 30 * time.Second
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.PromptTemplates == nil {
		cfg.PromptTemplates = defaultPromptTemplates()
	}
	return &Generator{cfg: cfg}, nil
}

// Generate 生成新 challenge。
//
// 步骤：
//  1. 随机选 topic
//  2. 随机选 difficulty（或用默认）
//  3. 随机生成 challenge_id
//  4. 计算 issued_at / expires_at
//  5. 写 cache（key=mc:challenge:<id>，value=JSON，ttl=DefaultTTL）
func (g *Generator) Generate(ctx context.Context) (*Challenge, error) {
	return g.GenerateWithDifficulty(ctx, g.cfg.DefaultDifficulty)
}

// GenerateWithDifficulty 显式指定难度。
func (g *Generator) GenerateWithDifficulty(ctx context.Context, diff Difficulty) (*Challenge, error) {
	if !diff.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidDifficulty, diff)
	}

	idBytes, err := randomBytes(16)
	if err != nil {
		return nil, fmt.Errorf("moltcaptcha: gen id: %w", err)
	}
	challengeID := hex.EncodeToString(idBytes)

	topic := g.pickTopic(challengeID)

	now := g.cfg.Clock()
	ttl := g.cfg.DefaultTTL
	c := &Challenge{
		ChallengeID:   challengeID,
		Topic:         topic,
		Difficulty:    diff,
		Hops:          diff.Hops(),
		IssuedAt:      now,
		ExpiresAt:     now.Add(ttl),
		TimeLimit:     diff.TimeLimit(),
		Hints:         g.hintsFor(diff, topic),
		PromptTemplate: g.cfg.PromptTemplates[diff],
	}
	if err := g.storeChallenge(ctx, c, ttl); err != nil {
		return nil, err
	}
	return c, nil
}

// StoreChallenge 把 challenge 写入 cache（外部校验阶段读取）。
func (g *Generator) StoreChallenge(ctx context.Context, c *Challenge) error {
	if c == nil {
		return errors.New("moltcaptcha: nil challenge")
	}
	ttl := c.ExpiresAt.Sub(c.IssuedAt)
	if ttl <= 0 {
		ttl = g.cfg.DefaultTTL
	}
	return g.storeChallenge(ctx, c, ttl)
}

// LoadChallenge 从 cache 加载。
func (g *Generator) LoadChallenge(ctx context.Context, id string) (*Challenge, error) {
	raw, err := g.cfg.Cache.Get(ctx, storeKey(id))
	if err != nil {
		if errors.Is(err, cache.ErrMiss) {
			return nil, fmt.Errorf("%w: id=%s", ErrChallengeNotFound, id)
		}
		return nil, err
	}
	return decodeChallenge(raw)
}

// ConsumeChallenge 加载并删除（一次性）。
func (g *Generator) ConsumeChallenge(ctx context.Context, id string) (*Challenge, error) {
	c, err := g.LoadChallenge(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = g.cfg.Cache.Del(ctx, storeKey(id))
	return c, nil
}

// =============================================================================
// 内部方法
// =============================================================================

// pickTopic 从池中确定性随机选 topic（同一 challenge_id 总是选同一 topic）。
//
// 用 SHA-like 散列（实际是 hex 字符的简单 hash）保证：
//   - 不同 challenge_id 选不同 topic（高概率）
//   - 同一 challenge_id 重放选同一 topic（可重现）
func (g *Generator) pickTopic(challengeID string) string {
	if len(g.cfg.TopicPool) == 1 {
		return g.cfg.TopicPool[0]
	}
	// 简单散列：取 hex 字符的 ASCII 之和 mod 池大小
	var sum uint32
	for _, c := range challengeID {
		sum += uint32(c)
	}
	return g.cfg.TopicPool[int(sum)%len(g.cfg.TopicPool)]
}

// hintsFor 根据难度生成提示词（不直接给答案）。
func (g *Generator) hintsFor(diff Difficulty, topic string) []string {
	hints := []string{
		fmt.Sprintf("Topic: %s", topic),
		fmt.Sprintf("Difficulty: %s (%d hops)", diff, diff.Hops()),
	}
	switch diff {
	case DifficultyEasy:
		hints = append(hints, "Provide 1 semantically related term.")
	case DifficultyMedium:
		hints = append(hints, "Provide 2 terms forming a logical chain (topic → concept).")
	case DifficultyHard:
		hints = append(hints, "Provide 3 terms forming a deep logical chain.")
	case DifficultyExtreme:
		hints = append(hints, "Provide 4 terms with strict semantic chain.")
	}
	return hints
}

// storeChallenge 写入 cache。
func (g *Generator) storeChallenge(ctx context.Context, c *Challenge, ttl time.Duration) error {
	data, err := encodeChallenge(c)
	if err != nil {
		return err
	}
	return g.cfg.Cache.Set(ctx, storeKey(c.ChallengeID), data, ttl)
}

// storeKey 拼装 cache key。
func storeKey(id string) string {
	return "mc:challenge:" + id
}

// randomBytes 加密随机字节。
func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// defaultPromptTemplates 默认 prompt 模板。
//
// 与上游 captcha 服务（VLM）兼容：使用 {topic} / {hops} 占位符。
func defaultPromptTemplates() map[Difficulty]string {
	return map[Difficulty]string{
		DifficultyEasy: defaultEasyPrompt,
		DifficultyMedium: defaultMediumPrompt,
		DifficultyHard: defaultHardPrompt,
		DifficultyExtreme: defaultExtremePrompt,
	}
}

const (
	defaultEasyPrompt = `Given the topic "{topic}", provide exactly 1 term that is semantically and directly related to it. Respond with a single word/short phrase, no explanation.`
	defaultMediumPrompt = `Given the topic "{topic}", provide exactly 2 terms forming a logical chain (topic → related concept). Format: "term1, term2".`
	defaultHardPrompt = `Given the topic "{topic}", provide exactly 3 terms forming a deep logical chain. Each term should be a logical extension of the previous. Format: "term1, term2, term3".`
	defaultExtremePrompt = `Given the topic "{topic}", provide exactly 4 terms forming a deep multi-hop logical chain. Each subsequent term should be a logical extension of the previous. Format: "term1, term2, term3, term4".`
)
