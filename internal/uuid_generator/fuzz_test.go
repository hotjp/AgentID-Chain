package uuid_generator

import (
	"testing"
)

// FuzzParseUUID fuzz ParseUUID（应永远不 panic）。
// 任何非法输入都应返回 error，而不是 panic。
//
// 运行：
//   go test -fuzz=FuzzParseUUID -fuzztime=30s ./internal/uuid_generator/...
func FuzzParseUUID(f *testing.F) {
	// seed corpus
	f.Add("019eab1a-b761-7a60-955c-37f926faa100")
	f.Add("00000000-0000-7000-8000-000000000000")
	f.Add("")
	f.Add("not-a-uuid")
	f.Add("xxxxxxxx-xxxx-Mxxx-Nxxx-xxxxxxxxxxxx")
	f.Add("00000000-0000-0000-0000-00000000000Z")
	f.Add("\x00\x01\x02\x03")
	f.Add("019eab1a-b761-7a60-955c-37f926faa100_extra_data_xxxxxxxxxxxxxxxxxxxxx")

	f.Fuzz(func(t *testing.T, s string) {
		// 只验证不 panic；返回值任意
		_ = ParseUUID(s)
	})
}

// FuzzGenerateV7 fuzz 生成 UUID（验证不重复 + 格式合法）。
func FuzzGenerateV7(f *testing.F) {
	f.Add(int64(1))
	f.Add(int64(1000))
	f.Add(int64(100000))

	f.Fuzz(func(t *testing.T, n int64) {
		if n < 0 || n > 10000 {
			t.Skip()
		}
		g := NewGenerator()
		seen := make(map[string]bool)
		for i := int64(0); i < n; i++ {
			u, err := g.GenerateV7()
			if err != nil {
				t.Fatalf("err at %d: %v", i, err)
			}
			if seen[u] {
				t.Fatalf("duplicate at %d: %s", i, u)
			}
			seen[u] = true
			if err := ParseUUID(u); err != nil {
				t.Fatalf("parse err at %d: %v (uuid=%s)", i, err, u)
			}
		}
	})
}
