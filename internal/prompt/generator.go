// Package prompt Config generator — 把 Intent + Slots 转换成 cobra 命令 args。
package prompt

// Generator 把解析结果生成 CLI 命令。
type Generator interface {
	// Generate 返回 cobra 子命令名 + args。
	Generate(intent Intent, slots Slots) (cmd string, args []string, err error)
}

// DefaultGenerator 默认实现。
type DefaultGenerator struct{}

// NewDefaultGenerator 构造。
func NewDefaultGenerator() *DefaultGenerator { return &DefaultGenerator{} }

// Generate 路由到具体的 intent 实现。
func (g *DefaultGenerator) Generate(intent Intent, slots Slots) (string, []string, error) {
	switch intent {
	case IntentRegister:
		return genRegister(slots)
	case IntentUpgrade:
		return genUpgrade(slots)
	case IntentQuery:
		return genQuery(slots)
	case IntentBatch:
		return genBatch(slots)
	case IntentConfig:
		return genConfig(slots)
	case IntentAudit:
		return genAudit(slots)
	default:
		return "", nil, ErrUnknownIntent
	}
}

// =============================================================================
// 各 Intent 的生成函数
// =============================================================================

func genRegister(s Slots) (string, []string, error) {
	owner := s.Get(SlotOwner, "")
	if owner == "" {
		if name := s.Get(SlotName, ""); name != "" {
			owner = "did:agentid:" + name
		}
	}
	if owner == "" {
		return "", nil, ErrMissingOwner
	}
	args := []string{"--owner", owner}
	if lvl := s.Get(SlotLevel, ""); lvl != "" {
		args = append(args, "--level", lvl)
	}
	if perm := s.Get(SlotPermission, ""); perm != "" {
		args = append(args, "--permission", perm)
	}
	pk := s.Get(SlotPublicKey, "")
	if pk == "" {
		// prompt 路径下允许自动补默认（NL 不会总指定 PK）
		pk = "pk_default"
	}
	args = append(args, "--public-key", pk)
	return "register", args, nil
}

func genUpgrade(s Slots) (string, []string, error) {
	uuid := s.Get(SlotUUID, "")
	if uuid == "" {
		return "", nil, ErrMissingUUID
	}
	args := []string{"--uuid", uuid}
	if lvl := s.Get(SlotLevel, ""); lvl != "" {
		args = append(args, "--target-level", lvl)
	} else {
		args = append(args, "--target-level", "2")
	}
	if reason := s.Get(SlotReason, ""); reason != "" {
		args = append(args, "--reason", reason)
	}
	return "upgrade", args, nil
}

func genQuery(s Slots) (string, []string, error) {
	uuid := s.Get(SlotUUID, "")
	if uuid == "" {
		return "", nil, ErrMissingUUID
	}
	args := []string{"--uuid", uuid}
	if f := s.Get(SlotFormat, ""); f != "" {
		args = append(args, "--format", f)
	}
	return "info", args, nil
}

func genBatch(s Slots) (string, []string, error) {
	path := s.Get(SlotPath, "")
	if path == "" {
		return "", nil, ErrMissingPath
	}
	args := []string{"--file", path}
	if s.Has(SlotFormat) {
		args = append(args, "--format", s.Get(SlotFormat, "json"))
	}
	return "batch-register", args, nil
}

func genConfig(s Slots) (string, []string, error) {
	key := s.Get(SlotConfigKey, "")
	val := s.Get(SlotConfigVal, "")
	if key == "" {
		return "", nil, ErrMissingConfigKey
	}
	if val == "" {
		// 退化为 show
		return "config", []string{"show"}, nil
	}
	return "config", []string{"set", key, val}, nil
}

func genAudit(s Slots) (string, []string, error) {
	uuid := s.Get(SlotUUID, "")
	if uuid == "" {
		return "", nil, ErrMissingUUID
	}
	args := []string{"--uuid", uuid}
	if limit := s.Get(SlotLimit, ""); limit != "" {
		args = append(args, "--limit", limit)
	}
	return "audit", args, nil
}
