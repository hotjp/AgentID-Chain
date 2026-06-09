// Package prompt 错误定义 + 验证器。
package prompt

import "errors"

var (
	// ErrUnknownIntent 未识别意图。
	ErrUnknownIntent = errors.New("prompt: unknown intent")
	// ErrMissingOwner 缺 owner 槽位。
	ErrMissingOwner = errors.New("prompt: missing owner slot")
	// ErrMissingUUID 缺 uuid 槽位。
	ErrMissingUUID = errors.New("prompt: missing uuid slot")
	// ErrMissingPath 缺文件路径槽位。
	ErrMissingPath = errors.New("prompt: missing file path slot")
	// ErrMissingConfigKey 缺 config key 槽位。
	ErrMissingConfigKey = errors.New("prompt: missing config key slot")
)

// Validator 槽位验证。
type Validator interface {
	Validate(intent Intent, slots Slots) error
}

// DefaultValidator 默认验证器（按 Intent 检查必填槽位）。
type DefaultValidator struct{}

// NewDefaultValidator 构造。
func NewDefaultValidator() *DefaultValidator { return &DefaultValidator{} }

// Validate 按 intent 检查必填槽位。
func (v *DefaultValidator) Validate(intent Intent, slots Slots) error {
	switch intent {
	case IntentRegister:
		// owner 槽位 OR name 槽位（generator 会把 name 转成 did:）
		if !slots.Has(SlotOwner) && !slots.Has(SlotName) {
			return ErrMissingOwner
		}
	case IntentUpgrade, IntentQuery, IntentAudit:
		if !slots.Has(SlotUUID) {
			return ErrMissingUUID
		}
	case IntentBatch:
		if !slots.Has(SlotPath) {
			return ErrMissingPath
		}
	case IntentConfig:
		// 可走 show 分支
	case IntentUnknown:
		return ErrUnknownIntent
	}
	return nil
}

// =============================================================================
// Pipeline：组合 Classifier + SlotFiller + Validator + Generator
// =============================================================================

// Pipeline 一次完成 classify → fill → validate → generate。
type Pipeline struct {
	Classifier IntentClassifier
	Filler     *SlotFiller
	Validator  Validator
	Generator  Generator
}

// NewPipeline 构造默认 pipeline。
func NewPipeline() *Pipeline {
	return &Pipeline{
		Classifier: NewKeywordClassifier(),
		Filler:     NewSlotFiller(),
		Validator:  NewDefaultValidator(),
		Generator:  NewDefaultGenerator(),
	}
}

// Result 解析结果。
type Result struct {
	Intent  Intent
	Slots   Slots
	Cmd     string
	Args    []string
	Score   float64
	Skipped string // 被 validator 跳过的原因
}

// Process 一站式处理文本。
func (p *Pipeline) Process(text string) (*Result, error) {
	intent, score := p.Classifier.Confidence(text)
	if intent == IntentUnknown {
		return nil, ErrUnknownIntent
	}
	slots := p.Filler.Fill(text)
	if err := p.Validator.Validate(intent, slots); err != nil {
		return &Result{Intent: intent, Slots: slots, Score: score, Skipped: err.Error()}, err
	}
	cmd, args, err := p.Generator.Generate(intent, slots)
	if err != nil {
		return &Result{Intent: intent, Slots: slots, Score: score, Skipped: err.Error()}, err
	}
	return &Result{Intent: intent, Slots: slots, Cmd: cmd, Args: args, Score: score}, nil
}
