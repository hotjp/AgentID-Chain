package moltcaptcha

import (
	"testing"
)

func TestNewMatcher_Default(t *testing.T) {
	m := NewMatcher()
	if !m.Loaded() {
		t.Error("should be loaded")
	}
	if m.Size() < 10 {
		t.Errorf("default should have at least 10 topics, got %d", m.Size())
	}
}

func TestNewMatcherFromMap(t *testing.T) {
	m := NewMatcherFromMap(map[string][]string{
		"ai": {"machine", "model", "neural"},
	})
	if m.Size() != 1 {
		t.Errorf("Size = %d", m.Size())
	}
}

func TestNewMatcherFromMap_Empty(t *testing.T) {
	m := NewMatcherFromMap(nil)
	if !m.Loaded() {
		t.Error("constructor was called, Loaded should be true even with empty data")
	}
	if m.Size() != 0 {
		t.Errorf("Size = %d", m.Size())
	}
}

func TestMatcher_Load(t *testing.T) {
	m := NewMatcher()
	m.Load(map[string][]string{
		"new": {"foo", "bar"},
	})
	if m.Size() != 1 {
		t.Errorf("Size = %d", m.Size())
	}
}

func TestMatcher_KeywordsFor(t *testing.T) {
	m := NewMatcher()
	kw := m.KeywordsFor("verification")
	if len(kw) == 0 {
		t.Error("expected keywords for verification")
	}
}

func TestMatcher_KeywordsFor_Unknown(t *testing.T) {
	m := NewMatcher()
	kw := m.KeywordsFor("nonexistent-topic-xyz")
	if kw != nil {
		t.Errorf("expected nil, got %v", kw)
	}
}

func TestMatcher_KeywordsFor_CaseInsensitive(t *testing.T) {
	m := NewMatcher()
	kw1 := m.KeywordsFor("Verification")
	kw2 := m.KeywordsFor("VERIFICATION")
	kw3 := m.KeywordsFor("verification")
	if len(kw1) == 0 || len(kw2) == 0 || len(kw3) == 0 {
		t.Error("all case variants should match")
	}
}

func TestMatcher_KeywordsFor_ReturnsCopy(t *testing.T) {
	m := NewMatcher()
	kw1 := m.KeywordsFor("verification")
	kw1[0] = "MUTATED"
	kw2 := m.KeywordsFor("verification")
	if kw2[0] == "MUTATED" {
		t.Error("should return a copy, not a reference")
	}
}

func TestMatcher_Topics(t *testing.T) {
	m := NewMatcher()
	topics := m.Topics()
	if len(topics) != m.Size() {
		t.Errorf("topics len = %d, size = %d", len(topics), m.Size())
	}
}

func TestContainsAny_Hit(t *testing.T) {
	m := NewMatcher()
	matched, kw := m.ContainsAny([]string{"please verify this"}, "verification")
	if !matched {
		t.Error("should match 'verify' in 'please verify this'")
	}
	if kw == "" {
		t.Error("expected non-empty keyword")
	}
}

func TestContainsAny_Miss(t *testing.T) {
	m := NewMatcher()
	matched, _ := m.ContainsAny([]string{"unrelated text"}, "verification")
	if matched {
		t.Error("should not match")
	}
}

func TestContainsAny_UnknownTopic(t *testing.T) {
	m := NewMatcher()
	matched, kw := m.ContainsAny([]string{"any"}, "unknown-topic-xyz")
	if matched {
		t.Error("should not match unknown topic")
	}
	if kw != "" {
		t.Errorf("kw should be empty, got %q", kw)
	}
}

func TestContainsAny_CaseInsensitive(t *testing.T) {
	m := NewMatcher()
	matched, _ := m.ContainsAny([]string{"AUTH and verify"}, "verification")
	if !matched {
		t.Error("should match case-insensitive")
	}
}

func TestContainsAny_SubstringMatch(t *testing.T) {
	m := NewMatcher()
	// "cipher" 在 "encryption" 关键词中；"encryptor" 应包含 "encrypt"
	matched, _ := m.ContainsAny([]string{"encryptor"}, "encryption")
	if !matched {
		t.Error("should substring match")
	}
}

func TestContainsAll_OK(t *testing.T) {
	m := NewMatcherFromMap(map[string][]string{
		"test": {"foo", "bar", "baz"},
	})
	if !m.ContainsAll([]string{"foo bar baz qux"}, "test") {
		t.Error("should match all")
	}
}

func TestContainsAll_Miss(t *testing.T) {
	m := NewMatcherFromMap(map[string][]string{
		"test": {"foo", "bar", "baz"},
	})
	if m.ContainsAll([]string{"foo bar"}, "test") {
		t.Error("should miss baz")
	}
}

func TestContainsAll_Unknown(t *testing.T) {
	m := NewMatcher()
	if m.ContainsAll([]string{"foo"}, "unknown-topic-xyz") {
		t.Error("unknown topic should not match")
	}
}

func TestMatchCount(t *testing.T) {
	m := NewMatcherFromMap(map[string][]string{
		"test": {"foo", "bar", "baz"},
	})
	c := m.MatchCount([]string{"foo qux"}, "test")
	if c != 1 {
		t.Errorf("count = %d, want 1", c)
	}
	c = m.MatchCount([]string{"foo bar baz"}, "test")
	if c != 3 {
		t.Errorf("count = %d, want 3", c)
	}
}

func TestMatchCount_Unknown(t *testing.T) {
	m := NewMatcher()
	if got := m.MatchCount([]string{"foo"}, "unknown"); got != 0 {
		t.Errorf("unknown: count = %d, want 0", got)
	}
}

func TestMatchedKeywords(t *testing.T) {
	m := NewMatcherFromMap(map[string][]string{
		"test": {"foo", "bar", "baz"},
	})
	got := m.MatchedKeywords([]string{"foo qux"}, "test")
	if len(got) != 1 || got[0] != "foo" {
		t.Errorf("got %v", got)
	}
}

func TestMatchedKeywords_None(t *testing.T) {
	m := NewMatcherFromMap(map[string][]string{
		"test": {"foo", "bar"},
	})
	got := m.MatchedKeywords([]string{"xyz"}, "test")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestMatchedKeywords_Unknown(t *testing.T) {
	m := NewMatcher()
	got := m.MatchedKeywords([]string{"foo"}, "unknown")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestCloneKeywords(t *testing.T) {
	src := map[string][]string{
		"a": {"1", "2"},
		"b": {"3"},
	}
	clone := cloneKeywords(src)
	if len(clone) != 2 {
		t.Errorf("len = %d", len(clone))
	}
	// 修改 clone 不影响 src
	clone["a"][0] = "X"
	if src["a"][0] == "X" {
		t.Error("clone is not deep")
	}
}

func TestCloneKeywords_Nil(t *testing.T) {
	clone := cloneKeywords(nil)
	if clone == nil || len(clone) != 0 {
		t.Error("clone of nil should be empty map")
	}
}

func TestDefaultKeywords_AllTopicsValid(t *testing.T) {
	// 默认表里所有 topic 都是字符串
	for topic, kws := range defaultKeywords {
		if topic == "" {
			t.Error("empty topic in default")
		}
		if len(kws) == 0 {
			t.Errorf("topic %q has no keywords", topic)
		}
	}
}
