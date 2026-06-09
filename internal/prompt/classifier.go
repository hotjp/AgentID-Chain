// Package prompt 提供 AgentID-Chain 的 NL → 命令路由器（4th 接入范式）。
//
// 设计：
//   - 6 类 Intent：register / upgrade / query / batch / config / audit
//   - Classifier：基于关键词 + 顺序优先级匹配
//   - Slot Filling：从自然语言提取 owner / uuid / level / permission / reason
//   - Generator：把 Intent + Slots 转成 cobra 命令 args
//   - Validator：验证 slot 的合法性和必填
//
// 不依赖 LLM，纯 deterministic 规则（关键词 + 正则）；可作为 LLM 路由的
// fallback 或 baseline（P21 Agent Skills 会替换为 LLM-based）。
package prompt

// Intent 6 类意图枚举。
type Intent string

const (
	IntentRegister Intent = "register"
	IntentUpgrade  Intent = "upgrade"
	IntentQuery    Intent = "query"
	IntentBatch    Intent = "batch"
	IntentConfig   Intent = "config"
	IntentAudit    Intent = "audit"
	// IntentUnknown 兜底。
	IntentUnknown Intent = "unknown"
)

// AllIntents 全部有效意图（测试 + 校验用）。
var AllIntents = []Intent{
	IntentRegister, IntentUpgrade, IntentQuery,
	IntentBatch, IntentConfig, IntentAudit,
}

// IntentClassifier 把文本分类到一个 Intent。
type IntentClassifier interface {
	Classify(text string) Intent
	// Confidence 返回 (intent, score ∈ [0, 1])。
	Confidence(text string) (Intent, float64)
}

// KeywordClassifier 关键词 + 顺序优先级的确定性分类器。
type KeywordClassifier struct {
	// rules 按优先级排序的关键词规则（先匹配优先）。
	rules []rule
}

type rule struct {
	intent  Intent
	keywords []string
	// antiKeywords 出现则降权（不绝对屏蔽）。
	antiKeywords []string
	// weight 基础权重。
	weight float64
}

// NewKeywordClassifier 构造默认分类器。
func NewKeywordClassifier() *KeywordClassifier {
	c := &KeywordClassifier{}
	// 注意：把更具体的 intent 放在前面（batch 在 register 前，避免 "batch register" 误判）。
	c.rules = []rule{
		{
			intent: IntentBatch,
			keywords: []string{
				"batch", "bulk", "many agents", "multiple agents", "csv", "批量", "多个 agent",
			},
			weight: 1.0,
		},
		{
			intent: IntentConfig,
			keywords: []string{
				"config set", "config show", "config reset", "configure", "configuration",
				"配置", "设置",
			},
			weight: 1.0,
		},
		{
			intent: IntentAudit,
			keywords: []string{
				"audit", "logs", "history", "changes", "log of", "audit logs", "审计", "变更记录",
			},
			weight: 0.9,
		},
		{
			intent: IntentRegister,
			keywords: []string{
				"register", "create agent", "new agent", "enroll", "onboard",
			},
			weight: 1.0,
		},
		{
			intent: IntentUpgrade,
			keywords: []string{
				"upgrade", "promote", "elevate", "increase level", "提升",
			},
			weight: 1.0,
		},
		{
			intent: IntentQuery,
			keywords: []string{
				"info", "show", "get info", "find", "look up", "查询", "详情", "获取",
			},
			weight: 0.8,
		},
	}
	return c
}

// Classify 分类。
func (c *KeywordClassifier) Classify(text string) Intent {
	intent, _ := c.Confidence(text)
	return intent
}

// Confidence 返回最佳匹配 + 得分（[0, 1]）。
func (c *KeywordClassifier) Confidence(text string) (Intent, float64) {
	low := lower(text)
	best := IntentUnknown
	bestScore := 0.0
	for _, r := range c.rules {
		score := 0.0
		for _, kw := range r.keywords {
			if contains(low, kw) {
				score += r.weight
			}
		}
		// 每次匹配上限 1.0
		if score > 1.0 {
			score = 1.0
		}
		if score > bestScore {
			bestScore = score
			best = r.intent
		}
	}
	if bestScore < 0.1 {
		return IntentUnknown, 0
	}
	return best, bestScore
}

func lower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}

func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	return indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	if m == 0 {
		return 0
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}
