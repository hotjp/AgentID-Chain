// Package prompt Slot 填充（从 NL 提取结构化参数）。
package prompt

import (
	"regexp"
	"strconv"
	"strings"
)

// Slot 名。
const (
	SlotOwner      = "owner"
	SlotUUID       = "uuid"
	SlotLevel      = "level"
	SlotPermission = "permission"
	SlotReason     = "reason"
	SlotPublicKey  = "public_key"
	SlotName       = "name"
	SlotPath       = "path"
	SlotConfigKey  = "config_key"
	SlotConfigVal  = "config_value"
	SlotLimit      = "limit"
	SlotFormat     = "format"
)

// Slots 槽位（key → value）。
type Slots map[string]string

// Has 槽位是否非空。
func (s Slots) Has(key string) bool {
	v, ok := s[key]
	return ok && v != ""
}

// Get 拿槽位（带默认值）。
func (s Slots) Get(key, def string) string {
	if v, ok := s[key]; ok {
		return v
	}
	return def
}

// SlotFiller 槽位填充器。
type SlotFiller struct {
	// extractors 按槽位注册的提取器。
	extractors map[string]Extractor
}

// Extractor 从文本提取槽位值（返回 "" 表示未提取到）。
type Extractor func(text string) string

// NewSlotFiller 构造默认填充器。
func NewSlotFiller() *SlotFiller {
	f := &SlotFiller{extractors: make(map[string]Extractor)}
	f.extractors[SlotUUID] = extractUUID
	f.extractors[SlotOwner] = extractOwner
	f.extractors[SlotLevel] = extractLevel
	f.extractors[SlotPermission] = extractPermission
	f.extractors[SlotReason] = extractReason
	f.extractors[SlotPublicKey] = extractPublicKey
	f.extractors[SlotName] = extractName
	f.extractors[SlotPath] = extractPath
	f.extractors[SlotConfigKey] = extractConfigKey
	f.extractors[SlotConfigVal] = extractConfigValue
	f.extractors[SlotLimit] = extractLimit
	f.extractors[SlotFormat] = extractFormat
	return f
}

// Fill 从 text 提取所有已注册槽位。
func (f *SlotFiller) Fill(text string) Slots {
	out := make(Slots, len(f.extractors))
	for k, ex := range f.extractors {
		if v := ex(text); v != "" {
			out[k] = v
		}
	}
	return out
}

// Register 注册自定义提取器（覆盖默认）。
func (f *SlotFiller) Register(name string, ex Extractor) {
	f.extractors[name] = ex
}

// =============================================================================
// 提取器实现（独立函数便于复用 + 测试）
// =============================================================================

var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func extractUUID(text string) string {
	return uuidRe.FindString(text)
}

var didRe = regexp.MustCompile(`did:agentid:[A-Za-z0-9_\-]+`)

func extractOwner(text string) string {
	return didRe.FindString(text)
}

var levelRe = regexp.MustCompile(`(?i)level\s+(\d+)`)

func extractLevel(text string) string {
	m := levelRe.FindStringSubmatch(text)
	if len(m) != 2 {
		return ""
	}
	if _, err := strconv.ParseUint(m[1], 10, 8); err != nil {
		return ""
	}
	return m[1]
}

var permRe = regexp.MustCompile(`(?i)permission[s]?\s+(0x[0-9a-fA-F]+|\d+)`)

func extractPermission(text string) string {
	m := permRe.FindStringSubmatch(text)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

var reasonRe = regexp.MustCompile(`(?i)(?:for|because|reason)\s+([^,\.\n]+?)(?:\s+(?:on|at|by|with|using)\s|$|[,\.\n]|$)`)

func extractReason(text string) string {
	m := reasonRe.FindStringSubmatch(text)
	if len(m) != 2 {
		return ""
	}
	// trim 标点
	return strings.TrimSpace(strings.Trim(m[1], " \t\n,.\"'`"))
}

var pkRe = regexp.MustCompile(`(?i)(?:public[_\s]?key|pk)\s*[=:]\s*([A-Za-z0-9+/=_\-]{8,})`)

func extractPublicKey(text string) string {
	m := pkRe.FindStringSubmatch(text)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

// nameRe 名字提取：匹配 "for X" / "named X" / "called X"。
//
// 关键：不要匹配 "agent X" 这种通用词序（"agent for ..." 会把 "for" 当名字）。
// 用 word boundary + 显式介词/动词。
var nameRe = regexp.MustCompile(`(?i)(?:^|[\s,])(?:for|named|called)\s+([a-zA-Z][a-zA-Z0-9_\-]{1,31})`)

func extractName(text string) string {
	m := nameRe.FindStringSubmatch(text)
	if len(m) != 2 {
		return ""
	}
	w := m[1]
	switch strings.ToLower(w) {
	case "the", "a", "an", "agent", "new", "this", "that", "for", "to", "at", "on", "by", "with":
		return ""
	}
	return w
}

var pathRe = regexp.MustCompile(`(?:\.{0,2}/)?(?:[\w\-]+\/)*[\w\-]+\.[a-zA-Z0-9]+|~\/[\w\-./]+|\/[\w\-./]+`)

func extractPath(text string) string {
	return pathRe.FindString(text)
}

var configKeyRe = regexp.MustCompile(`(?i)config(?:\s+set)?\s+([a-z][a-z0-9_\-]+)`)

func extractConfigKey(text string) string {
	m := configKeyRe.FindStringSubmatch(text)
	if len(m) != 2 {
		return ""
	}
	return strings.ToLower(m[1])
}

var configValRe = regexp.MustCompile(`(?i)(?:set|to|=)\s+[a-z][a-z0-9_\-]+\s+(\S+)$`)

func extractConfigValue(text string) string {
	m := configValRe.FindStringSubmatch(strings.TrimSpace(text))
	if len(m) != 2 {
		// fallback: last whitespace-delimited token
		parts := strings.Fields(text)
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return ""
	}
	return strings.Trim(m[1], " \t\n,.\"'`")
}

var limitRe = regexp.MustCompile(`(?i)limit\s+(\d+)|last\s+(\d+)|top\s+(\d+)`)

func extractLimit(text string) string {
	m := limitRe.FindStringSubmatch(text)
	for i := 1; i < len(m); i++ {
		if m[i] != "" {
			return m[i]
		}
	}
	return ""
}

var formatRe = regexp.MustCompile(`(?i)(?:as|format|in)\s+(json|yaml|table|csv)`)

func extractFormat(text string) string {
	m := formatRe.FindStringSubmatch(text)
	if len(m) != 2 {
		return ""
	}
	return strings.ToLower(m[1])
}
