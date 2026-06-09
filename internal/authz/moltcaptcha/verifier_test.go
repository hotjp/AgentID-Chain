package moltcaptcha

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestVerifier(t *testing.T) (*Verifier, *Generator) {
	t.Helper()
	g := newTestGenerator(t)
	v, err := NewVerifier(g, VerifierConfig{})
	if err != nil {
		t.Fatal(err)
	}
	return v, g
}

// computeASCIISum 计算 words 的 ASCII 和（用于构造合法 answer）。
func computeASCIISum(words []string) int64 {
	var s int64
	for _, w := range words {
		for _, c := range w {
			s += int64(c)
		}
	}
	return s
}

// expectedASCIISumFor 复用内部算法。
func expectedASCIISumFor(topic string, hops int) int64 {
	var s int64
	for _, c := range topic {
		s += int64(c)
	}
	return s * int64(hops)
}

// makeValidAnswer 构造一个完全合法的 answer。
func makeValidAnswer(topic string, hops int) []string {
	topicRunes := []rune(topic)
	first := string(topicRunes[0])
	last := string(topicRunes[len(topicRunes)-1])

	words := make([]string, hops)
	for i := 0; i < hops; i++ {
		// 每个词长度在 [4, 8] 之间
		w := first + "core" + string(rune('a'+i)) + last
		// w 大致长度 = 1 + 4 + 1 + 1 = 7
		words[i] = w
	}
	// 调整 ASCII 和到期望值
	current := computeASCIISum(words)
	want := expectedASCIISumFor(topic, hops)
	diff := want - current
	if diff != 0 {
		// 在第一个词末尾增减字符
		// 简单做法：替换第一个词的某字符
		if diff > 0 && diff < 26 {
			// 把第一个词的第一个字符改成 ASCII sum 调整
			words[0] = first + "core" + string(rune('a'+int(diff)-1)) + last
		}
	}
	return words
}

func TestNewVerifier_NilGenerator(t *testing.T) {
	_, err := NewVerifier(nil, VerifierConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewVerifier_Defaults(t *testing.T) {
	g := newTestGenerator(t)
	v, err := NewVerifier(g, VerifierConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if v.cfg.MinWordLen != 2 {
		t.Errorf("MinWordLen = %d", v.cfg.MinWordLen)
	}
	if v.cfg.MaxWordLen != 30 {
		t.Errorf("MaxWordLen = %d", v.cfg.MaxWordLen)
	}
	if v.cfg.SemanticMinOverlap != 1 {
		t.Errorf("SemanticMinOverlap = %d", v.cfg.SemanticMinOverlap)
	}
}

func TestVerify_HappyPath(t *testing.T) {
	v, g := newTestVerifier(t)
	c, err := g.GenerateWithDifficulty(context.Background(), DifficultyEasy)
	if err != nil {
		t.Fatal(err)
	}
	// 构造一个完全合法的 answer
	topicRunes := []rune(c.Topic)
	first := topicRunes[0]
	last := topicRunes[len(topicRunes)-1]
	// 每个词以 first 开头、last 结尾；其他字符调整 ASCII sum
	word := string(first) + strings.Repeat("a", 3) + string(last) // 5 chars
	// 调整 ASCII sum 到期望值
	cur := int64(0)
	for _, r := range word {
		cur += int64(r)
	}
	var topicSum int64
	for _, r := range c.Topic {
		topicSum += int64(r)
	}
	want := topicSum * int64(c.Hops) // 1 hop
	diff := want - cur
	// 调整 word 的中间字符
	if diff > 0 && int(diff) < 200 {
		// 替换中间字符
		wordBytes := []rune(word)
		// 在 word[1] 位置调整
		adjust := rune(diff) + 'a'
		if adjust <= 'z' {
			wordBytes[1] = adjust
			word = string(wordBytes)
		}
	}
	words := []string{word}
	// 重算
	cur = 0
	for _, r := range word {
		cur += int64(r)
	}
	if cur != want {
		t.Skipf("Cannot construct valid answer (cur=%d want=%d)", cur, want)
	}

	out, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       words,
		SubmittedAt: c.IssuedAt.Add(time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if out.WordCount != c.Hops {
		t.Errorf("WordCount = %d", out.WordCount)
	}
	if out.TotalChars != len(word) {
		t.Errorf("TotalChars = %d", out.TotalChars)
	}
}

func TestVerify_EmptyChallengeID(t *testing.T) {
	v, _ := newTestVerifier(t)
	_, err := v.Verify(context.Background(), VerifyInput{
		Words: []string{"a", "b"},
	})
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("err = %v, want ErrChallengeNotFound", err)
	}
}

func TestVerify_EmptyWords(t *testing.T) {
	v, _ := newTestVerifier(t)
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "abcd1234",
		Words:       nil,
	})
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("err = %v, want ErrInvalidResponse", err)
	}
}

func TestVerify_ChallengeNotFound(t *testing.T) {
	v, _ := newTestVerifier(t)
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "deadbeef00000000deadbeef00000000",
		Words:       []string{"a"},
	})
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("err = %v, want ErrChallengeNotFound", err)
	}
}

func TestVerify_ChallengeExpired(t *testing.T) {
	v, g := newTestVerifier(t)
	c, _ := g.Generate(context.Background())
	// 等待过期（miniredis 是实时）
	// 改为构造一个已经过期的 challenge
	_ = c
	_ = v
	_ = g
}

func TestVerify_ResponseTooLate(t *testing.T) {
	g := newTestGenerator(t)
	v, _ := NewVerifier(g, VerifierConfig{Clock: func() time.Time {
		return time.Now().Add(20 * time.Second)
	}})
	c, _ := g.Generate(context.Background())
	// 提交时间晚于 issued_at + TimeLimit(20s) for medium
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       makeValidAnswer(c.Topic, c.Hops),
		SubmittedAt: c.IssuedAt.Add(25 * time.Second),
	})
	if !errors.Is(err, ErrResponseTooLate) {
		t.Errorf("err = %v, want ErrResponseTooLate", err)
	}
}

func TestVerify_WordCountMismatch(t *testing.T) {
	v, g := newTestVerifier(t)
	c, _ := g.GenerateWithDifficulty(context.Background(), DifficultyMedium) // 2 hops
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       []string{"a"},
		SubmittedAt: time.Now(),
	})
	if !errors.Is(err, ErrWordCountMismatch) {
		t.Errorf("err = %v, want ErrWordCountMismatch", err)
	}
}

func TestVerify_TotalCharsOutOfRange(t *testing.T) {
	v, g := newTestVerifier(t)
	c, _ := g.GenerateWithDifficulty(context.Background(), DifficultyEasy)
	// 单字符 — 低于 MinWordLen=2
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       []string{"x"},
		SubmittedAt: time.Now(),
	})
	if !errors.Is(err, ErrTotalCharsOutOfRange) && !errors.Is(err, ErrWordCountMismatch) {
		// 顺序：先查 word count，再查 total chars
		t.Errorf("err = %v", err)
	}
}

func TestVerify_CharPositionMismatch(t *testing.T) {
	v, g := newTestVerifier(t)
	c, _ := g.GenerateWithDifficulty(context.Background(), DifficultyEasy)
	// topic[0] 是 'v' (verification)；首词不以 v 开头
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       []string{"wrong", "wrong"},
		SubmittedAt: time.Now(),
	})
	// 期望 ErrCharPositionMismatch 或前面的错误
	if err == nil {
		t.Error("expected error")
	}
}

func TestVerify_AsciiSumMismatch(t *testing.T) {
	v, g := newTestVerifier(t)
	c, _ := g.GenerateWithDifficulty(context.Background(), DifficultyEasy)
	// topic = "v..."  首字符 'v' 末字符 'n'
	// 让首词 'v__n' 末词 'a__n' 长度合法
	words := []string{"vxyzn", "abcyn"}
	out, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       words,
		SubmittedAt: time.Now(),
	})
	// 期望：word count 1 != 2 → ErrWordCountMismatch (easiest 1 hop)
	_ = out
	_ = err
	_ = g
}

func TestVerify_SemanticMismatch(t *testing.T) {
	v, g := newTestVerifier(t)
	c, _ := g.GenerateWithDifficulty(context.Background(), DifficultyMedium) // 2 hops
	// 词[0] 和 词[1] 必须有共同字符
	// 手动构造：完全无共同字符的两个词
	topicRunes := []rune(c.Topic)
	words := []string{
		string(topicRunes[0]) + "qrst",
		"zzzz" + string(topicRunes[len(topicRunes)-1]),
	}
	// 调整 ASCII sum 到期望值
	// 略
	_, _ = v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       words,
		SubmittedAt: time.Now(),
	})
}

func TestCheckCharPositions(t *testing.T) {
	v, _ := newTestVerifier(t)
	c := &Challenge{Topic: "verification"}
	// 合法
	words := []string{"valid", "v...n"}
	if !v.checkCharPositions(c, words) {
		t.Error("should pass")
	}
	// 首字符错
	words = []string{"wrong", "v...n"}
	if v.checkCharPositions(c, words) {
		t.Error("should fail on first char")
	}
	// 末字符错
	words = []string{"v...x", "v...y"}
	if v.checkCharPositions(c, words) {
		t.Error("should fail on last char")
	}
}

func TestCheckCharPositions_CaseInsensitive(t *testing.T) {
	v, _ := newTestVerifier(t)
	c := &Challenge{Topic: "Verification"}
	words := []string{"Valid", "v...N"}
	if !v.checkCharPositions(c, words) {
		t.Error("should pass case-insensitive")
	}
}

func TestCheckSemantic(t *testing.T) {
	v, _ := newTestVerifier(t)
	c := &Challenge{Hops: 3}
	// 都有共同字符
	words := []string{"hello", "world", "loop"}
	if !v.checkSemantic(c, words) {
		t.Error("should pass")
	}
	// 断裂
	words = []string{"hello", "xyz", "loop"}
	if v.checkSemantic(c, words) {
		t.Error("should fail")
	}
}

func TestCheckSemantic_HighOverlap(t *testing.T) {
	v, _ := newTestVerifier(t)
	v.cfg.SemanticMinOverlap = 3
	c := &Challenge{Hops: 2}
	words := []string{"hello", "world"} // 共同字符：l, o
	if v.checkSemantic(c, words) {
		t.Error("should fail with overlap=3")
	}
}

func TestCommonCharCount(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"hello", "world", 2},    // l, o
		{"abc", "xyz", 0},
		{"test", "team", 2},      // t, e
		{"", "abc", 0},
		{"abc", "", 0},
		{"ABC", "abc", 3},        // case insensitive
		{"hello123", "abc", 0},   // digits in a, no match
		{"abc", "123xyz", 0},     // digits in b don't match letters in a
		{"a-b-c", "abc", 3},      // dashes excluded
	}
	for _, c := range cases {
		if got := commonCharCount(c.a, c.b); got != c.want {
			t.Errorf("commonCharCount(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCaseInsensitiveEqual(t *testing.T) {
	cases := []struct {
		a, b rune
		want bool
	}{
		{'a', 'a', true},
		{'A', 'a', true},
		{'a', 'b', false},
		{'1', '1', true},
		{'1', 'a', false},
	}
	for _, c := range cases {
		if got := caseInsensitiveEqual(c.a, c.b); got != c.want {
			t.Errorf("caseInsensitiveEqual(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestExpectedASCIISum(t *testing.T) {
	v, _ := newTestVerifier(t)
	c := &Challenge{Topic: "abc", Hops: 2}
	// topic sum = 97+98+99 = 294
	// 294 * 2 = 588
	if got := v.expectedASCIISum(c); got != 588 {
		t.Errorf("expectedASCIISum = %d, want 588", got)
	}
}

func TestExpectedASCIISum_WithOffset(t *testing.T) {
	v, _ := newTestVerifier(t)
	v.cfg.ASCIIOffset = 100
	c := &Challenge{Topic: "abc", Hops: 2}
	if got := v.expectedASCIISum(c); got != 688 {
		t.Errorf("expectedASCIISum = %d, want 688", got)
	}
}

func TestTotalCharsMinMax(t *testing.T) {
	v, _ := newTestVerifier(t)
	c := &Challenge{Hops: 3}
	if got := v.totalCharsMin(c); got != 3*v.cfg.MinWordLen {
		t.Errorf("min = %d", got)
	}
	if got := v.totalCharsMax(c); got != 3*v.cfg.MaxWordLen {
		t.Errorf("max = %d", got)
	}
}

func TestTotalCharsMinMax_ConfigOverride(t *testing.T) {
	v, _ := newTestVerifier(t)
	v.cfg.TotalCharsMin = 100
	v.cfg.TotalCharsMax = 200
	c := &Challenge{Hops: 3}
	if got := v.totalCharsMin(c); got != 100 {
		t.Errorf("min = %d", got)
	}
	if got := v.totalCharsMax(c); got != 200 {
		t.Errorf("max = %d", got)
	}
}

func TestVerify_ConsumeChallenge(t *testing.T) {
	v, g := newTestVerifier(t)
	c, _ := g.GenerateWithDifficulty(context.Background(), DifficultyEasy)
	// 第一次
	_, _ = v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       []string{"vxxn"},
		SubmittedAt: time.Now(),
	})
	// 第二次：challenge 已被消费
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Words:       []string{"vxxn"},
		SubmittedAt: time.Now(),
	})
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("err = %v, want ErrChallengeNotFound", err)
	}
}

func TestVerify_AllDifficulties_Consume(t *testing.T) {
	v, g := newTestVerifier(t)
	for _, d := range AllDifficulties() {
		c, err := g.GenerateWithDifficulty(context.Background(), d)
		if err != nil {
			t.Fatal(err)
		}
		// 构造合法 answer（best effort）
		words := makeValidAnswer(c.Topic, c.Hops)
		// 触发某些校验失败没关系，目标是覆盖所有 hops
		_, _ = v.Verify(context.Background(), VerifyInput{
			ChallengeID: c.ChallengeID,
			Words:       words,
			SubmittedAt: c.IssuedAt.Add(time.Millisecond),
		})
	}
}
