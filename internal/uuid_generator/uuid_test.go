package uuid_generator

import (
	"strings"
	"testing"
)

func TestGenerateV7_Format(t *testing.T) {
	g := NewGenerator()
	u, err := g.GenerateV7()
	if err != nil {
		t.Fatal(err)
	}
	if err := ParseUUID(u); err != nil {
		t.Errorf("parse: %v (uuid=%s)", err, u)
	}
	// 验证第 14 位是 '7'（version）
	parts := strings.Split(u, "-")
	if parts[2][0] != '7' {
		t.Errorf("version char = %c, want '7'", parts[2][0])
	}
}

func TestGenerateV7_Unique(t *testing.T) {
	g := NewGenerator()
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		u, err := g.GenerateV7()
		if err != nil {
			t.Fatal(err)
		}
		if seen[u] {
			t.Errorf("duplicate: %s", u)
		}
		seen[u] = true
	}
}

func TestBatchGenerate(t *testing.T) {
	g := NewGenerator()
	out, err := g.BatchGenerate(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 100 {
		t.Errorf("len = %d, want 100", len(out))
	}
	// 验证全部不同
	seen := make(map[string]bool)
	for _, u := range out {
		if seen[u] {
			t.Errorf("duplicate in batch: %s", u)
		}
		seen[u] = true
	}
}

func TestParseUUID_Valid(t *testing.T) {
	valid := []string{
		"019eab1a-b761-7a60-955c-37f926faa100",
		"00000000-0000-7000-8000-000000000000",
		"ffffffff-ffff-7fff-bfff-ffffffffffff",
	}
	for _, u := range valid {
		if err := ParseUUID(u); err != nil {
			t.Errorf("parse(%s): %v", u, err)
		}
	}
}

func TestParseUUID_Invalid(t *testing.T) {
	invalid := []string{
		"too-short",
		"019eab1a-b761-7a60-955c-37f926faa10",  // 11 chars
		"019eab1a_b761_7a60_955c_37f926faa100", // underscore
		"019eab1a-b761-7a60-955c-37f926faa10Z", // 'Z'
		"",                                   // empty
	}
	for _, u := range invalid {
		if err := ParseUUID(u); err == nil {
			t.Errorf("expected error for %q", u)
		}
	}
}

// ---------- Benchmarks ----------

func BenchmarkGenerateV7(b *testing.B) {
	g := NewGenerator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.GenerateV7()
	}
}

func BenchmarkBatchGenerate10(b *testing.B) {
	g := NewGenerator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.BatchGenerate(10)
	}
}

func BenchmarkBatchGenerate100(b *testing.B) {
	g := NewGenerator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.BatchGenerate(100)
	}
}

func BenchmarkBatchGenerate1000(b *testing.B) {
	g := NewGenerator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.BatchGenerate(1000)
	}
}

func BenchmarkParseUUID(b *testing.B) {
	u := "019eab1a-b761-7a60-955c-37f926faa100"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseUUID(u)
	}
}
