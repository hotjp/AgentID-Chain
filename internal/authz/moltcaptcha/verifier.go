// Package moltcaptcha SMHL 验证器（verifier.go）。
//
// 验证规则（v2.0.1 §3.3.3.6）：
//  1. word_count : 提交词数 = challenge.Hops
//  2. char_position : 第 N 个词的第 1 / 末字符与 topic 首 / 末字符相关
//  3. ascii_sum : 所有词的 ASCII 之和 = 期望值（topic 派生）
//  4. total_chars : 总字符数在 [min, max] 区间
//  5. semantic : 每两个相邻词有共同字符（轻量语义代理）
//  6. timing : 提交时间 < TimeLimit
//
// 通过规则：6 项全过；任何一项失败 → 拒绝。
//
// 注：真正的语义判断应由 LLM 评估器完成（VLM 服务）。
// 本验证器提供"轻量"兜底（字符级 + 结构级校验），不替代 LLM。
package moltcaptcha

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrInvalidResponse 响应格式不合法。
var ErrInvalidResponse = errors.New("moltcaptcha: invalid response")

// ErrWordCountMismatch 词数不匹配。
var ErrWordCountMismatch = errors.New("moltcaptcha: word count mismatch")

// ErrTotalCharsOutOfRange 总字符数超界。
var ErrTotalCharsOutOfRange = errors.New("moltcaptcha: total chars out of range")

// ErrAsciiSumMismatch ASCII 和不匹配。
var ErrAsciiSumMismatch = errors.New("moltcaptcha: ascii sum mismatch")

// ErrCharPositionMismatch 字符位置不匹配。
var ErrCharPositionMismatch = errors.New("moltcaptcha: char position mismatch")

// ErrSemanticMismatch 语义链断裂。
var ErrSemanticMismatch = errors.New("moltcaptcha: semantic chain broken")

// ErrResponseTooLate 响应超时。
var ErrResponseTooLate = errors.New("moltcaptcha: response too late")

// =============================================================================
// 响应入参
// =============================================================================

// VerifyInput 验证入参。
type VerifyInput struct {
	// ChallengeID 关联的 challenge ID
	ChallengeID string
	// Words agent 提交的语义链（按顺序）
	Words []string
	// SubmittedAt 提交时间
	SubmittedAt time.Time
}

// VerifyOutput 验证出参。
type VerifyOutput struct {
	// Challenge 关联的 challenge
	Challenge *Challenge
	// ASCII 实际计算的 ASCII 和
	ASCII int64
	// TotalChars 总字符数
	TotalChars int
	// WordCount 词数
	WordCount int
	// Elapsed 用时
	Elapsed time.Duration
}

// =============================================================================
// Verifier
// =============================================================================

// Verifier 验证器配置。
type VerifierConfig struct {
	// Generator 关联的 challenge 生成器
	Generator *Generator
	// MinWordLen 每个词最小字符数（默认 2）
	MinWordLen int
	// MaxWordLen 每个词最大字符数（默认 30）
	MaxWordLen int
	// TotalCharsMin 总字符数下限（默认 hops * MinWordLen）
	TotalCharsMin int
	// TotalCharsMax 总字符数上限（默认 hops * MaxWordLen）
	TotalCharsMax int
	// ASCIIOffset ASCII 和的偏移量（用于将派生值映射到合理区间）
	ASCIIOffset int64
	// SemanticMinOverlap 语义相邻词最小共同字符数（默认 1）
	SemanticMinOverlap int
	// Clock 时间源
	Clock func() time.Time
}

// Verifier 验证器。
type Verifier struct {
	cfg VerifierConfig
}

// NewVerifier 构造验证器。
func NewVerifier(gen *Generator, cfg VerifierConfig) (*Verifier, error) {
	if gen == nil {
		return nil, errors.New("moltcaptcha: nil generator")
	}
	if cfg.MinWordLen <= 0 {
		cfg.MinWordLen = 2
	}
	if cfg.MaxWordLen <= 0 {
		cfg.MaxWordLen = 30
	}
	if cfg.SemanticMinOverlap <= 0 {
		cfg.SemanticMinOverlap = 1
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	cfg.Generator = gen
	return &Verifier{cfg: cfg}, nil
}

// Verify 验签 + 消费 challenge。
func (v *Verifier) Verify(ctx context.Context, in VerifyInput) (*VerifyOutput, error) {
	// 1. 基础校验
	if in.ChallengeID == "" {
		return nil, ErrChallengeNotFound
	}
	if len(in.Words) == 0 {
		return nil, fmt.Errorf("%w: empty words", ErrInvalidResponse)
	}

	// 2. 加载 + 消费 challenge
	c, err := v.cfg.Generator.ConsumeChallenge(ctx, in.ChallengeID)
	if err != nil {
		return nil, err
	}

	// 3. 时间窗口
	if in.SubmittedAt.IsZero() {
		in.SubmittedAt = v.cfg.Clock()
	}
	if c.IsExpired(in.SubmittedAt) {
		return nil, fmt.Errorf("%w: at %s", ErrResponseTooLate, in.SubmittedAt)
	}
	elapsed := in.SubmittedAt.Sub(c.IssuedAt)
	if elapsed > c.TimeLimit {
		return nil, fmt.Errorf("%w: elapsed=%v limit=%v", ErrResponseTooLate, elapsed, c.TimeLimit)
	}

	// 4. 词数 = hops
	if len(in.Words) != c.Hops {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrWordCountMismatch, len(in.Words), c.Hops)
	}

	// 5. 总字符数范围
	totalChars := 0
	for _, w := range in.Words {
		totalChars += len(strings.TrimSpace(w))
	}
	minChars := v.totalCharsMin(c)
	maxChars := v.totalCharsMax(c)
	if totalChars < minChars || totalChars > maxChars {
		return nil, fmt.Errorf("%w: got %d, want [%d, %d]",
			ErrTotalCharsOutOfRange, totalChars, minChars, maxChars)
	}

	// 6. ASCII 和
	var ascii int64
	normalized := make([]string, len(in.Words))
	for i, w := range in.Words {
		w = strings.TrimSpace(w)
		normalized[i] = w
		for _, c := range w {
			ascii += int64(c)
		}
	}
	expected := v.expectedASCIISum(c)
	if ascii != expected {
		return nil, fmt.Errorf("%w: got %d, want %d (offset=%d)",
			ErrAsciiSumMismatch, ascii, expected, v.cfg.ASCIIOffset)
	}

	// 7. 字符位置：第 1 词首字符 == topic 首字符；第 N 词末字符 == topic 末字符
	if !v.checkCharPositions(c, normalized) {
		return nil, ErrCharPositionMismatch
	}

	// 8. 语义链：每对相邻词有至少 N 个共同字符
	if !v.checkSemantic(c, normalized) {
		return nil, ErrSemanticMismatch
	}

	return &VerifyOutput{
		Challenge:  c,
		ASCII:      ascii,
		TotalChars: totalChars,
		WordCount:  len(in.Words),
		Elapsed:    elapsed,
	}, nil
}

// =============================================================================
// 内部规则
// =============================================================================

// totalCharsMin 总字符数下限。
func (v *Verifier) totalCharsMin(c *Challenge) int {
	if v.cfg.TotalCharsMin > 0 {
		return v.cfg.TotalCharsMin
	}
	return c.Hops * v.cfg.MinWordLen
}

// totalCharsMax 总字符数上限。
func (v *Verifier) totalCharsMax(c *Challenge) int {
	if v.cfg.TotalCharsMax > 0 {
		return v.cfg.TotalCharsMax
	}
	return c.Hops * v.cfg.MaxWordLen
}

// expectedASCIISum 期望的 ASCII 和。
//
// 算法：topic 字符 ASCII 之和 * hops + ASCIIOffset
func (v *Verifier) expectedASCIISum(c *Challenge) int64 {
	var topicSum int64
	for _, ch := range c.Topic {
		topicSum += int64(ch)
	}
	return topicSum*int64(c.Hops) + v.cfg.ASCIIOffset
}

// checkCharPositions 字符位置校验。
//
// 规则：
//   - 词[0] 首字符 == topic 首字符（不区分大小写）
//   - 词[Hops-1] 末字符 == topic 末字符（不区分大小写）
func (v *Verifier) checkCharPositions(c *Challenge, words []string) bool {
	if len(words) == 0 {
		return false
	}
	topic := strings.TrimSpace(c.Topic)
	if topic == "" {
		return false
	}
	runes := []rune(topic)
	first, last := runes[0], runes[len(runes)-1]

	// 第一个词首字符
	firstWord := []rune(strings.TrimSpace(words[0]))
	if len(firstWord) == 0 {
		return false
	}
	if !caseInsensitiveEqual(firstWord[0], first) {
		return false
	}

	// 最后一个词末字符
	lastWord := []rune(strings.TrimSpace(words[len(words)-1]))
	if len(lastWord) == 0 {
		return false
	}
	if !caseInsensitiveEqual(lastWord[len(lastWord)-1], last) {
		return false
	}
	return true
}

// checkSemantic 语义链校验（轻量代理）。
//
// 规则：每对相邻词有至少 SemanticMinOverlap 个共同字符（按字节集）。
// 真语义评估应由 LLM 完成；本函数仅做粗粒度兜底。
func (v *Verifier) checkSemantic(c *Challenge, words []string) bool {
	for i := 0; i+1 < len(words); i++ {
		if commonCharCount(words[i], words[i+1]) < v.cfg.SemanticMinOverlap {
			return false
		}
	}
	return true
}

// commonCharCount 两个词的共同字符数（按集合）。
func commonCharCount(a, b string) int {
	set := make(map[rune]bool)
	for _, r := range a {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			set[unicode.ToLower(r)] = true
		}
	}
	count := 0
	seen := make(map[rune]bool)
	for _, r := range b {
		lr := unicode.ToLower(r)
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		if set[lr] && !seen[lr] {
			count++
			seen[lr] = true
		}
	}
	return count
}

// caseInsensitiveEqual 比较两个 rune（忽略大小写）。
func caseInsensitiveEqual(a, b rune) bool {
	return unicode.ToLower(a) == unicode.ToLower(b)
}
