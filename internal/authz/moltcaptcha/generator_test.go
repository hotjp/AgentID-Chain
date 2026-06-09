package moltcaptcha

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/alicebob/miniredis/v2"
)

var defaultTopics = []string{
	"verification", "authenticity", "cryptography", "identity",
	"algorithms", "neural networks", "tokens", "protocols",
}

func newTestGenerator(t *testing.T) *Generator {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	g, err := NewGenerator(GeneratorConfig{
		Cache:            cache.NewMiniredis(mr),
		TopicPool:        defaultTopics,
		DefaultDifficulty: DifficultyMedium,
		DefaultTTL:       30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestDifficulty_Hops(t *testing.T) {
	cases := []struct {
		d    Difficulty
		hops int
		tl   time.Duration
	}{
		{DifficultyEasy, 1, 30 * time.Second},
		{DifficultyMedium, 2, 20 * time.Second},
		{DifficultyHard, 3, 15 * time.Second},
		{DifficultyExtreme, 4, 10 * time.Second},
		{Difficulty("invalid"), 0, 0},
	}
	for _, c := range cases {
		if c.d.Hops() != c.hops {
			t.Errorf("%s hops = %d, want %d", c.d, c.d.Hops(), c.hops)
		}
		if c.d.TimeLimit() != c.tl {
			t.Errorf("%s time limit = %v, want %v", c.d, c.d.TimeLimit(), c.tl)
		}
	}
}

func TestDifficulty_IsValid(t *testing.T) {
	for _, d := range AllDifficulties() {
		if !d.IsValid() {
			t.Errorf("%s should be valid", d)
		}
	}
	if Difficulty("invalid").IsValid() {
		t.Error("invalid should not be valid")
	}
}

func TestDifficulty_AllDifficulties(t *testing.T) {
	if len(AllDifficulties()) != 4 {
		t.Errorf("expected 4 difficulties, got %d", len(AllDifficulties()))
	}
}

func TestParseDifficulty(t *testing.T) {
	cases := []struct {
		in   string
		want Difficulty
		ok   bool
	}{
		{"easy", DifficultyEasy, true},
		{"EASY", DifficultyEasy, true},
		{"Medium", DifficultyMedium, true},
		{"hard", DifficultyHard, true},
		{"extreme", DifficultyExtreme, true},
		{"invalid", "", false},
		{"", "", false},
		{"easy ", DifficultyEasy, true}, // trim
	}
	for _, c := range cases {
		got, err := ParseDifficulty(c.in)
		if c.ok {
			if err != nil {
				t.Errorf("ParseDifficulty(%q) err = %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseDifficulty(%q) = %s, want %s", c.in, got, c.want)
			}
		} else {
			if err == nil {
				t.Errorf("ParseDifficulty(%q) expected error", c.in)
			}
		}
	}
}

func TestNewGenerator_NilCache(t *testing.T) {
	_, err := NewGenerator(GeneratorConfig{TopicPool: defaultTopics})
	if !errors.Is(err, ErrStoreUnavailable) {
		t.Errorf("err = %v, want ErrStoreUnavailable", err)
	}
}

func TestNewGenerator_EmptyTopicPool(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	_, err := NewGenerator(GeneratorConfig{Cache: cache.NewMiniredis(mr)})
	if !errors.Is(err, ErrEmptyTopicPool) {
		t.Errorf("err = %v, want ErrEmptyTopicPool", err)
	}
}

func TestNewGenerator_DefaultValues(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	g, err := NewGenerator(GeneratorConfig{
		Cache:     cache.NewMiniredis(mr),
		TopicPool: defaultTopics,
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.cfg.DefaultDifficulty != DifficultyMedium {
		t.Errorf("default difficulty = %s", g.cfg.DefaultDifficulty)
	}
	if g.cfg.DefaultTTL != 30*time.Second {
		t.Errorf("default TTL = %v", g.cfg.DefaultTTL)
	}
}

func TestNewGenerator_InvalidDefaultDifficulty(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	_, err := NewGenerator(GeneratorConfig{
		Cache:            cache.NewMiniredis(mr),
		TopicPool:        defaultTopics,
		DefaultDifficulty: Difficulty("invalid"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewGenerator_CustomPrompts(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	custom := map[Difficulty]string{
		DifficultyEasy: "custom {topic}",
	}
	g, err := NewGenerator(GeneratorConfig{
		Cache:          cache.NewMiniredis(mr),
		TopicPool:      defaultTopics,
		PromptTemplates: custom,
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.cfg.PromptTemplates[DifficultyEasy] != "custom {topic}" {
		t.Error("custom prompt not stored")
	}
}

func TestGenerate_HappyPath(t *testing.T) {
	g := newTestGenerator(t)
	c, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.ChallengeID == "" {
		t.Error("empty challenge id")
	}
	if c.Topic == "" {
		t.Error("empty topic")
	}
	if c.Difficulty != DifficultyMedium {
		t.Errorf("difficulty = %s, want medium", c.Difficulty)
	}
	if c.Hops != 2 {
		t.Errorf("hops = %d, want 2", c.Hops)
	}
	if c.TimeLimit != 20*time.Second {
		t.Errorf("time limit = %v, want 20s", c.TimeLimit)
	}
	if len(c.Hints) == 0 {
		t.Error("empty hints")
	}
	if c.PromptTemplate == "" {
		t.Error("empty prompt template")
	}
	if c.ExpiresAt.Before(c.IssuedAt) {
		t.Error("expires before issued")
	}
}

func TestGenerate_TopicFromPool(t *testing.T) {
	g := newTestGenerator(t)
	for i := 0; i < 50; i++ {
		c, err := g.Generate(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, t := range defaultTopics {
			if c.Topic == t {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("topic %q not in pool", c.Topic)
		}
	}
}

func TestGenerate_UniqueIDs(t *testing.T) {
	g := newTestGenerator(t)
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		c, _ := g.Generate(context.Background())
		if seen[c.ChallengeID] {
			t.Fatalf("duplicate id: %s", c.ChallengeID)
		}
		seen[c.ChallengeID] = true
	}
}

func TestGenerateWithDifficulty(t *testing.T) {
	g := newTestGenerator(t)
	for _, d := range AllDifficulties() {
		c, err := g.GenerateWithDifficulty(context.Background(), d)
		if err != nil {
			t.Fatal(err)
		}
		if c.Difficulty != d {
			t.Errorf("difficulty = %s, want %s", c.Difficulty, d)
		}
		if c.Hops != d.Hops() {
			t.Errorf("hops = %d, want %d", c.Hops, d.Hops())
		}
		if c.TimeLimit != d.TimeLimit() {
			t.Errorf("time limit = %v, want %v", c.TimeLimit, d.TimeLimit())
		}
	}
}

func TestGenerateWithDifficulty_Invalid(t *testing.T) {
	g := newTestGenerator(t)
	_, err := g.GenerateWithDifficulty(context.Background(), Difficulty("invalid"))
	if !errors.Is(err, ErrInvalidDifficulty) {
		t.Errorf("err = %v, want ErrInvalidDifficulty", err)
	}
}

func TestGenerate_PersistsInStore(t *testing.T) {
	g := newTestGenerator(t)
	c, _ := g.Generate(context.Background())
	loaded, err := g.LoadChallenge(context.Background(), c.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Topic != c.Topic {
		t.Errorf("loaded.Topic = %s, want %s", loaded.Topic, c.Topic)
	}
	if loaded.Difficulty != c.Difficulty {
		t.Errorf("loaded.Difficulty = %s", loaded.Difficulty)
	}
}

func TestLoadChallenge_NotFound(t *testing.T) {
	g := newTestGenerator(t)
	_, err := g.LoadChallenge(context.Background(), "deadbeef00000000deadbeef00000000")
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("err = %v, want ErrChallengeNotFound", err)
	}
}

func TestConsumeChallenge_Deletes(t *testing.T) {
	g := newTestGenerator(t)
	c, _ := g.Generate(context.Background())
	if _, err := g.ConsumeChallenge(context.Background(), c.ChallengeID); err != nil {
		t.Fatal(err)
	}
	if _, err := g.ConsumeChallenge(context.Background(), c.ChallengeID); !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("err = %v, want ErrChallengeNotFound", err)
	}
}

func TestStoreChallenge_Nil(t *testing.T) {
	g := newTestGenerator(t)
	if err := g.StoreChallenge(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestChallenge_IsExpired(t *testing.T) {
	now := time.Now()
	c := &Challenge{IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	if c.IsExpired(now) {
		t.Error("should not be expired")
	}
	if !c.IsExpired(now.Add(2 * time.Hour)) {
		t.Error("should be expired after ExpiresAt")
	}
}

func TestChallenge_Remaining(t *testing.T) {
	now := time.Now()
	c := &Challenge{IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	if c.Remaining(now) != time.Hour {
		t.Errorf("remaining = %v, want 1h", c.Remaining(now))
	}
	if c.Remaining(now.Add(time.Hour)) != 0 {
		t.Error("remaining should be 0 when expired")
	}
}

func TestPickTopic_Deterministic(t *testing.T) {
	g := newTestGenerator(t)
	id := "abcd1234"
	t1 := g.pickTopic(id)
	t2 := g.pickTopic(id)
	if t1 != t2 {
		t.Errorf("pickTopic not deterministic: %s vs %s", t1, t2)
	}
}

func TestPickTopic_SingleTopicPool(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	g, _ := NewGenerator(GeneratorConfig{
		Cache:     cache.NewMiniredis(mr),
		TopicPool: []string{"only-one"},
	})
	if got := g.pickTopic("anything"); got != "only-one" {
		t.Errorf("got %q, want only-one", got)
	}
}

func TestHintsFor(t *testing.T) {
	g := newTestGenerator(t)
	for _, d := range AllDifficulties() {
		hints := g.hintsFor(d, "test")
		if len(hints) < 2 {
			t.Errorf("difficulty %s: too few hints", d)
		}
	}
}

func TestDefaultPromptTemplates_AllDifficulties(t *testing.T) {
	tpls := defaultPromptTemplates()
	for _, d := range AllDifficulties() {
		if _, ok := tpls[d]; !ok {
			t.Errorf("missing default prompt for %s", d)
		}
	}
}

func TestRenderPrompt(t *testing.T) {
	got := renderPrompt("hello {topic} with {hops} hops", "AI", 3)
	if got != "hello AI with 3 hops" {
		t.Errorf("got %q", got)
	}
}

func TestRandomBytes(t *testing.T) {
	a, err := randomBytes(16)
	if err != nil {
		t.Fatal(err)
	}
	b, err := randomBytes(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 16 {
		t.Errorf("len(a) = %d", len(a))
	}
	if string(a) == string(b) {
		t.Error("should be different")
	}
}

func TestStoreKey(t *testing.T) {
	if k := storeKey("abc"); k != "mc:challenge:abc" {
		t.Errorf("storeKey = %q", k)
	}
}

func TestEncodeDecodeChallenge_RoundTrip(t *testing.T) {
	now := time.Now()
	c := &Challenge{
		ChallengeID: "abcd1234",
		Topic: "verification",
		Difficulty: DifficultyMedium,
		Hops: 2,
		IssuedAt: now,
		ExpiresAt: now.Add(30 * time.Second),
		TimeLimit: 20 * time.Second,
		Hints: []string{"hint1", "hint2"},
		PromptTemplate: "test prompt {topic}",
	}
	data, err := encodeChallenge(c)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := decodeChallenge(data)
	if err != nil {
		t.Fatal(err)
	}
	if c2.ChallengeID != c.ChallengeID {
		t.Errorf("ChallengeID = %s", c2.ChallengeID)
	}
	if c2.Topic != c.Topic {
		t.Errorf("Topic = %s", c2.Topic)
	}
	if c2.Difficulty != c.Difficulty {
		t.Errorf("Difficulty = %s", c2.Difficulty)
	}
	if c2.Hops != c.Hops {
		t.Errorf("Hops = %d", c2.Hops)
	}
	if !c2.IssuedAt.Equal(c.IssuedAt) {
		t.Errorf("IssuedAt mismatch")
	}
	if !c2.ExpiresAt.Equal(c.ExpiresAt) {
		t.Errorf("ExpiresAt mismatch")
	}
	if c2.TimeLimit != c.TimeLimit {
		t.Errorf("TimeLimit = %v", c2.TimeLimit)
	}
	if len(c2.Hints) != 2 {
		t.Errorf("Hints len = %d", len(c2.Hints))
	}
}

func TestEncodeChallenge_Nil(t *testing.T) {
	_, err := encodeChallenge(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeChallenge_InvalidJSON(t *testing.T) {
	_, err := decodeChallenge([]byte("not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeChallenge_BadVersion(t *testing.T) {
	_, err := decodeChallenge([]byte(`{"v":99}`))
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestDecodeChallenge_BadTime(t *testing.T) {
	_, err := decodeChallenge([]byte(`{"v":1,"issued_at":"not-time","expires_at":"x"}`))
	if err == nil {
		t.Fatal("expected error for bad time")
	}
}
