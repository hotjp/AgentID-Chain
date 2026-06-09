// Package moltcaptcha 关键词语义匹配（不依赖 LLM）。
//
// 业务场景：moltcaptcha 验证时，需要判断 agent 提交的 answer 是否与 topic 语义相关。
// 完整方案是 LLM 评估（VLM 服务），但 v2.0.1 阶段先做关键词兜底：
//   - 维护 TopicKeywords map：topic → 关联关键词列表
//   - 校验时调用 ContainsAny(answer, topic) → answer 包含任一关键词？
//
// 优点：
//   - 0 第三方依赖
//   - O(1) 查找 + 字符串子串扫描
//   - 误判可控（关键词列表是业务可调的白名单）
//
// 缺点：
//   - 覆盖率依赖关键词列表
//   - 无法处理同义词 / 语义推理
//
// 升级路径：未来可对接 embedding 服务（但保持接口不变）。
package moltcaptcha

import (
	"strings"
	"sync"
)

// =============================================================================
// 默认关键词表
// =============================================================================

// defaultKeywords 默认 topic → 关键词映射。
//
// 业务可注入自定义 map 覆盖；key 不区分大小写。
var defaultKeywords = map[string][]string{
	"verification":     {"auth", "authenticate", "check", "proof", "validate", "verify", "confirm", "attest", "credential"},
	"authenticity":     {"genuine", "real", "original", "legitimate", "trusted", "true", "sincere", "actual", "provenance"},
	"digital trust":    {"trust", "secure", "reliable", "confident", "dependable", "warranty", "guarantee", "faith", "integrity"},
	"cryptography":     {"cipher", "encrypt", "decrypt", "key", "hash", "signature", "secret", "code", "algorithm"},
	"identity":         {"identifier", "self", "persona", "subject", "principal", "claim", "attestation", "did", "uuid"},
	"algorithms":       {"procedure", "method", "routine", "process", "step", "logic", "function", "recipe", "computation"},
	"neural networks":  {"neuron", "layer", "perceptron", "activation", "weight", "backprop", "deep", "model", "tensor"},
	"computation":      {"compute", "calculate", "process", "execute", "evaluate", "operate", "derive", "solve", "run"},
	"binary":           {"bit", "byte", "zero", "one", "true", "false", "boolean", "base-2", "digital"},
	"protocols":        {"handshake", "exchange", "negotiation", "agreement", "standard", "specification", "rule", "format"},
	"encryption":       {"cipher", "secret", "key", "lock", "scramble", "obfuscate", "conceal", "protect", "crypt"},
	"tokens":           {"credential", "passport", "badge", "marker", "symbol", "voucher", "coupon", "chip", "nonce"},
	"agents":           {"actor", "entity", "delegate", "proxy", "representative", "operator", "executor", "doer", "broker"},
	"automation":       {"script", "bot", "machine", "robotic", "automatic", "scheduled", "triggered", "programmed", "systematic"},
	"circuits":         {"gate", "wire", "logic", "chip", "board", "electrical", "path", "loop", "transistor"},
	"logic gates":      {"and", "or", "not", "nand", "nor", "xor", "xnor", "boolean", "truth-table"},
	"recursion":        {"self-reference", "loop", "iterate", "recurse", "base-case", "induction", "stack", "tree", "nested"},
	"entropy":          {"randomness", "uncertainty", "chaos", "disorder", "information", "bits", "noise", "spread", "variance"},
	"hashing":          {"digest", "fingerprint", "checksum", "sha", "md5", "map", "bucket", "scatter", "transform"},
	"signatures":       {"seal", "mark", "sign", "endorsement", "authorization", "approval", "handwriting", "credential", "proof"},
}

// =============================================================================
// Matcher 关键词匹配器
// =============================================================================

// Matcher 关键词匹配器。
//
// 线程安全：所有操作加锁（读写锁）。
type Matcher struct {
	mu     sync.RWMutex
	words  map[string][]string // topic -> keywords
	loaded bool
}

// NewMatcher 返回默认匹配器。
func NewMatcher() *Matcher {
	return &Matcher{words: cloneKeywords(defaultKeywords), loaded: true}
}

// NewMatcherFromMap 用给定 map 构造匹配器。
func NewMatcherFromMap(m map[string][]string) *Matcher {
	return &Matcher{words: cloneKeywords(m), loaded: true}
}

// Load 加载 / 替换关键词表。
func (m *Matcher) Load(words map[string][]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.words = cloneKeywords(words)
	m.loaded = true
}

// Loaded 是否已加载。
func (m *Matcher) Loaded() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loaded
}

// Size 返回关键词表大小。
func (m *Matcher) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.words)
}

// KeywordsFor 返回 topic 的关键词列表（用于调试 / 审计）。
func (m *Matcher) KeywordsFor(topic string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k, ok := m.words[strings.ToLower(strings.TrimSpace(topic))]
	if !ok {
		return nil
	}
	out := make([]string, len(k))
	copy(out, k)
	return out
}

// Topics 返回所有已注册 topic。
func (m *Matcher) Topics() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.words))
	for t := range m.words {
		out = append(out, t)
	}
	return out
}

// =============================================================================
// 匹配方法
// =============================================================================

// ContainsAny 校验 answer 中是否包含 topic 的任一关键词（不区分大小写、子串匹配）。
//
// 返回 (matched, firstKeyword)：
//   - matched: 是否匹配
//   - firstKeyword: 第一个匹配到的关键词（无匹配时为空）
func (m *Matcher) ContainsAny(answer []string, topic string) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kw, ok := m.words[strings.ToLower(strings.TrimSpace(topic))]
	if !ok {
		return false, ""
	}
	for _, w := range answer {
		lower := strings.ToLower(w)
		for _, k := range kw {
			if strings.Contains(lower, k) {
				return true, k
			}
		}
	}
	return false, ""
}

// ContainsAll 校验 answer 中是否包含 topic 的所有关键词。
//
// 业务用途：更严格的语义验证（如 extreme 难度）。
func (m *Matcher) ContainsAll(answer []string, topic string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kw, ok := m.words[strings.ToLower(strings.TrimSpace(topic))]
	if !ok {
		return false
	}
	combined := strings.ToLower(strings.Join(answer, " "))
	for _, k := range kw {
		if !strings.Contains(combined, k) {
			return false
		}
	}
	return true
}

// MatchCount 返回 answer 中匹配到 topic 关键词的次数。
func (m *Matcher) MatchCount(answer []string, topic string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kw, ok := m.words[strings.ToLower(strings.TrimSpace(topic))]
	if !ok {
		return 0
	}
	combined := strings.ToLower(strings.Join(answer, " "))
	count := 0
	for _, k := range kw {
		if strings.Contains(combined, k) {
			count++
		}
	}
	return count
}

// MatchedKeywords 返回 answer 中匹配到的所有关键词。
func (m *Matcher) MatchedKeywords(answer []string, topic string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kw, ok := m.words[strings.ToLower(strings.TrimSpace(topic))]
	if !ok {
		return nil
	}
	combined := strings.ToLower(strings.Join(answer, " "))
	var out []string
	for _, k := range kw {
		if strings.Contains(combined, k) {
			out = append(out, k)
		}
	}
	return out
}

// =============================================================================
// 工具函数
// =============================================================================

// cloneKeywords 深拷贝关键词表。
func cloneKeywords(src map[string][]string) map[string][]string {
	out := make(map[string][]string, len(src))
	for k, v := range src {
		dup := make([]string, len(v))
		copy(dup, v)
		out[k] = dup
	}
	return out
}
